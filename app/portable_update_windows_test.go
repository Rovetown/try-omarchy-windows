//go:build windows

package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPortableUpdateHelperProcess(t *testing.T) {
	marker := os.Getenv("TRYOMARCHY_UPDATE_TEST_MARKER")
	if marker == "" {
		return
	}
	self, err := os.Executable()
	if err != nil {
		os.Exit(2)
	}
	if err := os.WriteFile(marker, []byte(self), 0600); err != nil {
		os.Exit(3)
	}
	os.Exit(0)
}

func TestPortableLauncherReplacementUsesBundleRoot(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "data")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, stableLauncherName)
	if err := os.WriteFile(target, []byte("previous launcher"), 0700); err != nil {
		t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	state := &launcherUpdateState{Schema: updateStateVersion, Version: currentVersion, SHA256: testSHA256(data), Portable: true}
	if err := writeLauncherUpdateState(dir, state); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(root, "started.txt")
	t.Setenv("TRYOMARCHY_UPDATE_TEST_MARKER", marker)
	args, err := encodeRestartArgs([]string{"-test.run=^TestPortableUpdateHelperProcess$"})
	if err != nil {
		t.Fatal(err)
	}
	if err := applyLauncherUpdate(dir, 2147483647, args, false); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		data, err = os.ReadFile(marker)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("updated launcher did not restart")
		}
		time.Sleep(25 * time.Millisecond)
	}
	if !pathsEqual(string(data), target) {
		t.Fatalf("started wrong executable: %s", data)
	}
	previous, err := os.ReadFile(previousLauncherPath(dir))
	if err != nil || string(previous) != "previous launcher" {
		t.Fatal("previous launcher was not retained")
	}
	state, err = readLauncherUpdateState(dir)
	if err != nil || state == nil || !state.Portable || !state.HasPrevious {
		t.Fatalf("lost rollback state: %+v %v", state, err)
	}
	if _, err := os.Stat(filepath.Join(dir, stableLauncherName)); !os.IsNotExist(err) {
		t.Fatal("placed portable launcher inside data")
	}
}
