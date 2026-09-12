package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Identity binds RAM and device state to the exact machine recipe and runtime.
// The launch coordinator calculates these from verified files and arguments,
// before permitting updates or changing device configuration.
type savedSessionIdentity struct {
	Architecture  string `json:"architecture"`
	RuntimeSHA256 string `json:"runtimeSHA256"`
	MachineSHA256 string `json:"machineSHA256"`
}

func (identity savedSessionIdentity) valid() bool {
	return (identity.Architecture == "x86_64" || identity.Architecture == "aarch64") && validSHA256(identity.RuntimeSHA256) && validSHA256(identity.MachineSHA256)
}

type savedSessionRecord struct {
	Version  int                  `json:"version"`
	Created  time.Time            `json:"created"`
	Identity savedSessionIdentity `json:"identity"`
	Disk     savedMemoryFile      `json:"disk"`
	Memory   savedMemoryFile      `json:"memory"`
}

// capturePausedSession publishes a complete disk/RAM pair. The caller owns
// pause, cancellation recovery and process exit. A failed transfer retains its
// private staging because QEMU may still hold an open block or migration job.
func capturePausedSession(ctx context.Context, c *qmpClient, sourceDisk, destination string, identity savedSessionIdentity, progress backupProgress) (savedSessionRecord, error) {
	var record savedSessionRecord
	if !identity.valid() {
		return record, fmt.Errorf("invalid saved-session machine identity")
	}
	if !filepath.IsAbs(destination) {
		return record, fmt.Errorf("saved-session destination must be absolute")
	}
	if err := validateMovePath(destination); err != nil {
		return record, err
	}
	if _, err := os.Lstat(destination); !os.IsNotExist(err) {
		return record, fmt.Errorf("saved-session destination already exists or is unavailable")
	}
	facts, err := inspectVMRuntime(ctx, c)
	if err != nil {
		return record, err
	}
	if facts.State.Running || (facts.State.Status != "paused" && facts.State.Status != "prelaunch") {
		return record, fmt.Errorf("pause the guest before saving its session")
	}
	if len(facts.MigrationBlockers) != 0 {
		return record, fmt.Errorf("runtime cannot preserve the attached devices: %v", facts.MigrationBlockers)
	}
	stage, err := os.MkdirTemp(filepath.Dir(destination), ".session-writing-")
	if err != nil {
		return record, err
	}
	fail := func(err error) (savedSessionRecord, error) {
		return savedSessionRecord{}, fmt.Errorf("saved session was not published; recovery files are in %s: %w", stage, err)
	}
	diskPath := filepath.Join(stage, "disk.raw")
	if _, err := copyPausedVMDisk(ctx, c, sourceDisk, diskPath, progress); err != nil {
		return fail(err)
	}
	disk, err := openSavedMemory(diskPath)
	if err != nil {
		return fail(err)
	}
	record.Disk, err = hashSavedMemory(ctx, disk)
	disk.Close()
	if err != nil {
		return fail(err)
	}
	record.Memory, err = capturePausedVMState(ctx, c, filepath.Join(stage, "memory.bin"), func(state vmMigrationProgress) {
		if progress != nil {
			progress(int64(state.RAM.Transferred), int64(state.RAM.Total), "Saving application memory")
		}
	})
	if err != nil {
		return fail(err)
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	record.Version = 1
	record.Created = time.Now().UTC()
	record.Identity = identity
	manifest, err := os.OpenFile(filepath.Join(stage, "saved-session.json"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return fail(err)
	}
	err = json.NewEncoder(manifest).Encode(record)
	if err == nil {
		err = manifest.Sync()
	}
	closeErr := manifest.Close()
	if err != nil {
		return fail(err)
	}
	if closeErr != nil {
		return fail(closeErr)
	}
	if err := publishMoveDirectory(stage, destination); err != nil {
		return fail(err)
	}
	return record, nil
}

type verifiedSavedSession struct {
	Record savedSessionRecord
	Disk   *os.File
	Memory *os.File
}

func (session *verifiedSavedSession) Close() {
	if session.Disk != nil {
		session.Disk.Close()
	}
	if session.Memory != nil {
		session.Memory.Close()
	}
}

// Verification holds read handles that deny mutation on Windows until the
// coordinator finishes preparing its independent resume disk and memory stream.
func openVerifiedSavedSession(ctx context.Context, directory string, identity savedSessionIdentity) (*verifiedSavedSession, error) {
	if !identity.valid() || !filepath.IsAbs(directory) {
		return nil, fmt.Errorf("invalid saved-session identity or path")
	}
	if err := validateMovePath(directory); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	if len(entries) != 3 {
		return nil, fmt.Errorf("unexpected saved-session inventory")
	}
	for _, entry := range entries {
		name := entry.Name()
		if name != "saved-session.json" && name != "memory.bin" && name != "disk.raw" {
			return nil, fmt.Errorf("unexpected saved-session file")
		}
		path := filepath.Join(directory, name)
		if err := validateMovePath(path); err != nil {
			return nil, err
		}
		info, err := os.Lstat(path)
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() || (name == "saved-session.json" && info.Size() > 4096) {
			return nil, fmt.Errorf("invalid saved-session file")
		}
	}
	manifest, err := openSavedMemory(filepath.Join(directory, "saved-session.json"))
	if err != nil {
		return nil, err
	}
	defer manifest.Close()
	var record savedSessionRecord
	decoder := json.NewDecoder(io.LimitReader(manifest, 4097))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&record); err != nil {
		return nil, err
	}
	if decoder.Decode(new(any)) != io.EOF || record.Version != 1 || record.Created.IsZero() || record.Identity != identity {
		return nil, fmt.Errorf("saved session requires its original runtime and machine configuration")
	}
	session := &verifiedSavedSession{Record: record}
	for _, item := range []struct {
		name     string
		expected savedMemoryFile
		target   **os.File
	}{
		{"disk.raw", record.Disk, &session.Disk}, {"memory.bin", record.Memory, &session.Memory},
	} {
		if !validSHA256(item.expected.SHA256) || item.expected.Bytes <= 0 || item.expected.Bytes > backupMaxBytes {
			session.Close()
			return nil, fmt.Errorf("invalid saved-session data identity")
		}
		*item.target, err = openSavedMemory(filepath.Join(directory, item.name))
		if err != nil {
			session.Close()
			return nil, err
		}
		actual, err := hashSavedMemory(ctx, *item.target)
		if err != nil {
			session.Close()
			return nil, err
		}
		if actual != item.expected {
			session.Close()
			return nil, fmt.Errorf("saved-session %s checksum mismatch", item.name)
		}
		if _, err := (*item.target).Seek(0, io.SeekStart); err != nil {
			session.Close()
			return nil, err
		}
	}
	return session, nil
}
