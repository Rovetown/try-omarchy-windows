//go:build windows

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckpointRecoveryPortableLaunchers(t *testing.T) {
	dir, payload := portableRecoveryFixture(t)
	root := filepath.Dir(dir)
	digest, ok := installReceiptArtifactSHA256(filepath.Join(dir, "guest"), "rootfs.ext4")
	if !ok {
		t.Fatal("missing fixture identity")
	}
	disk := filepath.Join(dir, "vm", "disk.qcow2")
	if err := createQcow2Overlay(disk, "../guest/rootfs.ext4", 4<<20); err != nil {
		t.Fatal(err)
	}
	if err := writePortableBackingState(disk, digest); err != nil {
		t.Fatal(err)
	}
	if err := createRollbackRecoveryLaunchers(dir, filepath.Join(filepath.Dir(payload), "data")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Start Omarchy.cmd", "Settings.cmd"} {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil || !strings.Contains(string(data), `"%~dp0TryOmarchy.exe" "-portable" "-no-update"`) {
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
