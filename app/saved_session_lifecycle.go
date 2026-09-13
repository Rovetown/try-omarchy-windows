package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// A coordinator serializes changes to one guest. Successful saves leave that
// guest stopped until the launcher has observed its process exit. Failed saves
// recover through a fresh context even when the user's operation was cancelled.
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
			if recoveryErr := recoverRunningSession(recovery, c); recoveryErr != nil {
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
	// A failed block-job cleanup must not race a resumed guest. Leave the guest
	// paused with its original disk when we cannot establish exclusive ownership.
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
