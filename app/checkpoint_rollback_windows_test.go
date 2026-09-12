//go:build windows

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckpointRecoveryPortableLaunchers(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "data")
	if err := os.MkdirAll(filepath.Join(dir, "vm"), 0700); err != nil {
		t.Fatal(err)
	}
	digest := writePortableGuestReceipt(t, filepath.Join(dir, "guest"), []byte("factory"))
	disk := filepath.Join(dir, "vm", "disk.qcow2")
	if err := createQcow2Overlay(disk, "../guest/rootfs.ext4", 4<<20); err != nil {
		t.Fatal(err)
	}
	if err := writePortableBackingState(disk, digest); err != nil {
		t.Fatal(err)
	}
	if err := createRollbackRecoveryLaunchers(dir); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Start Omarchy.cmd", "Settings.cmd"} {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil || !strings.Contains(string(data), `"%~dp0TryOmarchy.exe" -portable`) {
			t.Fatalf("%s: %s %v", name, data, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, stableLauncherName)); err != nil {
		t.Fatal(err)
	}
	if _, err := inspectInstallationDisk(dir); err != nil {
		t.Fatal(err)
	}
}
