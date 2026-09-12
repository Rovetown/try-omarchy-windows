package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type savedMemoryFile struct {
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}

type vmMigrationProgress struct {
	Status string `json:"status"`
	Error  string `json:"error-desc"`
	RAM    struct {
		Transferred uint64 `json:"transferred"`
		Remaining   uint64 `json:"remaining"`
		Total       uint64 `json:"total"`
	} `json:"ram"`
}

// Windows QEMU cannot change blocking mode on file channels. Socket channels
// use its supported migration path while the launcher owns file I/O and checks.
func memorySocketChannel(path string) map[string]any {
	return map[string]any{"channels": []any{map[string]any{
		"channel-type": "main", "addr": map[string]any{"transport": "socket", "type": "unix", "path": path},
	}}}
}

// Each memory stream uses a fresh private directory outside guest networking.
func privateMemorySocket() (string, func(), error) {
	base, err := qmpControlDirectory()
	if err != nil {
		return "", nil, err
	}
	if err := validateMovePath(base); err != nil {
		return "", nil, err
	}
	if err := os.MkdirAll(base, 0700); err != nil {
		return "", nil, err
	}
	dir, err := os.MkdirTemp(base, "m-")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { os.RemoveAll(dir) }
	path := filepath.Join(dir, "ram.sock")
	if !filepath.IsAbs(path) || len([]byte(path)) > 103 {
		cleanup()
		return "", nil, fmt.Errorf("private saved-session path is too long")
	}
	return path, cleanup, nil
}

