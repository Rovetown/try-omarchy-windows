package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// A coordinator serializes changes to one guest. Successful saves leave that
// guest stopped until the launcher has observed its process exit. Failed saves
// recover through a fresh connection and context even after cancellation.
// A failed attempt closes its supplied feature-control client; callers open
// a new client for subsequent operations. The supervisor has its own stream.
type savedSessionCoordinator struct{ mu sync.Mutex }

func (s *savedSessionCoordinator) Save(ctx context.Context, c *qmpClient, source, destination string, identity savedSessionIdentity, progress backupProgress) (record savedSessionRecord, err error) {
	if !s.mu.TryLock() {
		return record, fmt.Errorf("another saved-session operation is in progress")
	}
	defer s.mu.Unlock()
	if !identity.valid() {
		return record, fmt.Errorf("invalid saved-session machine identity")
	}
	facts, err := inspectVMRuntime(ctx, c)
	if err != nil {
		return record, err
	}
	if !facts.State.Running || facts.State.Status != "running" {
		return record, fmt.Errorf("the guest must be running before saving its session")
	}
	if len(facts.MigrationBlockers) != 0 {
		return record, fmt.Errorf("runtime cannot preserve the attached devices: %s", strings.Join(facts.MigrationBlockers, "; "))
	}
	for _, command := range []string{"stop", "cont", "migrate", "migrate_cancel", "query-migrate", "query-jobs", "blockdev-backup"} {
		if !facts.Commands[command] {
			return record, fmt.Errorf("runtime is missing saved-session command %s", command)
		}
	}
	// Do not take ownership of an unrelated migration or disk job.
	if err := idleSessionTransfers(ctx, c); err != nil {
		return record, err
	}
	// A cancelled stop request may have reached QEMU even without an answer.
	// Install recovery before submitting it, not after receiving its reply.
	defer func() {
		if err != nil {
			recovery, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			control, closeControl, recoveryErr := savedSessionRecoveryConnection(recovery, c)
			if recoveryErr == nil {
				defer closeControl()
				recoveryErr = recoverRunningSession(recovery, control)
			}
			if recoveryErr != nil {
				err = errors.Join(err, fmt.Errorf("the guest could not be resumed automatically; keep its process and recovery files: %w", recoveryErr))
			}
		}
	}()
	if err = c.Call(ctx, "stop", nil, nil); err != nil {
		return record, err
	}
	record, err = capturePausedSession(ctx, c, source, destination, identity, progress)
	return record, err
}

func idleSessionTransfers(ctx context.Context, c *qmpClient) error {
	var migration vmMigrationProgress
	if err := c.Call(ctx, "query-migrate", nil, &migration); err != nil {
		return err
	}
	switch migration.Status {
	case "", "none", "completed", "failed", "cancelled":
	default:
		return fmt.Errorf("a memory transfer is already in progress: %s", migration.Status)
	}
	var jobs []diskCopyJob
	if err := c.Call(ctx, "query-jobs", nil, &jobs); err != nil {
		return err
	}
	if len(jobs) != 0 {
		return fmt.Errorf("finish the current disk operation before saving a session")
	}
	return nil
}

func recoverRunningSession(ctx context.Context, c *qmpClient) error {
	var migration vmMigrationProgress
	if err := c.Call(ctx, "query-migrate", nil, &migration); err != nil {
		return err
	}
	switch migration.Status {
	case "", "none", "completed", "failed", "cancelled":
	default:
		cancelErr := c.Call(ctx, "migrate_cancel", nil, nil)
		for {
			if err := c.Call(ctx, "query-migrate", nil, &migration); err != nil {
				return errors.Join(cancelErr, err)
			}
			if migration.Status == "cancelled" || migration.Status == "failed" || migration.Status == "completed" {
				break
			}
			select {
			case <-ctx.Done():
				return errors.Join(cancelErr, ctx.Err())
			case <-time.After(25 * time.Millisecond):
			}
		}
	}
	// The save operation started with no disk jobs. Settle only jobs in its
	// private namespace before resuming, including a lost cleanup reply.
	var jobs []diskCopyJob
	if err := c.Call(ctx, "query-jobs", nil, &jobs); err != nil {
		return err
	}
	for _, job := range jobs {
		suffix := strings.TrimPrefix(job.ID, "tom-save-job-")
		if suffix == job.ID || len(suffix) != 16 {
			return fmt.Errorf("an unrelated disk operation prevents safe recovery")
		}
		if job.Status != "concluded" {
			cancelErr := c.Call(ctx, "job-cancel", map[string]any{"id": job.ID}, nil)
			if err := waitForDiskCopy(ctx, c, job.ID, nil, true); err != nil {
				return errors.Join(cancelErr, err)
			}
		}
		if err := c.Call(ctx, "job-dismiss", map[string]any{"id": job.ID}, nil); err != nil {
			return err
		}
		if err := c.Call(ctx, "blockdev-del", map[string]any{"node-name": "tom-save-" + suffix}, nil); err != nil {
			return err
		}
	}
	// Leave the original guest paused if exclusive ownership is still unclear.
	if err := idleSessionTransfers(ctx, c); err != nil {
		return err
	}
	var state vmRuntimeStatus
	if err := c.Call(ctx, "query-status", nil, &state); err != nil {
		return err
	}
	if state.Running && state.Status == "running" {
		return nil
	}
	if state.Status != "paused" && state.Status != "postmigrate" {
		return fmt.Errorf("guest is not safe to resume from state %s", state.Status)
	}
	if err := c.Call(ctx, "cont", nil, nil); err != nil {
		return err
	}
	if err := c.Call(ctx, "query-status", nil, &state); err != nil {
		return err
	}
	if !state.Running || state.Status != "running" {
		return fmt.Errorf("guest did not resume: %s", state.Status)
	}
	return nil
}

// Interrupted QMP commands deliberately invalidate their stream. Reconnecting
// allows recovery to inspect the result without replaying the interrupted command.
func savedSessionRecoveryConnection(ctx context.Context, c *qmpClient) (*qmpClient, func(), error) {
	select {
	case c.gate <- struct{}{}:
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}
	address := c.conn.RemoteAddr()
	c.conn.Close()
	c.broken = true
	<-c.gate
	if address == nil || (address.Network() != "tcp" && address.Network() != "unix") {
		return nil, nil, fmt.Errorf("runtime control address is unavailable for recovery")
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, address.Network(), address.String())
	if err != nil {
		return nil, nil, err
	}
	fresh, err := newQMPClient(ctx, conn)
	if err != nil {
		return nil, nil, err
	}
	return fresh, func() { fresh.Close() }, nil
}
