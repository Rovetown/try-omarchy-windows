//go:build !windows

package main

import (
	"context"
	"net"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestQMPClientAgainstQEMU(t *testing.T) {
	qemu, err := exec.LookPath("qemu-system-x86_64")
	if err != nil {
		t.Skip("QEMU not installed")
	}
	socket := filepath.Join(t.TempDir(), "qmp.sock")
	cmd := exec.Command(qemu, "-machine", "none", "-nodefaults", "-display", "none", "-S", "-qmp", "unix:"+socket+",server=on,wait=off")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var conn net.Conn
	for ctx.Err() == nil {
		conn, err = (&net.Dialer{}).DialContext(ctx, "unix", socket)
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	c, err := newQMPClient(ctx, conn)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	facts, err := inspectVMRuntime(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	if facts.State.Running || !facts.Commands["stop"] || !facts.Commands["cont"] {
		t.Fatalf("unexpected facts: %+v", facts)
	}
	for _, step := range []struct {
		command string
		running bool
	}{{"cont", true}, {"stop", false}} {
		if err := c.Call(ctx, step.command, nil, nil); err != nil {
			t.Fatal(err)
		}
		var state vmRuntimeStatus
		if err := c.Call(ctx, "query-status", nil, &state); err != nil {
			t.Fatal(err)
		}
		if state.Running != step.running {
			t.Fatalf("%s: %+v", step.command, state)
		}
	}
}
