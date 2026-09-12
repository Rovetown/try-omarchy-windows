package main

import (
	"archive/zip"
	"bytes"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestClipboardFilesRoundTrip(t *testing.T) {
	source := t.TempDir()
	os.Mkdir(filepath.Join(source, "Empty"), 0700)
	os.WriteFile(filepath.Join(source, "hello 世界.txt"), []byte("hello\x00binary"), 0600)
	data, err := packClipboardFiles([]string{source})
	if err != nil {
		t.Fatal(err)
	}
	item, ok := decodeClipFrame(encodeClipFrame(clipItem{Kind: clipFiles, Data: data}))
	if !ok || item.Kind != clipFiles {
		t.Fatal("frame failed")
	}
	dest := t.TempDir()
	paths, err := unpackClipboardFiles(data, dest)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(paths[0], "hello 世界.txt"))
	if err != nil || string(got) != "hello\x00binary" {
		t.Fatalf("%q %v", got, err)
	}
	if i, err := os.Stat(filepath.Join(paths[0], "Empty")); err != nil || !i.IsDir() {
		t.Fatal("empty directory lost")
	}
	paths2, err := unpackClipboardFiles(data, dest)
	if err != nil || paths2[0] == paths[0] {
		t.Fatal("restore overwrote earlier clipboard files")
	}
}

func TestClipboardFilesRejectUnsafeNames(t *testing.T) {
	for _, name := range []string{"../escape", "/absolute", "C:/drive", "name:stream", "CON.txt", "dir/../file", "a\\b", "trailing.", "a\nfile", "LPT1", "dir//file"} {
		t.Run(name, func(t *testing.T) {
			var b bytes.Buffer
			z := zip.NewWriter(&b)
			w, _ := z.Create(name)
			w.Write([]byte("bad"))
			z.Close()
			dest := t.TempDir()
			if _, err := unpackClipboardFiles(b.Bytes(), dest); err == nil {
				t.Fatal("unsafe archive accepted")
			}
			entries, _ := os.ReadDir(dest)
			if len(entries) != 0 {
				t.Fatal("partial extraction")
			}
		})
	}
}

func TestClipboardFilesRejectLinksAndCollisions(t *testing.T) {
	source := t.TempDir()
	file := filepath.Join(source, "file")
	os.WriteFile(file, []byte("secret"), 0600)
	if err := os.Symlink(file, filepath.Join(source, "link")); err == nil {
		if _, err := packClipboardFiles([]string{source}); err == nil {
			t.Fatal("symlink copied")
		}
	}
	for _, names := range [][]string{{"A", "a"}, {"a", "a/child"}, {"A/x", "a/y"}} {
		var b bytes.Buffer
		z := zip.NewWriter(&b)
		for _, name := range names {
			w, _ := z.Create(name)
			w.Write([]byte("x"))
		}
		z.Close()
		if _, err := inspectClipboardArchive(b.Bytes()); err == nil {
			t.Fatal("collision accepted")
		}
	}
}

func TestClipboardFilesRejectExpansionAndCorruption(t *testing.T) {
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	w, _ := z.Create("large")
	w.Write(make([]byte, maxClipboardFileBytes+1))
	z.Close()
	if _, err := inspectClipboardArchive(b.Bytes()); err == nil {
		t.Fatal("oversize expanded archive accepted")
	}
	source := filepath.Join(t.TempDir(), "file")
	os.WriteFile(source, []byte("hello"), 0600)
	data, err := packClipboardFiles([]string{source})
	if err != nil {
		t.Fatal(err)
	}
	// Damage compressed data while retaining the directory, so extraction must fail
	// and remove its private staging directory.
	zr, _ := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	offset, _ := zr.File[0].DataOffset()
	data[offset] ^= 0xff
	dest := t.TempDir()
	if _, err := unpackClipboardFiles(data, dest); err == nil {
		t.Fatal("corrupt archive accepted")
	}
	entries, _ := os.ReadDir(dest)
	if len(entries) != 0 {
		t.Fatal("failed staging retained")
	}
}

func TestClipboardFilesPythonInterop(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("guest helper uses Linux descriptor semantics")
	}
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("Python not installed")
	}
	helper := filepath.Join("..", "scripts", "guest", "clipboard-files")
	source := filepath.Join(t.TempDir(), "hello 世界.txt")
	os.WriteFile(source, []byte("cross-platform\x00data"), 0600)
	uri := (&url.URL{Scheme: "file", Path: source}).String() + "\n"
	cmd := exec.Command(python, helper, "pack")
	cmd.Stdin = strings.NewReader(uri)
	data, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	paths, err := unpackClipboardFiles(data, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(paths[0])
	if string(got) != "cross-platform\x00data" {
		t.Fatal("Python to Go lost content")
	}
	data, err = packClipboardFiles([]string{source})
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	cmd = exec.Command(python, helper, "unpack")
	cmd.Env = append(os.Environ(), "HOME="+home)
	cmd.Stdin = bytes.NewReader(data)
	output, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(strings.TrimSpace(string(output)))
	if err != nil {
		t.Fatal(err)
	}
	got, err = os.ReadFile(parsed.Path)
	if err != nil || string(got) != "cross-platform\x00data" {
		t.Fatalf("Go to Python failed: %v", err)
	}
}
