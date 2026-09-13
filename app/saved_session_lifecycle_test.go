package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestSaveRunningSessionRecoversOriginalAfterFailure(t *testing.T) {
	for _, failure := range []string{"missing-disk", "cancelled-copy"} {
		t.Run(failure, func(t *testing.T) {
			dir := t.TempDir()
			disk := filepath.Join(dir, "active.raw")
			if err := os.WriteFile(disk, make([]byte, 1<<20), 0600); err != nil {
				t.Fatal(err)
			}
			c, ctx, _ := startSavedSessionTestQEMU(t, "-drive", "file="+qemuOptionValue(disk)+",format=raw,if=virtio")
			if err := c.Call(ctx, "cont", nil, nil); err != nil {
				t.Fatal(err)
			}
			operation, cancel := context.WithCancel(ctx)
			defer cancel()
			source := disk
			var progress backupProgress
			if failure == "missing-disk" {
				source = filepath.Join(dir, "missing.raw")
			} else {
				progress = func(int64, int64, string) { cancel() }
			}
			coordinator := &savedSessionCoordinator{}
			destination := filepath.Join(dir, "saved")
			if _, err := coordinator.Save(operation, c, source, destination, testSavedSessionIdentity(), progress); err == nil {
				t.Fatal("save unexpectedly succeeded")
			}
			var state vmRuntimeStatus
			if err := c.Call(ctx, "query-status", nil, &state); err != nil {
				t.Fatal(err)
			}
			if !state.Running {
				t.Fatalf("original guest not recovered: %+v", state)
			}
			if _, err := os.Stat(destination); !os.IsNotExist(err) {
				t.Fatal("incomplete session published", err)
			}
			if _, err := os.Stat(disk); err != nil {
				t.Fatal("original disk lost", err)
			}
		})
	}
}

func TestSaveRunningSessionLeavesVerifiedGuestStopped(t *testing.T) {
	dir := t.TempDir()
	disk := filepath.Join(dir, "active.raw")
	if err := os.WriteFile(disk, make([]byte, 1<<20), 0600); err != nil {
		t.Fatal(err)
	}
	c, ctx, _ := startSavedSessionTestQEMU(t, "-drive", "file="+qemuOptionValue(disk)+",format=raw,if=virtio")
	if err := c.Call(ctx, "cont", nil, nil); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(dir, "saved")
	coordinator := &savedSessionCoordinator{}
	record, err := coordinator.Save(ctx, c, disk, destination, testSavedSessionIdentity(), nil)
	if err != nil {
		t.Fatal(err)
	}
	verified, err := openVerifiedSavedSession(ctx, destination, testSavedSessionIdentity())
	if err != nil {
		t.Fatal(err)
	}
	defer verified.Close()
	if record != verified.Record {
		t.Fatal("published state changed")
	}
	var state vmRuntimeStatus
	if err := c.Call(ctx, "query-status", nil, &state); err != nil {
		t.Fatal(err)
	}
	if state.Running {
		t.Fatal("saved guest resumed before launcher exit")
	}
}

func TestSavedSessionBlockerDoesNotPauseGuest(t *testing.T) {
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
			if err := json.Unmarshal(line, &request); err != nil {
				return
			}
			var response any
			switch request.Execute {
			case "query-status":
				response = map[string]any{"running": true, "status": "running"}
			case "query-commands":
				response = []map[string]string{{"name": "query-migrate"}}
			case "query-migrate":
				response = map[string]any{"blocked-reasons": []string{"device state unsupported"}}
			default:
				t.Errorf("unsupported save mutated guest: %s", request.Execute)
				return
			}
			data, _ := json.Marshal(response)
			fmt.Fprintf(conn, "{\"return\":%s,\"id\":%q}\n", data, request.ID)
		}
	})
	coordinator := &savedSessionCoordinator{}
	if _, err := coordinator.Save(context.Background(), c, "unused", "unused", testSavedSessionIdentity(), nil); err == nil {
		t.Fatal("unsupported save accepted")
	}
}
