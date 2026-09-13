//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

func configureDiskTool(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}
