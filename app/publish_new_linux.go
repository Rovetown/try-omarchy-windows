//go:build linux

package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"
)

// Linux development tests exercise the same no-replacement publication contract
// as MoveFileEx on Windows. A check followed by Rename would race other writers.
func publishNewDirectory(from, to string) error {
	// Flush newly created directory entries before making the tree visible.
	var directories []string
	if err := filepath.WalkDir(from, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("cannot publish linked transfer state")
		}
		if entry.IsDir() {
			directories = append(directories, name)
		}
		return nil
	}); err != nil {
		return err
	}
	for index := len(directories) - 1; index >= 0; index-- {
		dir, err := os.Open(directories[index])
		if err != nil {
			return err
		}
		err = dir.Sync()
		dir.Close()
		if err != nil {
			return err
		}
	}
	var number uintptr
	switch runtime.GOARCH {
	case "amd64":
		number = 316
	case "arm64", "riscv64", "loong64":
		number = 276
	default:
		return fmt.Errorf("atomic no-replacement publication is unavailable on this development architecture")
	}
	source, err := syscall.BytePtrFromString(from)
	if err != nil {
		return err
	}
	target, err := syscall.BytePtrFromString(to)
	if err != nil {
		return err
	}
	// renameat2(AT_FDCWD, from, AT_FDCWD, to, RENAME_NOREPLACE).
	_, _, errno := syscall.Syscall6(number, ^uintptr(99), uintptr(unsafe.Pointer(source)), ^uintptr(99), uintptr(unsafe.Pointer(target)), 1, 0)
	runtime.KeepAlive(source)
	runtime.KeepAlive(target)
	if errno != 0 {
		return &os.LinkError{Op: "renameat2", Old: from, New: to, Err: errno}
	}
	parent, err := os.Open(filepath.Dir(to))
	if err != nil {
		return err
	}
	defer parent.Close()
	return parent.Sync()
}
