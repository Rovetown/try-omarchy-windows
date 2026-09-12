package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCheckpointRollbackPortableAndArchitecture(t *testing.T) {
	tool := os.Getenv("QEMU_IMG")
	if tool == "" {
		var err error
		tool, err = exec.LookPath("qemu-img")
		if err != nil {
			t.Skip("qemu-img unavailable")
		}
	}
	s := checkpointFixture(t)
	disk := filepath.Join(s.installation, "vm", "disk.raw")
	before, _ := os.ReadFile(disk)
	entry, err := s.Create("Portable baseline", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(disk, []byte("portable current work"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := makeRestoredDiskPortable(s.installation, tool, nil); err != nil {
		t.Fatal(err)
	}
	retained, err := s.rollbackUsingTool(entry.ID, tool, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		dir  string
		want []byte
	}{{s.installation, before}, {retained, []byte("portable current work")}} {
		inventory, err := inspectInstallationDisk(item.dir)
		if err != nil || inventory.Format != "qcow2" || inventory.Backing != "" {
			t.Fatalf("portable inventory: %+v %v", inventory, err)
		}
		output := filepath.Join(t.TempDir(), "actual.raw")
		cmd := exec.Command(tool, "convert", "-f", "qcow2", "-O", "raw", inventory.Path, output)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
		got, _ := os.ReadFile(output)
		// QEMU pads tiny fixture images to a sector boundary.
		if len(got) < len(item.want) || !bytes.Equal(got[:len(item.want)], item.want) {
			t.Fatal("portable contents changed")
		}
	}
	if err := os.WriteFile(filepath.Join(s.installation, "guest", "build-spec.json"), []byte(`{"image":{"architecture":"aarch64"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.rollbackUsingTool(entry.ID, tool, nil); err == nil {
		t.Fatal("accepted cross-architecture rollback")
	}
}

func TestCheckpointRollbackCommittedRecoveryAndInvalidJournal(t *testing.T) {
	for _, committed := range []bool{false, true} {
		t.Run(fmt.Sprint(committed), func(t *testing.T) {
			dir := t.TempDir()
			state := checkpointRollbackState{Version: 1, ID: randomCheckpointRollbackID(), Committed: committed}
			stage := filepath.Join(dir, ".snapshot-rollback-"+state.ID)
			for _, side := range []string{"next", "data"} {
				if err := os.MkdirAll(filepath.Join(stage, side), 0700); err != nil {
					t.Fatal(err)
				}
			}
			for _, name := range checkpointRollbackNames() {
				enabled := name == "settings.json"
				state.Items = append(state.Items, checkpointRollbackItem{name, enabled, enabled})
			}
			if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte("new"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(stage, "data", "settings.json"), []byte("old"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := saveCheckpointRollback(dir, state); err != nil {
				t.Fatal(err)
			}
			if err := recoverCheckpointRollback(dir); err != nil {
				t.Fatal(err)
			}
			got, _ := os.ReadFile(filepath.Join(dir, "settings.json"))
			want := "old"
			if committed {
				want = "new"
			}
			if string(got) != want {
				t.Fatalf("got %s, want %s", got, want)
			}
			if err := recoverCheckpointRollback(dir); err != nil {
				t.Fatal(err)
			}
			state.Items[0].Name = "../outside"
			if err := saveCheckpointRollback(dir, state); err != nil {
				t.Fatal(err)
			}
			if err := recoverCheckpointRollback(dir); err == nil {
				t.Fatal("accepted unsafe journal")
			}
		})
	}
}

func TestCheckpointRollbackRetainsCurrentAndUnmanagedFiles(t *testing.T) {
	s := checkpointFixture(t)
	disk := filepath.Join(s.installation, "vm", "disk.raw")
	original, _ := os.ReadFile(disk)
	entry, err := s.Create("Known working", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(disk, []byte("new documents"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.installation, "unrelated.txt"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	retained, err := s.Rollback(entry.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(disk)
	if !bytes.Equal(got, original) {
		t.Fatal("restored disk differs")
	}
	got, _ = os.ReadFile(filepath.Join(retained, "vm", "disk.raw"))
	if string(got) != "new documents" {
		t.Fatal("lost current documents")
	}
	got, _ = os.ReadFile(filepath.Join(s.installation, "unrelated.txt"))
	if string(got) != "keep" {
		t.Fatal("changed unrelated file")
	}
	entries, err := s.List()
	if err != nil || len(entries) != 1 {
		t.Fatalf("lost catalog: %v", err)
	}
	if err := recoverCheckpointRollback(s.installation); err != nil {
		t.Fatal(err)
	}
	// Repeating rollback must retain the first recovery folder.
	second, err := s.Rollback(entry.ID, nil)
	if err != nil || second == retained {
		t.Fatalf("repeat: %s %v", second, err)
	}
	if got, _ := os.ReadFile(filepath.Join(retained, "vm", "disk.raw")); string(got) != "new documents" {
		t.Fatal("overwrote recovery")
	}
}

// Crash at every publication rename, then crash at every undo rename as well.
// The same journal must recover exactly once without losing either copy.
func TestCheckpointRollbackEveryInterruptedRename(t *testing.T) {
	names := checkpointRollbackNames()
	for cut := 0; cut <= 6; cut++ {
		for undoCut := 0; undoCut <= 6; undoCut++ {
			t.Run(fmt.Sprintf("publish%d-undo%d", cut, undoCut), func(t *testing.T) {
				dir := t.TempDir()
				state := checkpointRollbackState{Version: 1, ID: randomCheckpointRollbackID()}
				stage := filepath.Join(dir, ".snapshot-rollback-"+state.ID)
				for _, side := range []string{"next", "data"} {
					if err := os.MkdirAll(filepath.Join(stage, side), 0700); err != nil {
						t.Fatal(err)
					}
				}
				var moves []resetMove
				for index, name := range names {
					// Both sides, removed optional value, added optional value.
					old, next := index < 3, index == 0 || index == 1 || index == 3
					state.Items = append(state.Items, checkpointRollbackItem{name, old, next})
					if old {
						if err := os.WriteFile(filepath.Join(dir, name), []byte("old"+name), 0600); err != nil {
							t.Fatal(err)
						}
						moves = append(moves, resetMove{filepath.Join(dir, name), filepath.Join(stage, "data", name)})
					}
					if next {
						if err := os.WriteFile(filepath.Join(stage, "next", name), []byte("next"+name), 0600); err != nil {
							t.Fatal(err)
						}
						moves = append(moves, resetMove{filepath.Join(stage, "next", name), filepath.Join(dir, name)})
					}
				}
				if err := saveCheckpointRollback(dir, state); err != nil {
					t.Fatal(err)
				}
				for _, move := range moves[:cut] {
					if err := os.Rename(move.from, move.to); err != nil {
						t.Fatal(err)
					}
				}
				calls := 0
				_ = undoCheckpointRollback(dir, stage, state, func(from, to string) error {
					if calls == undoCut {
						return fmt.Errorf("interruption")
					}
					calls++
					return os.Rename(from, to)
				})
				// Fixture roots are plain files; exercise the same undo engine
				// without the production disk-lock inspection of vm directories.
				if err := undoCheckpointRollback(dir, stage, state, os.Rename); err != nil {
					t.Fatal(err)
				}
				for _, item := range state.Items {
					got, err := os.ReadFile(filepath.Join(dir, item.Name))
					if item.Previous && (err != nil || string(got) != "old"+item.Name) {
						t.Fatalf("lost %s: %v", item.Name, err)
					}
					if !item.Previous && !os.IsNotExist(err) {
						t.Fatalf("unexpected active %s", item.Name)
					}
					if item.Next {
						got, err = os.ReadFile(filepath.Join(stage, "next", item.Name))
						if err != nil || string(got) != "next"+item.Name {
							t.Fatalf("lost staged %s: %v", item.Name, err)
						}
					}
				}
			})
		}
	}
}

func TestCheckpointRollbackRejectsCorruptionAndBusyDisk(t *testing.T) {
	for _, mode := range []string{"corrupt", "busy", "cancel", "pending-update"} {
		t.Run(mode, func(t *testing.T) {
			s := checkpointFixture(t)
			entry, err := s.Create("Before", nil)
			if err != nil {
				t.Fatal(err)
			}
			disk := filepath.Join(s.installation, "vm", "disk.raw")
			before, _ := os.ReadFile(disk)
			switch mode {
			case "corrupt":
				if err := os.WriteFile(filepath.Join(s.path(), entry.ID, "vm.zip"), []byte("broken"), 0600); err != nil {
					t.Fatal(err)
				}
			case "busy":
				lock, err := openBackupDisk(disk)
				if err != nil {
					t.Fatal(err)
				}
				defer lock.Close()
			case "pending-update":
				if err := os.WriteFile(filepath.Join(s.installation, updateStateFilename), []byte("pending"), 0600); err != nil {
					t.Fatal(err)
				}
			case "cancel":
				configureSetupCancellation(false)
				requestSetupCancel()
				defer configureSetupCancellation(false)
			}
			if _, err := s.Rollback(entry.ID, nil); err == nil {
				t.Fatal("accepted unsafe rollback")
			}
			got, _ := os.ReadFile(disk)
			if !bytes.Equal(got, before) {
				t.Fatal("changed active disk")
			}
			if _, err := os.Stat(filepath.Join(s.installation, checkpointRollbackFile)); !os.IsNotExist(err) {
				t.Fatal("published journal before verification")
			}
		})
	}
}
