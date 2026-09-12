package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testSavedSessionIdentity() savedSessionIdentity {
	return savedSessionIdentity{Architecture: "x86_64", RuntimeSHA256: strings.Repeat("a", 64), MachineSHA256: strings.Repeat("b", 64)}
}

func TestSavedSessionPublishesMatchedDiskAndRAM(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "active.raw")
	contents := bytes.Repeat([]byte("saved document!\x00"), 65536)
	if err := os.WriteFile(source, contents, 0600); err != nil {
		t.Fatal(err)
	}
	pattern := bytes.Repeat([]byte("unsaved editor buffer\x00\xff"), 1024)
	seed := filepath.Join(dir, "seed.bin")
	if err := os.WriteFile(seed, pattern, 0600); err != nil {
		t.Fatal(err)
	}
	c, ctx, stop := startSavedSessionTestQEMU(t, "-drive", "file="+qemuOptionValue(source)+",format=raw,if=virtio", "-device", "loader,file="+qemuOptionValue(seed)+",addr=1048576,force-raw=on")
	destination := filepath.Join(dir, "saved")
	record, err := capturePausedSession(ctx, c, source, destination, testSavedSessionIdentity(), nil)
	if err != nil {
		t.Fatal(err)
	}
	stop()
	verified, err := openVerifiedSavedSession(ctx, destination, testSavedSessionIdentity())
	if err != nil {
		t.Fatal(err)
	}
	defer verified.Close()
	if verified.Record != record {
		t.Fatal("published record changed")
	}
	got, err := io.ReadAll(verified.Disk)
	if err != nil || !bytes.Equal(got, contents) {
		t.Fatalf("disk data changed: %v", err)
	}
	// Resume uses an independent disk, so the saved session remains reusable.
	restoredDisk := filepath.Join(dir, "resumed.raw")
	if err := os.WriteFile(restoredDisk, got, 0600); err != nil {
		t.Fatal(err)
	}
	restored, restoreCtx, _ := startSavedSessionTestQEMU(t, "-drive", "file="+qemuOptionValue(restoredDisk)+",format=raw,if=virtio", "-incoming", "defer")
	if err := restorePausedVMState(restoreCtx, restored, filepath.Join(destination, "memory.bin"), record.Memory); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "restored-memory.bin")
	if err := restored.Call(restoreCtx, "pmemsave", map[string]any{"val": 1048576, "size": len(pattern), "filename": output}, nil); err != nil {
		t.Fatal(err)
	}
	got, err = os.ReadFile(output)
	if err != nil || !bytes.Equal(got, pattern) {
		t.Fatalf("editor state changed: %v", err)
	}
	if err := restored.Call(restoreCtx, "cont", nil, nil); err != nil {
		t.Fatal(err)
	}
}

func writeSavedSessionFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	record := savedSessionRecord{Version: 1, Created: time.Now().UTC(), Identity: testSavedSessionIdentity()}
	for _, item := range []struct {
		name   string
		target *savedMemoryFile
	}{{"disk.raw", &record.Disk}, {"memory.bin", &record.Memory}} {
		path := filepath.Join(dir, item.name)
		if err := os.WriteFile(path, []byte(item.name+" contents"), 0600); err != nil {
			t.Fatal(err)
		}
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		*item.target, err = hashSavedMemory(context.Background(), f)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "saved-session.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestSavedSessionRejectsChangedStateBeforeRestore(t *testing.T) {
	for _, kind := range []string{"runtime", "machine", "architecture", "disk", "memory", "unknown-file", "metadata", "cancelled"} {
		t.Run(kind, func(t *testing.T) {
			dir := writeSavedSessionFixture(t)
			identity := testSavedSessionIdentity()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch kind {
			case "runtime":
				identity.RuntimeSHA256 = strings.Repeat("c", 64)
			case "machine":
				identity.MachineSHA256 = strings.Repeat("c", 64)
			case "architecture":
				identity.Architecture = "aarch64"
			case "disk", "memory":
				name := "disk.raw"
				if kind == "memory" {
					name = "memory.bin"
				}
				if err := os.WriteFile(filepath.Join(dir, name), []byte("corrupted"), 0600); err != nil {
					t.Fatal(err)
				}
			case "unknown-file":
				if err := os.WriteFile(filepath.Join(dir, "unrelated"), []byte("keep"), 0600); err != nil {
					t.Fatal(err)
				}
			case "metadata":
				if err := os.WriteFile(filepath.Join(dir, "saved-session.json"), []byte(`{"version":1,"unexpected":true}`), 0600); err != nil {
					t.Fatal(err)
				}
			case "cancelled":
				cancel()
			}
			before, err := os.ReadFile(filepath.Join(dir, "disk.raw"))
			if err != nil {
				t.Fatal(err)
			}
			session, err := openVerifiedSavedSession(ctx, dir, identity)
			if err == nil {
				session.Close()
				t.Fatal("accepted incompatible or damaged session")
			}
			after, err := os.ReadFile(filepath.Join(dir, "disk.raw"))
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("changed retained disk")
			}
		})
	}
}

func TestSavedSessionCaptureCancellationKeepsOriginal(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "active.raw")
	contents := bytes.Repeat([]byte("original"), 65536)
	if err := os.WriteFile(source, contents, 0600); err != nil {
		t.Fatal(err)
	}
	c, ctx, stop := startSavedSessionTestQEMU(t, "-drive", "file="+qemuOptionValue(source)+",format=raw,if=virtio")
	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	destination := filepath.Join(dir, "saved")
	_, err := capturePausedSession(cancelCtx, c, source, destination, testSavedSessionIdentity(), func(int64, int64, string) { cancel() })
	if err == nil {
		t.Fatal("published cancelled session")
	}
	if _, err := os.Lstat(destination); !os.IsNotExist(err) {
		t.Fatal("incomplete session was published")
	}
	var state vmRuntimeStatus
	if err := c.Call(ctx, "query-status", nil, &state); err != nil || state.Running {
		t.Fatalf("changed guest state: %+v %v", state, err)
	}
	stop()
	got, err := os.ReadFile(source)
	if err != nil || !bytes.Equal(got, contents) {
		t.Fatal("changed original disk")
	}
}

func TestSavedSessionCapturePreservesExistingDestination(t *testing.T) {
	directory := writeSavedSessionFixture(t)
	before, err := os.ReadFile(filepath.Join(directory, "saved-session.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := capturePausedSession(context.Background(), nil, "unused", directory, testSavedSessionIdentity(), nil); err == nil {
		t.Fatal("accepted existing session destination")
	}
	after, err := os.ReadFile(filepath.Join(directory, "saved-session.json"))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("changed existing saved session")
	}
}
