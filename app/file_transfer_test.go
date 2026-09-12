package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func transferTestLimits() fileTransferLimits {
	return fileTransferLimits{Entries: 100, Bytes: 128 << 20, ArchiveBytes: 128 << 20}
}

func TestFileTransferStreamsLargeSelectionAndEmptyFolders(t *testing.T) {
	source := filepath.Join(t.TempDir(), "Selected Ω")
	if err := os.MkdirAll(filepath.Join(source, "Empty"), 0700); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(filepath.Join(source, "large.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(80 << 20); err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteAt([]byte("last saved bytes"), (80<<20)-16); err != nil {
		t.Fatal(err)
	}
	file.Close()
	stamp := time.Date(2026, 1, 2, 3, 4, 6, 0, time.UTC)
	if err := os.Chtimes(filepath.Join(source, "large.bin"), stamp, stamp); err != nil {
		t.Fatal(err)
	}
	var prepareProgress, receiveProgress bool
	offer, archive, err := prepareFileTransfer(context.Background(), []string{source}, t.TempDir(), transferTestLimits(), func(int64, int64, string) { prepareProgress = true })
	if err != nil {
		t.Fatal(err)
	}
	if offer.FileBytes != 80<<20 || offer.Entries != 3 {
		t.Fatalf("incorrect offer: %+v", offer)
	}
	input, err := os.Open(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	destination := filepath.Join(t.TempDir(), "Received")
	if err := receiveFileTransfer(context.Background(), input, offer, destination, transferTestLimits(), func(int64, int64, string) { receiveProgress = true }); err != nil {
		t.Fatal(err)
	}
	restored, err := os.Open(filepath.Join(destination, "Selected Ω", "large.bin"))
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	if info, err := restored.Stat(); err != nil || info.ModTime().Unix() != stamp.Unix() {
		t.Fatal("file modification time lost")
	}
	original, err := os.Open(filepath.Join(source, "large.bin"))
	if err != nil {
		t.Fatal(err)
	}
	defer original.Close()
	a, b := sha256.New(), sha256.New()
	io.Copy(a, original)
	io.Copy(b, restored)
	if !bytes.Equal(a.Sum(nil), b.Sum(nil)) {
		t.Fatal("large file changed")
	}
	if info, err := os.Stat(filepath.Join(destination, "Selected Ω", "Empty")); err != nil || !info.IsDir() {
		t.Fatal("empty folder lost")
	}
	if !prepareProgress || !receiveProgress {
		t.Fatal("transfer did not report progress")
	}
}

func TestFileTransferFailureNeverPublishesOrChangesOriginal(t *testing.T) {
	for _, kind := range []string{"corrupt", "truncated", "extra", "cancelled", "quota", "existing", "inventory"} {
		t.Run(kind, func(t *testing.T) {
			source := filepath.Join(t.TempDir(), "document.txt")
			original := []byte("original document contents")
			if err := os.WriteFile(source, original, 0600); err != nil {
				t.Fatal(err)
			}
			offer, archive, err := prepareFileTransfer(context.Background(), []string{source}, t.TempDir(), transferTestLimits(), nil)
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(archive)
			if err != nil {
				t.Fatal(err)
			}
			destination := filepath.Join(t.TempDir(), "Received")
			limits := transferTestLimits()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var report backupProgress
			switch kind {
			case "corrupt":
				data[0] ^= 1
			case "truncated":
				data = data[:len(data)-1]
			case "extra":
				data = append(data, 0)
			case "cancelled":
				report = func(int64, int64, string) { cancel() }
			case "quota":
				limits.Bytes = 1
			case "existing":
				if err := os.Mkdir(destination, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(destination, "keep"), original, 0600); err != nil {
					t.Fatal(err)
				}
			case "inventory":
				offer.Entries++
			}
			err = receiveFileTransfer(ctx, bytes.NewReader(data), offer, destination, limits, report)
			if err == nil {
				t.Fatal("published failed transfer")
			}
			if kind == "cancelled" && !errors.Is(err, context.Canceled) {
				t.Fatalf("lost cancellation: %v", err)
			}
			if kind != "existing" {
				if _, err := os.Stat(destination); !os.IsNotExist(err) {
					t.Fatal("published incomplete destination")
				}
			} else {
				got, err := os.ReadFile(filepath.Join(destination, "keep"))
				if err != nil || !bytes.Equal(got, original) {
					t.Fatal("changed destination")
				}
			}
			siblings, err := os.ReadDir(filepath.Dir(destination))
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range siblings {
				if entry.Name() != "Received" {
					t.Fatalf("left staging: %s", entry.Name())
				}
			}
			got, err := os.ReadFile(source)
			if err != nil || !bytes.Equal(got, original) {
				t.Fatal("changed source")
			}
		})
	}
}

func TestFileTransferRejectsUnsafeArchiveEvenWithMatchingHash(t *testing.T) {
	for _, name := range []string{"../escape", "C:/drive", "folder/../escape", "CON.txt", "folder\\escape"} {
		t.Run(name, func(t *testing.T) {
			var data bytes.Buffer
			z := zip.NewWriter(&data)
			w, err := z.Create(name)
			if err != nil {
				t.Fatal(err)
			}
			w.Write([]byte("data"))
			z.Close()
			sum := sha256.Sum256(data.Bytes())
			offer := fileTransferOffer{Version: 1, ArchiveBytes: int64(data.Len()), FileBytes: 4, Entries: 1, SHA256: hex.EncodeToString(sum[:])}
			destination := filepath.Join(t.TempDir(), "Received")
			if err := receiveFileTransfer(context.Background(), bytes.NewReader(data.Bytes()), offer, destination, transferTestLimits(), nil); err == nil {
				t.Fatal("accepted unsafe archive")
			}
			if _, err := os.Stat(destination); !os.IsNotExist(err) {
				t.Fatal("published unsafe selection")
			}
		})
	}
}

func TestFileTransferPreparationCancellationAndLimitsCleanStaging(t *testing.T) {
	for _, kind := range []string{"cancelled", "bytes", "archive", "duplicate"} {
		t.Run(kind, func(t *testing.T) {
			source := filepath.Join(t.TempDir(), "file")
			if err := os.WriteFile(source, bytes.Repeat([]byte("data"), 1000), 0600); err != nil {
				t.Fatal(err)
			}
			cache := t.TempDir()
			limits := transferTestLimits()
			sources := []string{source}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var report backupProgress
			switch kind {
			case "cancelled":
				report = func(int64, int64, string) { cancel() }
			case "bytes":
				limits.Bytes = 1
			case "archive":
				limits.ArchiveBytes = 1
			case "duplicate":
				sources = append(sources, source)
			}
			if _, _, err := prepareFileTransfer(ctx, sources, cache, limits, report); err == nil {
				t.Fatal("accepted invalid selection")
			}
			entries, err := os.ReadDir(cache)
			if err != nil || len(entries) != 0 {
				t.Fatal("left failed transfer archive")
			}
		})
	}
}

func TestFileTransferProtectsDiskReserve(t *testing.T) {
	source := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(source, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	cache := t.TempDir()
	previous := diskFreeBytes
	diskFreeBytes = func(string) (int64, error) { return diskSpaceReserve + 32, nil }
	defer func() { diskFreeBytes = previous }()
	if _, _, err := prepareFileTransfer(context.Background(), []string{source}, cache, transferTestLimits(), nil); !errors.Is(err, errInsufficientDiskSpace) {
		t.Fatalf("reserve: %v", err)
	}
	entries, err := os.ReadDir(cache)
	if err != nil || len(entries) != 0 {
		t.Fatal("left failed archive")
	}
}

func TestFileTransferRejectsSourceChangedDuringCopy(t *testing.T) {
	source := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(source, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	cache := t.TempDir()
	changed := false
	_, _, err := prepareFileTransfer(context.Background(), []string{source}, cache, transferTestLimits(), func(int64, int64, string) {
		if changed {
			return
		}
		changed = true
		f, e := os.OpenFile(source, os.O_WRONLY|os.O_APPEND, 0600)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = f.Write([]byte(" changed")); e != nil {
			t.Fatal(e)
		}
		f.Close()
	})
	if err == nil {
		t.Fatal("accepted a changing source")
	}
	entries, err := os.ReadDir(cache)
	if err != nil || len(entries) != 0 {
		t.Fatal("left changed source archive")
	}
}

func TestFileTransferPreservesDestinationCreatedWhileReceiving(t *testing.T) {
	source := filepath.Join(t.TempDir(), "document")
	if err := os.WriteFile(source, []byte("contents"), 0600); err != nil {
		t.Fatal(err)
	}
	offer, archive, err := prepareFileTransfer(context.Background(), []string{source}, t.TempDir(), transferTestLimits(), nil)
	if err != nil {
		t.Fatal(err)
	}
	input, err := os.Open(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	destination := filepath.Join(t.TempDir(), "Received")
	created := false
	err = receiveFileTransfer(context.Background(), input, offer, destination, transferTestLimits(), func(int64, int64, string) {
		if !created {
			created = true
			if err := os.Mkdir(destination, 0700); err != nil {
				t.Fatal(err)
			}
		}
	})
	if err == nil {
		t.Fatal("replaced a destination created during transfer")
	}
	entries, err := os.ReadDir(destination)
	if err != nil || len(entries) != 0 {
		t.Fatal("changed concurrently created destination")
	}
}

func TestFileTransferGuestStreamingInterop(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("guest helper requires Linux")
	}
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("Python unavailable")
	}
	helper := filepath.Join("..", "scripts", "guest", "file-transfer")
	source := filepath.Join(t.TempDir(), "large 世界.bin")
	f, err := os.Create(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(80 << 20); err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteAt([]byte("cross-platform"), 4096); err != nil {
		t.Fatal(err)
	}
	f.Close()
	offer, archive, err := prepareFileTransfer(context.Background(), []string{source}, t.TempDir(), transferTestLimits(), nil)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(offer)
	if err != nil {
		t.Fatal(err)
	}
	offerPath := filepath.Join(t.TempDir(), "offer.json")
	if err := os.WriteFile(offerPath, encoded, 0600); err != nil {
		t.Fatal(err)
	}
	input, err := os.Open(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	destination := filepath.Join(t.TempDir(), "guest-received")
	command := exec.Command(python, helper, "receive", "--offer", offerPath, "--destination", destination)
	command.Stdin = input
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("guest receive: %v: %s", err, output)
	}
	received := filepath.Join(destination, filepath.Base(source))
	guestArchive := filepath.Join(t.TempDir(), "guest.zip")
	command = exec.Command(python, helper, "pack", "--archive", guestArchive, received)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("guest send: %v: %s", err, output)
	}
	if err := json.Unmarshal(output, &offer); err != nil {
		t.Fatalf("offer: %v: %s", err, output)
	}
	if offer.FileBytes != 80<<20 {
		t.Fatal("guest stream retained the small clipboard limit")
	}
	incoming, err := os.Open(guestArchive)
	if err != nil {
		t.Fatal(err)
	}
	defer incoming.Close()
	host := filepath.Join(t.TempDir(), "host-received")
	if err := receiveFileTransfer(context.Background(), incoming, offer, host, transferTestLimits(), nil); err != nil {
		t.Fatal(err)
	}
	original, err := os.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer original.Close()
	restored, err := os.Open(filepath.Join(host, filepath.Base(source)))
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	a, b := sha256.New(), sha256.New()
	io.Copy(a, original)
	io.Copy(b, restored)
	if !bytes.Equal(a.Sum(nil), b.Sum(nil)) {
		t.Fatal("guest round trip changed contents")
	}
}

func TestLegacyFileClipboardIdentityIgnoresMetadata(t *testing.T) {
	source := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(source, []byte("contents"), 0600); err != nil {
		t.Fatal(err)
	}
	first, err := packClipboardFiles([]string{source})
	if err != nil {
		t.Fatal(err)
	}
	stamp := time.Date(2025, 1, 2, 3, 4, 6, 0, time.UTC)
	if err := os.Chtimes(source, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	second, err := packClipboardFiles([]string{source})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("legacy clipboard metadata would break loop prevention")
	}
}

func TestFileTransferDoesNotArchiveItsOwnStorage(t *testing.T) {
	source := t.TempDir()
	cache := filepath.Join(source, "cache")
	if _, _, err := prepareFileTransfer(context.Background(), []string{source}, cache, transferTestLimits(), nil); err == nil {
		t.Fatal("prepared archive inside its selected source")
	}
	entries, err := os.ReadDir(cache)
	if err != nil || len(entries) != 0 {
		t.Fatal("left recursive transfer staging")
	}
}
