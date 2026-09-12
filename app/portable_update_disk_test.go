package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestPortableDiskDetachmentSurvivesFactoryUpdate(t *testing.T) {
	tool := os.Getenv("QEMU_IMG")
	if tool == "" {
		var err error
		tool, err = exec.LookPath("qemu-img")
		if err != nil {
			t.Skip("qemu-img unavailable")
		}
	}
	configureSetupCancellation(false)
	dir, _ := backupFixture(t)
	if err := os.Remove(filepath.Join(dir, "vm", "disk.raw")); err != nil {
		t.Fatal(err)
	}
	base := bytes.Repeat([]byte{0x51}, 1<<20)
	digest := writePortableGuestReceipt(t, filepath.Join(dir, "guest"), base)
	diskPath := filepath.Join(dir, "vm", "disk.qcow2")
	if err := createQcow2Overlay(diskPath, "../guest/rootfs.ext4", 4<<20); err != nil {
		t.Fatal(err)
	}
	if err := writePortableBackingState(diskPath, digest); err != nil {
		t.Fatal(err)
	}
	disk, err := inspectInstallationDisk(dir)
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(diskPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := detachPortableDisk(dir, disk, tool, func(string, string) error { return fmt.Errorf("injected publication failure") }); err == nil {
		t.Fatal("ignored publication failure")
	}
	after, err := os.ReadFile(diskPath)
	if err != nil || !bytes.Equal(after, original) {
		t.Fatal("failed publication changed the current disk")
	}
	if err := detachPortableDisk(dir, disk, tool, publishMoveFile); err != nil {
		t.Fatal(err)
	}
	independent, err := inspectInstallationDisk(dir)
	if err != nil || independent.Backing != "" {
		t.Fatalf("still dependent: %+v %v", independent, err)
	}
	if err := os.WriteFile(disk.Backing, []byte("new factory image"), 0600); err != nil {
		t.Fatal(err)
	}
	raw, cleanup, err := materializeInstallationDiskWithTool(t.TempDir(), independent, tool)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	data, err := os.ReadFile(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data[:len(base)], base) {
		t.Fatal("factory update changed persistent guest contents")
	}
}
