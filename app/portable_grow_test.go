package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func portableGrowthFixture(t *testing.T, backed bool) (string, installationDisk, string, []byte) {
	t.Helper()
	tool := os.Getenv("QEMU_IMG")
	if tool == "" {
		var err error
		tool, err = exec.LookPath("qemu-img")
		if err != nil {
			t.Skip("qemu-img unavailable")
		}
	}
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "vm"), 0700); err != nil {
		t.Fatal(err)
	}
	data := bytes.Repeat([]byte("saved user data\n"), 65536)
	digest := writePortableGuestReceipt(t, filepath.Join(dir, "guest"), data)
	path := filepath.Join(dir, "vm", "disk.qcow2")
	if backed {
		if err := createQcow2Overlay(path, "../guest/rootfs.ext4", 4<<20); err != nil {
			t.Fatal(err)
		}
		if err := writePortableBackingState(path, digest); err != nil {
			t.Fatal(err)
		}
	} else {
		cmd := exec.Command(tool, "convert", "-f", "raw", "-O", "qcow2", filepath.Join(dir, "guest", "rootfs.ext4"), path)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}
	disk, err := inspectInstallationDisk(dir)
	if err != nil {
		t.Fatal(err)
	}
	return dir, disk, tool, data
}
func TestPortableGrowthPreservesDataAndBacking(t *testing.T) {
	for _, backed := range []bool{false, true} {
		t.Run(fmt.Sprint(backed), func(t *testing.T) {
			configureSetupCancellation(false)
			dir, disk, tool, data := portableGrowthFixture(t, backed)
			if err := growPortableDiskWithTool(disk, 8<<20, tool, publishMoveFile); err != nil {
				t.Fatal(err)
			}
			grown, err := inspectInstallationDisk(dir)
			if err != nil {
				t.Fatal(err)
			}
			if grown.VirtualBytes != 8<<20 || grown.Backing != disk.Backing {
				t.Fatal("changed backing or capacity", grown)
			}
			raw, cleanup, err := materializeInstallationDiskWithTool(t.TempDir(), grown, tool)
			if err != nil {
				t.Fatal(err)
			}
			defer cleanup()
			got, err := os.ReadFile(raw)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got[:len(data)], data) || !bytes.Equal(got[len(data):], make([]byte, len(got)-len(data))) {
				t.Fatal("expansion changed contents or exposed nonzero new space")
			}
			if err := growPortableDiskWithTool(grown, 4<<20, tool, publishMoveFile); err != nil {
				t.Fatal(err)
			}
			after, err := inspectInstallationDisk(dir)
			if err != nil || after.VirtualBytes != 8<<20 {
				t.Fatal("shrunk existing disk", err)
			}
		})
	}
}
func TestPortableGrowthFailuresPreserveOriginal(t *testing.T) {
	for _, mode := range []string{"publish", "cancel", "space", "locked"} {
		t.Run(mode, func(t *testing.T) {
			configureSetupCancellation(false)
			t.Cleanup(func() { configureSetupCancellation(false) })
			_, disk, tool, _ := portableGrowthFixture(t, true)
			before, err := os.ReadFile(disk.Path)
			if err != nil {
				t.Fatal(err)
			}
			publish := publishMoveFile
			switch mode {
			case "publish":
				publish = func(string, string) error { return fmt.Errorf("injected publication failure") }
			case "cancel":
				requestSetupCancel()
			case "space":
				previous := diskFreeBytes
				diskFreeBytes = func(string) (int64, error) { return 0, nil }
				defer func() { diskFreeBytes = previous }()
			case "locked":
				f, err := openBackupDisk(disk.Path)
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()
			}
			err = growPortableDiskWithTool(disk, 8<<20, tool, publish)
			if err == nil {
				t.Fatal("ignored failure")
			}
			if mode != "locked" {
				after, err := os.ReadFile(disk.Path)
				if err != nil || !bytes.Equal(before, after) {
					t.Fatal("changed source on failure", err)
				}
			}
			stages, err := filepath.Glob(filepath.Join(filepath.Dir(disk.Path), ".try-omarchy-grow-*"))
			if err != nil || len(stages) != 0 {
				t.Fatal("left failed staging", err)
			}
		})
	}
}

func TestPortableGrowthRecoversOnlyOwnedInterruptedFiles(t *testing.T) {
	configureSetupCancellation(false)
	_, disk, tool, _ := portableGrowthFixture(t, true)
	parent := filepath.Dir(disk.Path)
	owned := filepath.Join(parent, ".try-omarchy-grow-1234.qcow2")
	unrelated := filepath.Join(parent, ".try-omarchy-grow-personal.qcow2")
	for _, path := range []string{owned, unrelated} {
		if err := os.WriteFile(path, []byte("keep unrelated contents"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := growPortableDiskWithTool(disk, disk.VirtualBytes, tool, publishMoveFile); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(owned); !os.IsNotExist(err) {
		t.Fatal("kept interrupted expansion", err)
	}
	if _, err := os.Stat(unrelated); err != nil {
		t.Fatal("removed unrelated file", err)
	}
}
