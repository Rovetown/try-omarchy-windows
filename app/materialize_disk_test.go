package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestMaterializePackagedRuntime(t *testing.T) {
	tool := os.Getenv("QEMU_IMG")
	if tool == "" {
		t.Skip("QEMU_IMG selects the runtime under test")
	}
	configureSetupCancellation(false)
	dir, _ := backupFixture(t)
	if err := os.Remove(filepath.Join(dir, "vm", "disk.raw")); err != nil {
		t.Fatal(err)
	}
	base := bytes.Repeat([]byte{0x3a}, 1<<20)
	digest := writePortableGuestReceipt(t, filepath.Join(dir, "guest"), base)
	overlay := filepath.Join(dir, "vm", "disk.qcow2")
	if err := createQcow2Overlay(overlay, "../guest/rootfs.ext4", 4<<20); err != nil {
		t.Fatal(err)
	}
	if err := writePortableBackingState(overlay, digest); err != nil {
		t.Fatal(err)
	}
	disk, err := inspectInstallationDisk(dir)
	if err != nil {
		t.Fatal(err)
	}
	out, cleanup, err := materializeInstallationDiskWithTool(filepath.Dir(dir), disk, tool)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 4<<20 || !bytes.Equal(data[:len(base)], base) || !zeroBytes(data[len(base):]) {
		t.Fatal("materialized data differs")
	}
	// Corruption must fail before a converted disk is returned.
	if err := os.WriteFile(disk.Backing, []byte("damaged"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, clean, err := materializeInstallationDiskWithTool(filepath.Dir(dir), disk, tool); err == nil {
		clean()
		t.Fatal("accepted corrupt backing")
	}
}

func TestMaterializeFailureCleansStaging(t *testing.T) {
	configureSetupCancellation(false)
	t.Cleanup(func() { configureSetupCancellation(false) })
	dir, _ := backupFixture(t)
	base := []byte("factory")
	digest := writePortableGuestReceipt(t, filepath.Join(dir, "guest"), base)
	disk := installationDisk{Format: "qcow2", Path: filepath.Join(dir, "vm", "disk.qcow2"), VirtualBytes: 4 << 20, Backing: filepath.Join(dir, "guest", "rootfs.ext4"), BackingSHA256: digest}
	parent := t.TempDir()
	for _, cancelled := range []bool{false, true} {
		configureSetupCancellation(false)
		if cancelled {
			requestSetupCancel()
		}
		if _, cleanup, err := materializeInstallationDiskWithTool(parent, disk, filepath.Join(dir, "missing-qemu-img")); err == nil {
			cleanup()
			t.Fatal("expected conversion failure")
		}
		files, err := os.ReadDir(parent)
		if err != nil || len(files) != 0 {
			t.Fatalf("left staging files: %v %v", files, err)
		}
	}
}
