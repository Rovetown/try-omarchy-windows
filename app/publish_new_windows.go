//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

func publishNewDirectory(from, to string) error {
	source, err := syscall.UTF16PtrFromString(from)
	if err != nil {
		return err
	}
	target, err := syscall.UTF16PtrFromString(to)
	if err != nil {
		return err
	}
	result, _, err := procMoveFileExW.Call(uintptr(unsafe.Pointer(source)), uintptr(unsafe.Pointer(target)), moveFileWriteThrough)
	if result == 0 {
		return err
	}
	return nil
}
