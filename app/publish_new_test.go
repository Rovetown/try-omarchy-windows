package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPublishNewDirectoryNeverReplacesExistingTarget(t *testing.T) {
	for _, file := range []bool{false, true} {
		name := "directory"
		if file {
			name = "file"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "source")
			target := filepath.Join(root, "target")
			if err := os.Mkdir(source, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(source, "new"), []byte("new"), 0600); err != nil {
				t.Fatal(err)
			}
			var err error
			if file {
				err = os.WriteFile(target, []byte("existing"), 0600)
			} else {
				err = os.Mkdir(target, 0700)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := publishNewDirectory(source, target); err == nil {
				t.Fatal("replaced an existing target")
			}
			if _, err := os.Stat(filepath.Join(source, "new")); err != nil {
				t.Fatal("lost source")
			}
			if file {
				data, err := os.ReadFile(target)
				if err != nil || string(data) != "existing" {
					t.Fatal("changed target")
				}
			} else {
				entries, err := os.ReadDir(target)
				if err != nil || len(entries) != 0 {
					t.Fatal("changed target")
				}
			}
		})
	}
}
