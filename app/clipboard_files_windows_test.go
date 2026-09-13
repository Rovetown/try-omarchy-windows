//go:build windows

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// Opt in only on a disposable Windows desktop. This exercises actual Win32
// ownership and CF_HDROP publication, without opening Explorer or a Linux VM.
func TestWindowsFileClipboardNative(t *testing.T) {
	if os.Getenv("TRY_OMARCHY_TEST_CLIPBOARD") != "1" {
		t.Skip("requires disposable clipboard desktop")
	}
	t.Setenv("LOCALAPPDATA", t.TempDir())
	source := filepath.Join(t.TempDir(), "file 世界.txt")
	want := []byte("clipboard snapshot\x00binary")
	if err := os.WriteFile(source, want, 0600); err != nil {
		t.Fatal(err)
	}
	data, err := packClipboardFiles([]string{source})
	if err != nil {
		t.Fatal(err)
	}
	if !clipboardSetItem(clipItem{Kind: clipFiles, Data: data}) {
		t.Fatal("CF_HDROP publication failed")
	}
	if r, _, _ := procIsClipboardFormatAvail.Call(preferredDropEffectFormat); r == 0 {
		t.Fatal("copy drop effect missing")
	}
	item, ok := clipboardGetItem()
	if !ok || item.Kind != clipFiles {
		t.Fatal("CF_HDROP read failed")
	}
	paths, err := unpackClipboardFiles(item.Data, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(paths[0])
	if err != nil || !bytes.Equal(got, want) {
		t.Fatal("native roundtrip lost data")
	}
	if got, err = os.ReadFile(source); err != nil || !bytes.Equal(got, want) {
		t.Fatal("source modified")
	}
	if !clipboardSetItem(textItem("after files")) {
		t.Fatal("text publication failed")
	}
	item, ok = clipboardGetItem()
	if !ok || item.Kind != clipText || string(item.Data) != "after files" {
		t.Fatal("file-to-text transition failed")
	}
}

func TestWindowsStreamingClipboardPaths(t *testing.T) {
	if os.Getenv("TRY_OMARCHY_TEST_CLIPBOARD") != "1" {
		t.Skip("requires disposable clipboard desktop")
	}
	source := filepath.Join(t.TempDir(), "large 世界.txt")
	contents := bytes.Repeat([]byte("files"), 4<<20)
	if err := os.WriteFile(source, contents, 0600); err != nil {
		t.Fatal(err)
	}
	if !clipboardSetFilePaths([]string{source}) {
		t.Fatal("could not publish streamed paths")
	}
	defer clipboardSetItem(textItem("after streaming files"))
	paths, ok := clipboardGetFilePaths()
	if !ok || len(paths) != 1 || paths[0] != source {
		t.Fatal(paths, ok)
	}
	content, err := os.ReadFile(paths[0])
	if err != nil || !bytes.Equal(content, contents) {
		t.Fatal("streaming clipboard changed file", err)
	}
}
