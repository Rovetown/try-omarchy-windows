package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func checkpointFixture(t *testing.T) checkpointStore {
	t.Helper()
	dir, _ := backupFixture(t)
	if err := os.WriteFile(filepath.Join(dir, "guest", "build-spec.json"), []byte(`{"image":{"architecture":"x86_64"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	return checkpointStore{installation: dir}
}

func TestCheckpointRoundTripAndDeletePreserveCurrentState(t *testing.T) {
	s := checkpointFixture(t)
	disk := filepath.Join(s.installation, "vm", "disk.raw")
	before, _ := os.ReadFile(disk)
	first, err := s.Create("Before changing theme Ω", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(disk, []byte("new user work"), 0600); err != nil {
		t.Fatal(err)
	}
	second, err := s.Create(first.Name, nil)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == second.ID {
		t.Fatal("reused identity")
	}
	entries, err := s.List()
	if err != nil || len(entries) != 2 {
		t.Fatalf("list: %v %+v", err, entries)
	}
	restored := filepath.Join(filepath.Dir(s.installation), "restored-snapshot")
	if err := s.Restore(first.ID, restored, nil); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(restored, "vm", "disk.raw"))
	if !bytes.Equal(got, before) {
		t.Fatal("snapshot did not restore original bytes")
	}
	current, _ := os.ReadFile(disk)
	if string(current) != "new user work" {
		t.Fatal("changed current user work")
	}
	if err := s.Restore(first.ID, s.installation, nil); err == nil {
		t.Fatal("allowed restore over original")
	}
	if err := s.Restore(first.ID, filepath.Join(s.installation, "nested"), nil); err == nil {
		t.Fatal("allowed overlapping restore")
	}
	if err := s.Delete(first.ID); err != nil {
		t.Fatal(err)
	}
	entries, err = s.List()
	if err != nil || len(entries) != 1 || entries[0].ID != second.ID {
		t.Fatalf("deleted wrong state: %v %+v", err, entries)
	}
}

func TestCheckpointFailureDoesNotPublishOrLeaveStaging(t *testing.T) {
	for _, mode := range []string{"cancel", "space", "locked", "update"} {
		t.Run(mode, func(t *testing.T) {
			s := checkpointFixture(t)
			previous := diskFreeBytes
			t.Cleanup(func() { diskFreeBytes = previous; configureSetupCancellation(false) })
			var want error
			switch mode {
			case "cancel":
				want = errSetupCancelled
			case "space":
				diskFreeBytes = func(string) (int64, error) { return 0, nil }
				want = errInsufficientDiskSpace
			case "locked":
				f, err := openBackupDisk(filepath.Join(s.installation, "vm", "disk.raw"))
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()
			case "update":
				if err := os.WriteFile(filepath.Join(s.installation, updateStateFilename), []byte("pending"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			_, err := s.Create("Interrupted", func(int64, int64, string) {
				if mode == "cancel" {
					requestSetupCancel()
				}
			})
			if err == nil || want != nil && !errors.Is(err, want) {
				t.Fatalf("got %v, want %v", err, want)
			}
			entries, err := s.List()
			if err != nil || len(entries) != 0 {
				t.Fatalf("published failed snapshot: %v %+v", err, entries)
			}
			files, err := os.ReadDir(s.path())
			if err != nil {
				t.Fatal(err)
			}
			for _, f := range files {
				if strings.HasPrefix(f.Name(), ".pending-") {
					t.Fatal("left failed staging")
				}
			}
		})
	}
}

func TestCheckpointCorruptionIsIsolatedAndCannotRestore(t *testing.T) {
	s := checkpointFixture(t)
	entry, err := s.Create("Retained", nil)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(s.path(), entry.ID, "vm.zip")
	f, err := os.OpenFile(p, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.WriteAt([]byte("bad"), 0); err != nil {
		t.Fatal(err)
	}
	f.Close()
	destination := filepath.Join(filepath.Dir(s.installation), "damaged-restore")
	if err := s.Restore(entry.ID, destination, nil); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("accepted corruption: %v", err)
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatal("published corrupt restore")
	}
	if err := os.WriteFile(filepath.Join(s.path(), entry.ID, "snapshot.json"), []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	entries, err := s.List()
	if err != nil || len(entries) != 1 || entries[0].Problem == "" {
		t.Fatalf("damaged snapshot disappeared: %v %+v", err, entries)
	}
	if _, err := s.Create("Still usable", nil); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(entry.ID); err != nil {
		t.Fatal(err)
	}
}

func TestCheckpointDeleteRejectsUnknownFilesAndTraversal(t *testing.T) {
	s := checkpointFixture(t)
	entry, err := s.Create("Keep", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Delete("../guest"); err == nil {
		t.Fatal("accepted traversal")
	}
	p := filepath.Join(s.path(), entry.ID, "personal.txt")
	if err := os.WriteFile(p, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(entry.ID); err == nil {
		t.Fatal("deleted unrelated file")
	}
	if data, err := os.ReadFile(p); err != nil || string(data) != "keep" {
		t.Fatal("lost unrelated file")
	}
}

func TestCheckpointStoreRejectsLinkedDirectory(t *testing.T) {
	s := checkpointFixture(t)
	outside := t.TempDir()
	if err := os.Symlink(outside, s.path()); err != nil {
		t.Skip("symlink creation unavailable")
	}
	if _, err := s.Create("Unsafe", nil); err == nil {
		t.Fatal("followed snapshot store link")
	}
}

func TestCheckpointDeletePreservesUnexpectedNestedFiles(t *testing.T) {
	s := checkpointFixture(t)
	entry, err := s.Create("Before changes", nil)
	if err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(s.path(), entry.ID, "vm.zip")
	if err := os.Remove(archive); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(archive, 0700); err != nil {
		t.Fatal(err)
	}
	personal := filepath.Join(archive, "personal.txt")
	if err := os.WriteFile(personal, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(entry.ID); err == nil {
		t.Fatal("deleted unexpected directory")
	}
	if data, err := os.ReadFile(personal); err != nil || string(data) != "keep" {
		t.Fatal("lost personal file")
	}
}
