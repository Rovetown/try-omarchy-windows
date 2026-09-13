package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestSavedSessionMigrationFailureAndCancellation(t *testing.T) {
	for _, status := range []string{"failed", "cancelled", "postcopy-active", "active"} {
		t.Run(status, func(t *testing.T) {
			c := qmpTestPeer(t, func(conn net.Conn, r *bufio.Reader) {
				id, err := qmpReadRequest(r)
				if err != nil {
					return
				}
				fmt.Fprintf(conn, "{\"return\":{\"status\":%q,\"error-desc\":\"fixture failure\"},\"id\":%q}\n", status, id)
			})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			err := waitForVMMigration(ctx, c, func(vmMigrationProgress) {
				if status == "active" {
					cancel()
				}
			})
			if err == nil {
				t.Fatal("accepted incomplete transfer")
			}
			if status == "active" && !errors.Is(err, context.Canceled) {
				t.Fatalf("cancellation: %v", err)
			}
		})
	}
}

func TestSavedSessionRefusesDeviceBlockersBeforeCreatingOutput(t *testing.T) {
	c := qmpTestPeer(t, func(conn net.Conn, r *bufio.Reader) {
		for {
			line, err := r.ReadBytes('\n')
			if err != nil {
				return
			}
			var request struct {
				ID      string `json:"id"`
				Execute string `json:"execute"`
			}
			if json.Unmarshal(line, &request) != nil {
				return
			}
			var response any
			switch request.Execute {
			case "query-status":
				response = map[string]any{"running": false, "status": "paused"}
			case "query-commands":
				response = []map[string]string{{"name": "migrate"}, {"name": "query-migrate"}}
			case "query-migrate":
				response = map[string]any{"blocked-reasons": []string{"renderer state cannot be preserved"}}
			default:
				return
			}
			json.NewEncoder(conn).Encode(map[string]any{"return": response, "id": request.ID})
		}
	})
	path := filepath.Join(t.TempDir(), "memory.bin")
	if _, err := capturePausedVMState(context.Background(), c, path, nil); err == nil {
		t.Fatal("ignored migration blocker")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("created output for unsupported devices")
	}
}

func startSavedSessionTestQEMU(t *testing.T, extra ...string) (*qmpClient, context.Context, func()) {
	t.Helper()
	qemu := os.Getenv("QEMU_SYSTEM")
	if qemu == "" {
		var err error
		qemu, err = exec.LookPath("qemu-system-x86_64")
		if err != nil {
			t.Skip("QEMU unavailable")
		}
	}
	// Keep fixture IPC out of the real installation's control directory. Its
	// ACL may differ from this test process, and a concurrent launcher must not
	// share the fixture's socket namespace. Use a short path for AF_UNIX.
	ipcDir, err := os.MkdirTemp(os.TempDir(), "tom-ram-")
	if err != nil {
		t.Fatal(err)
	}
	previousControlDirectory := qmpControlDirectory
	qmpControlDirectory = func() (string, error) { return ipcDir, nil }
	t.Cleanup(func() {
		qmpControlDirectory = previousControlDirectory
		os.RemoveAll(ipcDir)
	})
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := l.Addr().String()
	l.Close()
	args := []string{"-machine", "q35,accel=tcg", "-m", "32", "-nodefaults", "-display", "none", "-S", "-qmp", "tcp:" + address + ",server=on,wait=off"}
	args = append(args, extra...)
	cmd := exec.Command(qemu, args...)
	configureDiskTool(cmd)
	log, err := os.Create(filepath.Join(t.TempDir(), "qemu.log"))
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = log
	if err := cmd.Start(); err != nil {
		log.Close()
		t.Fatal(err)
	}
	var once sync.Once
	stop := func() { once.Do(func() { cmd.Process.Kill(); cmd.Wait(); log.Close() }) }
	t.Cleanup(stop)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	var client *qmpClient
	for ctx.Err() == nil {
		client, err = dialQMPClient(ctx, address)
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		data, _ := os.ReadFile(log.Name())
		t.Fatalf("QEMU: %v: %s", err, data)
	}
	t.Cleanup(func() { client.Close() })
	return client, ctx, stop
}

func TestSavedSessionRAMSurvivesNewQEMUProcess(t *testing.T) {
	dir := t.TempDir()
	pattern := bytes.Repeat([]byte("unsaved application memory\x00\xff"), 1024)
	seed := filepath.Join(dir, "seed.bin")
	if err := os.WriteFile(seed, pattern, 0600); err != nil {
		t.Fatal(err)
	}
	c, ctx, stop := startSavedSessionTestQEMU(t, "-device", "loader,file="+qemuOptionValue(seed)+",addr=1048576,force-raw=on")
	path := filepath.Join(dir, "RAM,offset=literal Ω.bin")
	lastStatus := ""
	identity, err := capturePausedVMState(ctx, c, path, func(p vmMigrationProgress) {
		if p.Status != lastStatus {
			t.Logf("migration %s: transferred=%d remaining=%d total=%d", p.Status, p.RAM.Transferred, p.RAM.Remaining, p.RAM.Total)
			lastStatus = p.Status
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if identity.Bytes == 0 || !validSHA256(identity.SHA256) {
		t.Fatal("missing state identity")
	}
	stop()
	// The destination has no loader. These bytes can only come from the RAM
	// stream after the original process has exited, not from a fresh boot.
	restored, restoreCtx, _ := startSavedSessionTestQEMU(t, "-incoming", "defer")
	if err := restorePausedVMState(restoreCtx, restored, path, identity); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "restored-memory.bin")
	if err := restored.Call(restoreCtx, "pmemsave", map[string]any{"val": 1048576, "size": len(pattern), "filename": output}, nil); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(output)
	if err != nil || !bytes.Equal(got, pattern) {
		t.Fatalf("RAM was not preserved: %v", err)
	}
	if err := restored.Call(restoreCtx, "cont", nil, nil); err != nil {
		t.Fatal(err)
	}
	var status vmRuntimeStatus
	if err := restored.Call(restoreCtx, "query-status", nil, &status); err != nil || !status.Running {
		t.Fatalf("resume: %+v %v", status, err)
	}
}

func TestSavedSessionRejectsExistingOutputAndCorruptInput(t *testing.T) {
	c, ctx, _ := startSavedSessionTestQEMU(t)
	path := filepath.Join(t.TempDir(), "memory.bin")
	if err := os.WriteFile(path, []byte("keep existing session"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := capturePausedVMState(ctx, c, path, nil); err == nil {
		t.Fatal("overwrote existing state")
	}
	got, _ := os.ReadFile(path)
	if string(got) != "keep existing session" {
		t.Fatal("changed existing state")
	}
	if err := restorePausedVMState(ctx, c, path, savedMemoryFile{Bytes: int64(len(got)), SHA256: string(bytes.Repeat([]byte("0"), 64))}); err == nil {
		t.Fatal("accepted corrupt stream")
	}
	var status vmRuntimeStatus
	if err := c.Call(ctx, "query-status", nil, &status); err != nil || status.Running {
		t.Fatalf("changed runtime: %+v %v", status, err)
	}
}
