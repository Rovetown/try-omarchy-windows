//go:build !windows

package main

import "os/exec"

func configureDiskTool(cmd *exec.Cmd) {}
