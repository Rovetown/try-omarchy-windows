//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

func TestWindowsShellDropData(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ole := syscall.NewLazyDLL("ole32.dll")
	hr, _, _ := ole.NewProc("OleInitialize").Call(0)
	if int32(hr) < 0 {
		t.Fatal("OLE initialization failed")
	}
	defer ole.NewProc("OleUninitialize").Call()
	root := t.TempDir()
	paths := []string{filepath.Join(root, "first 世界.txt"), filepath.Join(root, "second.txt")}
	for _, path := range paths {
		if err := os.WriteFile(path, []byte("preserved"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	sequence := clipboardSequence()
	err := withHostDropData(paths, func(object uintptr) error {
		type formatEtc struct {
			format uint16
			target uintptr
			aspect uint32
			index  int32
			medium uint32
		}
		type storageMedium struct {
			kind   uint32
			handle uintptr
			owner  uintptr
		}
		format := formatEtc{format: cfHDrop, aspect: 1, index: -1, medium: 1}
		var medium storageMedium
		table := *(*uintptr)(unsafe.Pointer(object))
		getData := *(*uintptr)(unsafe.Pointer(table + 3*unsafe.Sizeof(uintptr(0))))
		result, _, _ := syscall.SyscallN(getData, object, uintptr(unsafe.Pointer(&format)), uintptr(unsafe.Pointer(&medium)))
		if int32(result) < 0 {
			return fmt.Errorf("Shell did not provide CF_HDROP: %x", result)
		}
		defer ole.NewProc("ReleaseStgMedium").Call(uintptr(unsafe.Pointer(&medium)))
		count, _, _ := shell32.NewProc("DragQueryFileW").Call(medium.handle, 0xffffffff, 0, 0)
		if count != uintptr(len(paths)) {
			return fmt.Errorf("unexpected dropped path count %d", count)
		}
		for i, want := range paths {
			var path [32769]uint16
			shell32.NewProc("DragQueryFileW").Call(medium.handle, uintptr(i), uintptr(unsafe.Pointer(&path[0])), uintptr(len(path)))
			if syscall.UTF16ToString(path[:]) != want {
				return fmt.Errorf("Shell changed dropped path")
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if clipboardSequence() != sequence {
		t.Fatal("drag data preparation changed clipboard")
	}
}

func TestNativeFileDropWindow(t *testing.T) {
	if os.Getenv("TRYOMARCHY_NATIVE_UI_TEST") != "1" {
		t.Skip("interactive Windows desktop required")
	}
	path := filepath.Join(t.TempDir(), "received.txt")
	os.WriteFile(path, []byte("received"), 0600)
	done := make(chan struct{})
	go func() { defer close(done); showFileDropWindow([]string{path}) }()
	class, _ := syscall.UTF16PtrFromString("TryOmarchyFileDrops")
	var hwnd uintptr
	deadline := time.Now().Add(5 * time.Second)
	for hwnd == 0 && time.Now().Before(deadline) {
		hwnd, _, _ = user32.NewProc("FindWindowW").Call(uintptr(unsafe.Pointer(class)), 0)
		time.Sleep(20 * time.Millisecond)
	}
	if hwnd == 0 {
		t.Fatal("file transfer window did not open")
	}
	defer procPostMessageW.Call(hwnd, wmClose, 0, 0)
	var count uintptr
	for time.Now().Before(deadline) {
		list, _, _ := user32.NewProc("GetDlgItem").Call(hwnd, 4600)
		count, _, _ = procSendMessageW.Call(list, 0x18b, 0, 0)
		if count == 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if count != 1 {
		t.Fatal("received file not listed", count)
	}
	procPostMessageW.Call(hwnd, wmCommand, 2, 0)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("file transfer window did not close")
	}
}
