//go:build windows

package main

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestQMPWindowsUnixSocketRuntime(t *testing.T) {
	qemu := os.Getenv("QEMU_SYSTEM")
	if qemu == "" {
		t.Skip("set QEMU_SYSTEM to test private Windows control sockets")
	}
	dir, err := os.MkdirTemp(os.TempDir(), "tom,qmp-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	previous := qmpControlDirectory
	qmpControlDirectory = func() (string, error) { return dir, nil }
	defer func() { qmpControlDirectory = previous }()
	if _, err := prepareQMPControl(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, qmpControlName(qmpToolsPort))
	cmd := exec.Command(qemu, "-machine", "none", "-nodefaults", "-display", "none", "-S", "-qmp", "unix:"+qemuOptionValue(path)+",server=on,wait=off")
	configureDiskTool(cmd)
	log, err := os.Create(filepath.Join(dir, "stderr.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	cmd.Stderr = log
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { cmd.Process.Kill(); cmd.Wait() }()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var conn net.Conn
	for ctx.Err() == nil {
		conn, err = (&net.Dialer{}).DialContext(ctx, "unix", path)
		if err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		data, _ := os.ReadFile(log.Name())
		t.Fatalf("private socket: %v: %s", err, data)
	}
	client, err := newQMPClient(ctx, conn)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	var state vmRuntimeStatus
	if err := client.Call(ctx, "query-status", nil, &state); err != nil {
		t.Fatal(err)
	}
	if state.Running {
		t.Fatal("unexpected running fixture")
	}
	if !isQMPControlSocket(path) {
		t.Fatal("Windows did not identify its AF_UNIX socket")
	}
	if _, err := prepareQMPControl(); err == nil {
		t.Fatal("replaced a live runtime socket")
	}
	client.Close()
	cmd.Process.Kill()
	cmd.Wait()
	if _, err := prepareQMPControl(); err != nil {
		t.Fatalf("stale control recovery: %v", err)
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatal("stale socket remained")
	}

}
