package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallationDiskInventory(t *testing.T) {
	dir, _ := backupFixture(t)
	disk, err := inspectInstallationDisk(dir)
	if err != nil || disk.Format != "raw" {
		t.Fatalf("raw: %+v %v", disk, err)
	}
	if err := os.Remove(disk.Path); err != nil {
		t.Fatal(err)
	}
	digest := writePortableGuestReceipt(t, filepath.Join(dir, "guest"), []byte("factory"))
	overlay := filepath.Join(dir, "vm", "disk.qcow2")
	if err := createQcow2Overlay(overlay, "../guest/rootfs.ext4", 32<<20); err != nil {
		t.Fatal(err)
	}
	if err := writePortableBackingState(overlay, digest); err != nil {
		t.Fatal(err)
	}
	disk, err = inspectInstallationDisk(dir)
	if err != nil || disk.Format != "qcow2" || disk.VirtualBytes != 32<<20 || disk.BackingSHA256 != digest {
		t.Fatalf("portable: %+v %v", disk, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "vm", "disk.raw"), []byte("other disk"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := inspectInstallationDisk(dir); err == nil {
		t.Fatal("accepted ambiguous disk selection")
	}
}
