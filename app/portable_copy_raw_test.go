package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRawPortableCopyBudgetsUsedBlocks(t *testing.T) {
	tool := os.Getenv("QEMU_IMG")
	if tool == "" {
		var err error
		tool, err = exec.LookPath("qemu-img")
		if err != nil {
			t.Skip("qemu-img unavailable")
		}
	}
	dir, _ := backupFixture(t)
	raw := filepath.Join(dir, "vm", "disk.raw")
	f, err := os.OpenFile(raw, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := setSparse(f); err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(32 << 20); err != nil {
		t.Fatal(err)
	}
	f.Close()
	disk, err := inspectInstallationDisk(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Recovery data must not become an additional portable copy dependency.
	if err := os.MkdirAll(filepath.Join(dir, "checkpoints", "retained"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "checkpoints", "retained", "do-not-copy"), []byte("recovery"), 0600); err != nil {
		t.Fatal(err)
	}
	previous := diskFreeBytes
	t.Cleanup(func() { diskFreeBytes = previous })
	diskFreeBytes = func(string) (int64, error) { return diskSpaceReserve + 4<<20, nil }
	data := filepath.Join(t.TempDir(), "data")
	if err := stageRawPortableData(dir, data, disk, tool, nil); err != nil {
		t.Fatal(err)
	}
	for _, excluded := range []string{"vm/disk.raw", "checkpoints", "source.zip"} {
		if _, err := os.Lstat(filepath.Join(data, excluded)); !os.IsNotExist(err) {
			t.Fatalf("unexpected staging data: %s (%v)", excluded, err)
		}
	}
	for _, name := range []string{"guest/rootfs.ext4", "runtime/bin/qemu.exe", "settings.json"} {
		before, _ := os.ReadFile(filepath.Join(dir, name))
		after, err := os.ReadFile(filepath.Join(data, name))
		if err != nil || !bytes.Equal(before, after) {
			t.Fatalf("copy mismatch: %s (%v)", name, err)
		}
	}
	if info, err := os.Stat(filepath.Join(data, "vm", "disk.qcow2")); err != nil || info.Size() >= 4<<20 {
		t.Fatalf("portable disk is not compact: %v (%v)", info, err)
	}
	diskFreeBytes = func(string) (int64, error) { return diskSpaceReserve, nil }
	blocked := filepath.Join(t.TempDir(), "data")
	if err := stageRawPortableData(dir, blocked, disk, tool, nil); !errors.Is(err, errInsufficientDiskSpace) {
		t.Fatalf("expected space rejection, got %v", err)
	}
	if _, err := os.Lstat(blocked); !os.IsNotExist(err) {
		t.Fatalf("space rejection created output: %v", err)
	}
}
