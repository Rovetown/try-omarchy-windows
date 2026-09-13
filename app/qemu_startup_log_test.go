package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQemuStartupFailureTail(t *testing.T) {
	dir := t.TempDir()
	if got := qemuStartupFailureTail(dir); got != "" {
		t.Fatalf("missing stderr: %q", got)
	}
	path := filepath.Join(dir, "qemu-stderr.log")
	original := strings.Repeat("old startup noise\n", 2000) + "original GPU failure\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	retained := qemuStartupFailureTail(dir)
	if len(retained) > 16*1024 || !strings.HasSuffix(retained, "original GPU failure") {
		t.Fatalf("incorrect bounded failure tail (%d bytes)", len(retained))
	}
	if err := os.WriteFile(path, []byte("CPU retry failure\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(retained, "original GPU failure") || qemuStartupFailureTail(dir) != "CPU retry failure" {
		t.Fatal("retry replaced the captured original failure")
	}
}
