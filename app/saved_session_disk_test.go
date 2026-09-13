package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSavedSessionDiskCancellationReleasesTemporaryNodes(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "active.raw")
	contents := bytes.Repeat([]byte("current work"), 65536)
	if err := os.WriteFile(source, contents, 0600); err != nil {
		t.Fatal(err)
	}
	c, ctx, _ := startSavedSessionTestQEMU(t, "-drive", "file="+qemuOptionValue(source)+",format=raw,if=virtio")
	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	destination := filepath.Join(dir, "partial.raw")
	_, err := copyPausedVMDisk(cancelCtx, c, source, destination, func(int64, int64, string) { cancel() })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
	var nodes []struct {
		Name string `json:"node-name"`
	}
	if err := c.Call(ctx, "query-named-block-nodes", nil, &nodes); err != nil {
		t.Fatal(err)
	}
	for _, node := range nodes {
		if strings.HasPrefix(node.Name, "tom-save-") {
			t.Fatal("cancelled target remained attached")
		}
	}
	var jobs []diskCopyJob
	if err := c.Call(ctx, "query-jobs", nil, &jobs); err != nil {
		t.Fatal(err)
	}
	for _, job := range jobs {
		if strings.HasPrefix(job.ID, "tom-save-job-") {
			t.Fatal("cancelled copy job remained")
		}
	}
	var state vmRuntimeStatus
	if err := c.Call(ctx, "query-status", nil, &state); err != nil || state.Running {
		t.Fatalf("guest resumed on failure: %+v %v", state, err)
	}
	lock, err := openBackupDisk(destination)
	if err != nil {
		t.Fatal(err)
	}
	lock.Close()
}

func TestSavedSessionCopiesOpenDiskThroughQEMU(t *testing.T) {
	for _, backed := range []bool{false, true} {
		name := "raw"
		if backed {
			name = "backed-qcow2"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			raw := filepath.Join(dir, "factory.raw")
			contents := make([]byte, 4<<20)
			copy(contents[8192:], bytes.Repeat([]byte("saved documents"), 4096))
			if err := os.WriteFile(raw, contents, 0600); err != nil {
				t.Fatal(err)
			}
			source, format := raw, "raw"
			if backed {
				tool := os.Getenv("QEMU_IMG")
				if tool == "" {
					var err error
					tool, err = exec.LookPath("qemu-img")
					if err != nil {
						t.Skip("qemu-img unavailable")
					}
				}
				source = filepath.Join(dir, "overlay.qcow2")
				format = "qcow2"
				if output, err := exec.Command(tool, "create", "-f", "qcow2", "-F", "raw", "-b", raw, source).CombinedOutput(); err != nil {
					t.Fatalf("%v: %s", err, output)
				}
			}
			c, ctx, _ := startSavedSessionTestQEMU(t, "-drive", "file="+qemuOptionValue(source)+",format="+format+",if=virtio")
			destination := filepath.Join(dir, "snapshot.raw")
			result, err := copyPausedVMDisk(ctx, c, source, destination, nil)
			if err != nil {
				t.Fatal(err)
			}
			if result.Path != destination || result.VirtualBytes != int64(len(contents)) {
				t.Fatalf("invalid result: %+v", result)
			}
			lock, err := openBackupDisk(destination)
			if err != nil {
				t.Fatalf("completed snapshot remains open: %v", err)
			}
			lock.Close()
			got, err := os.ReadFile(destination)
			if err != nil || !bytes.Equal(got, contents) {
				t.Fatalf("snapshot contents changed: %v", err)
			}
			var nodes []struct {
				Name string `json:"node-name"`
			}
			if err := c.Call(ctx, "query-named-block-nodes", nil, &nodes); err != nil {
				t.Fatal(err)
			}
			for _, node := range nodes {
				if strings.HasPrefix(node.Name, "tom-save-") {
					t.Fatal("temporary disk node remained attached")
				}
			}
			// An existing destination must remain byte-for-byte intact.
			if _, err := copyPausedVMDisk(ctx, c, source, destination, nil); err == nil {
				t.Fatal("replaced an existing snapshot")
			}
			got, _ = os.ReadFile(destination)
			if !bytes.Equal(got, contents) {
				t.Fatal("changed existing snapshot")
			}
		})
	}
}

func TestSavedSessionDiskBudgetIncludesBacking(t *testing.T) {
	allocated := int64(2 << 20)
	image := qmpDiskImage{VirtualSize: 64 << 20, ActualSize: &allocated, Backing: &qmpDiskImage{VirtualSize: 64 << 20, ActualSize: &allocated}}
	budget, err := qmpDiskCopyBudget(image)
	if err != nil || budget != 4<<20+diskSpaceReserve {
		t.Fatalf("backing allocation: %d %v", budget, err)
	}
	image.Backing.ActualSize = nil
	budget, err = qmpDiskCopyBudget(image)
	if err != nil || budget != 64<<20+diskSpaceReserve {
		t.Fatalf("unknown allocation: %d %v", budget, err)
	}
}
