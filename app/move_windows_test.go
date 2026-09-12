//go:build windows

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestMoveRejectsAlternateStreamsBeforeCopyAndCleanup(t *testing.T) {
	s, source, destination := moveFixture(t)
	if err := os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
		t.Fatal(err)
	}
	stream := filepath.Join(source, "settings.json") + ":extra"
	if err := os.WriteFile(stream, []byte("preserve this stream"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.prepare(source, destination, nil); err == nil {
		t.Fatal("silently dropped an alternate stream")
	}
	if err := os.Remove(stream); err != nil {
		t.Fatal(err)
	}
	prepareFixtureMove(t, s, source, destination)
	if err := s.recover(func(*installationMove) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := s.markBooted(destination); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stream, []byte("added after move"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.cleanup(destination); err == nil {
		t.Fatal("deleted a new alternate stream")
	}
	if _, err := os.Stat(filepath.Join(source, "vm", "disk.raw")); err != nil {
		t.Fatal("removed original disk despite failed stream check")
	}
}

func TestMoveUninstallRetiresOnlyMatchingRedirects(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	s := hostMoveStore()
	a, b, c, d := filepath.Join(t.TempDir(), "a"), filepath.Join(t.TempDir(), "b"), filepath.Join(t.TempDir(), "c"), filepath.Join(t.TempDir(), "d")
	state := moveState{Version: 1, Redirects: map[string]string{a: b, c: d}}
	if err := s.save(state); err != nil {
		t.Fatal(err)
	}
	if err := forgetMovedInstallation(b); err != nil {
		t.Fatal(err)
	}
	state, err := s.load()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := state.Redirects[a]; ok {
		t.Fatal("uninstalled destination still redirects fresh launches")
	}
	if state.Redirects[c] != d {
		t.Fatal("removed another installation's redirect")
	}
}

func TestMoveCopiesFilePastSpaceCheckThreshold(t *testing.T) {
	s, source, destination := moveFixture(t)
	data := bytes.Repeat([]byte{0x5a}, 33<<20)
	name := "large-file.bin"
	if err := os.WriteFile(filepath.Join(source, name), data, 0600); err != nil {
		t.Fatal(err)
	}
	prepareFixtureMove(t, s, source, destination)
	if err := s.recover(func(*installationMove) error { return nil }); err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{source, destination} {
		got, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, data) {
			t.Fatalf("file changed at %s", root)
		}
	}
}
