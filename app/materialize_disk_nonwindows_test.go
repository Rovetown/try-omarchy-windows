//go:build !windows

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPortableBackupMaterializesRealDisk(t *testing.T) {
	tool, err := exec.LookPath("qemu-img")
	if err != nil {
		t.Skip("qemu-img unavailable")
	}
	dir, archive := backupFixture(t)
	if err := os.Remove(filepath.Join(dir, "vm", "disk.raw")); err != nil {
		t.Fatal(err)
	}
	base := make([]byte, 1<<20)
	copy(base, []byte("original factory data"))
	digest := writePortableGuestReceipt(t, filepath.Join(dir, "guest"), base)
	disk := filepath.Join(dir, "vm", "disk.qcow2")
	if err := createQcow2Overlay(disk, "../guest/rootfs.ext4", 4<<20); err != nil {
		t.Fatal(err)
	}
	if err := writePortableBackingState(disk, digest); err != nil {
		t.Fatal(err)
	}
	qemu, err := exec.LookPath("qemu-system-x86_64")
	if err != nil {
		t.Skip("QEMU runtime unavailable")
	}
	vm := exec.Command(qemu, "-machine", "none", "-nodefaults", "-display", "none", "-drive", "if=none,id=portable,format=qcow2,file="+disk, "-monitor", "stdio")
	vm.Stdin = strings.NewReader("qemu-io portable \"write -P 0x5a 65536 4096\"\nquit\n")
	if out, err := vm.CombinedOutput(); err != nil {
		t.Fatalf("guest write: %v %s", err, out)
	}
	original, err := os.ReadFile(disk)
	if err != nil {
		t.Fatal(err)
	}
	// The Android SDK tool resolves DLLs relative to its executable, so keep
	// that location while exercising the packaged-tool invocation contract.
	wrapper := "#!/bin/sh\nexec '" + strings.ReplaceAll(tool, "'", "'\"'\"'") + "' \"$@\"\n"
	if err := os.WriteFile(filepath.Join(dir, "runtime", "bin", "qemu-img"), []byte(wrapper), 0700); err != nil {
		t.Fatal(err)
	}
	if err := writeVMBackup(dir, archive); err != nil {
		t.Fatal(err)
	}
	restored := filepath.Join(filepath.Dir(dir), "restored")
	if err := restoreVMBackup(archive, restored); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(restored, "vm", "disk.raw"))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 4<<20 || string(data[:21]) != "original factory data" {
		t.Fatal("materialized disk content differs")
	}
	before, err := os.ReadFile(disk)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(portableBackingStatePath(disk)); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, original) {
		t.Fatal("source overlay changed during backup")
	}
	for _, b := range data[65536:69632] {
		if b != 0x5a {
			t.Fatal("guest writes lost during conversion")
		}
	}
	if len(before) == 0 {
		t.Fatal("lost source overlay")
	}
}