func waitForVMMigration(ctx context.Context, c *qmpClient, progress func(vmMigrationProgress)) error {
	for {
		var state vmMigrationProgress
		if err := c.Call(ctx, "query-migrate", nil, &state); err != nil {
			return err
		}
		if progress != nil {
			progress(state)
		}
		switch state.Status {
		case "completed":
			return nil
		case "failed", "cancelled":
			return fmt.Errorf("saved-session transfer %s: %s", state.Status, state.Error)
		case "setup", "active", "device", "pre-switchover", "wait-unplug", "cancelling":
		default:
			return fmt.Errorf("unexpected saved-session transfer state %q", state.Status)
		}
		timer := time.NewTimer(50 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

// capturePausedVMState does not stop, resume or destroy a guest. The session
// coordinator owns that lifecycle and must bind this stream to its unchanged
// disk and exact runtime configuration before publishing a saved session.
// After any interrupted command, the private output remains available until
// the coordinator has cancelled/inspected migration and knows writes stopped.
func capturePausedVMState(ctx context.Context, c *qmpClient, path string, progress func(vmMigrationProgress)) (savedMemoryFile, error) {
	var result savedMemoryFile
	facts, err := inspectVMRuntime(ctx, c)
	if err != nil {
		return result, err
	}
	if facts.State.Running || (facts.State.Status != "paused" && facts.State.Status != "prelaunch") {
		return result, fmt.Errorf("pause the guest before saving its memory")
	}
	if len(facts.MigrationBlockers) != 0 {
		return result, fmt.Errorf("runtime cannot preserve the attached devices: %s", strings.Join(facts.MigrationBlockers, "; "))
	}
	if !facts.Commands["migrate"] || !facts.Commands["query-migrate"] {
		return result, fmt.Errorf("runtime does not support saved-session transfer")
	}
	if !filepath.IsAbs(path) {
		return result, fmt.Errorf("saved memory requires an absolute path")
	}
	if err := validateMovePath(path); err != nil {
		return result, err
	}
	var memory struct {
		Base    uint64 `json:"base-memory"`
		Plugged uint64 `json:"plugged-memory"`
	}
	if err := c.Call(ctx, "query-memory-size-summary", nil, &memory); err != nil {
		return result, err
	}
	if memory.Base == 0 || memory.Base > 1<<40 || memory.Plugged > 1<<40-memory.Base {
		return result, fmt.Errorf("invalid guest memory size")
	}
	if err := requireDiskSpace(filepath.Dir(path), int64(memory.Base+memory.Plugged)+diskSpaceReserve); err != nil {
		return result, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil {
		return result, err
	}

	defer f.Close()
	socketPath, cleanupSocket, err := privateMemorySocket()
	if err != nil {
		return result, err
	}
	defer cleanupSocket()
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return result, err
	}
	transferCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan error, 1)
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		conn, err := listener.Accept()
		if err != nil {
			done <- err
			return
		}
		defer conn.Close()
		stop := context.AfterFunc(transferCtx, func() { conn.Close() })
		defer stop()
		_, err = io.Copy(f, io.LimitReader(conn, int64(memory.Base+memory.Plugged)+diskSpaceReserve))
		done <- err
		if err != nil {
			cancel()
		}
	}()
	defer func() { cancel(); listener.Close(); <-finished }()
	if err := c.Call(transferCtx, "migrate", memorySocketChannel(socketPath), nil); err != nil {
		return result, err
	}
	if err := waitForVMMigration(transferCtx, c, progress); err != nil {
		return result, err
	}

	drain := time.NewTicker(50 * time.Millisecond)
	defer drain.Stop()
draining:
	for {
		select {
		case err := <-done:
			if err != nil {
				return result, err
			}
			break draining
		case <-transferCtx.Done():
			return result, transferCtx.Err()
		case <-drain.C:
			var state vmRuntimeStatus
			if err := c.Call(transferCtx, "query-status", nil, &state); err != nil {
				return result, err
			}
			if state.Running {
				return result, fmt.Errorf("guest resumed before saved memory was complete")
			}
		}
	}

	if err := f.Sync(); err != nil {
		return result, err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return result, err
	}

	return hashSavedMemory(ctx, f)
}

func hashSavedMemory(ctx context.Context, f *os.File) (savedMemoryFile, error) {
	var result savedMemoryFile
	info, err := f.Stat()
	if err != nil {
		return result, err
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > 2<<40 {
		return result, fmt.Errorf("invalid saved memory file")
	}
	h := sha256.New()
	buffer := make([]byte, 1<<20)
	for {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		n, err := f.Read(buffer)
		if n > 0 {
			h.Write(buffer[:n])
			result.Bytes += int64(n)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return result, err
		}
	}
	if result.Bytes != info.Size() {
		return result, fmt.Errorf("saved memory changed during verification")
	}
	result.SHA256 = hex.EncodeToString(h.Sum(nil))
	return result, nil
}

// The receiving QEMU is started with -incoming defer -S. Verify the complete
// stream before allowing QEMU to read it, and keep the restored guest paused.
func restorePausedVMState(ctx context.Context, c *qmpClient, path string, expected savedMemoryFile) error {
	if !filepath.IsAbs(path) || !validSHA256(expected.SHA256) || expected.Bytes <= 0 {
		return fmt.Errorf("invalid saved-session identity")
	}
	if err := validateMovePath(path); err != nil {
		return err
	}
	f, err := openSavedMemory(path)
	if err != nil {
		return err
	}
	defer f.Close()
	actual, err := hashSavedMemory(ctx, f)
	if err != nil {
		return err
	}
	if actual != expected {
		return fmt.Errorf("saved memory checksum mismatch")
	}
	var state vmRuntimeStatus
	if err := c.Call(ctx, "query-status", nil, &state); err != nil {
		return err
	}
	if state.Running || state.Status != "inmigrate" {
		return fmt.Errorf("runtime was not prepared to receive a saved session")
	}

	address, cleanupSocket, err := privateMemorySocket()
	if err != nil {
		return err
	}
	defer cleanupSocket()
	args := memorySocketChannel(address)
	args["exit-on-error"] = false
	if err := c.Call(ctx, "migrate-incoming", args, nil); err != nil {
		return err
	}
	transferCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(transferCtx, "unix", address)
	if err != nil {
		return err
	}
	defer conn.Close()
	stop := context.AfterFunc(transferCtx, func() { conn.Close() })
	defer stop()
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	done := make(chan error, 1)
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		_, err := io.Copy(conn, f)
		if stream, ok := conn.(*net.UnixConn); ok {
			stream.CloseWrite()
		}
		done <- err
		if err != nil {
			cancel()
		}
	}()
	defer func() { cancel(); conn.Close(); <-finished }()
	if err := waitForVMMigration(transferCtx, c, nil); err != nil {
		return err
	}
	select {
	case err := <-done:
		if err != nil {
			return err
		}
	case <-transferCtx.Done():
		return transferCtx.Err()
	}

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	actual, err = hashSavedMemory(ctx, f)
	if err != nil {
		return err
	}
	if actual != expected {
		return fmt.Errorf("saved memory changed during restore")
	}
	if err := c.Call(ctx, "query-status", nil, &state); err != nil {
		return err
	}
	if state.Running {
		return fmt.Errorf("restored runtime unexpectedly started before validation")
	}
	if state.Status != "paused" && state.Status != "prelaunch" {
		return fmt.Errorf("restored runtime is not ready: %s", state.Status)
	}
	return nil
}
