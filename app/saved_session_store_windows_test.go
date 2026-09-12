//go:build windows

package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSavedSessionVerificationLocksDataAgainstWriters(t *testing.T) {
	dir := writeSavedSessionFixture(t)
	session, err := openVerifiedSavedSession(context.Background(), dir, testSavedSessionIdentity())
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	for _, name := range []string{"disk.raw", "memory.bin"} {
		path := filepath.Join(dir, name)
		if f, err := os.OpenFile(path, os.O_WRONLY, 0600); err == nil {
			f.Close()
			t.Fatal("verified state allowed a writer")
		}
		if err := os.Rename(path, path+".moved"); err == nil {
			t.Fatal("verified state allowed replacement")
		}
	}
	session.Close()
	again, err := openVerifiedSavedSession(context.Background(), dir, testSavedSessionIdentity())
	if err != nil {
		t.Fatal(err)
	}
	again.Close()
}
