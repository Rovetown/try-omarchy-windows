package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func TestSaveRunningSessionRecoversOriginalAfterFailure(t *testing.T) {
	for _, failure := range []string{"missing-disk", "cancelled-copy", "lost-copy-connection"} {
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
			} else if failure == "cancelled-copy" {
				progress = func(int64, int64, string) { cancel() }
			} else {
				progress = func(int64, int64, string) { c.Close() }
			}
			coordinator := &savedSessionCoordinator{}
			destination := filepath.Join(dir, "saved")
			if _, err := coordinator.Save(operation, c, source, destination, testSavedSessionIdentity(), progress); err == nil {
				t.Fatal("save unexpectedly succeeded")
			}
			control, closeControl, err := savedSessionRecoveryConnection(ctx, c)
			if err != nil {
				t.Fatal(err)
			}
			defer closeControl()
			var state vmRuntimeStatus
			if err := control.Call(ctx, "query-status", nil, &state); err != nil {
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

func TestSavedSessionRecoversWhenStopReplyIsLost(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	operation, cancel := context.WithCancel(context.Background())
	defer cancel()
	var running atomic.Bool
	running.Store(true)
	var stops atomic.Int32
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			fmt.Fprintln(conn, `{"QMP":{"version":{"qemu":{"major":11,"minor":0,"micro":0}},"capabilities":[]}}`)
			reader := bufio.NewReader(conn)
			for {
				line, err := reader.ReadBytes('\n')
				if err != nil {
					break
				}
				var request struct {
					ID      string `json:"id"`
					Execute string `json:"execute"`
				}
				if json.Unmarshal(line, &request) != nil {
					break
				}
				var result any = map[string]any{}
				switch request.Execute {
				case "qmp_capabilities":
				case "query-status":
					state := "paused"
					if running.Load() {
						state = "running"
					}
					result = map[string]any{"running": running.Load(), "status": state}
				case "query-commands":
					names := []string{"stop", "cont", "migrate", "migrate_cancel", "query-migrate", "query-jobs", "blockdev-backup"}
					commands := []map[string]string{}
					for _, name := range names {
						commands = append(commands, map[string]string{"name": name})
					}
					result = commands
				case "query-migrate":
					result = map[string]any{"status": "none"}
				case "query-jobs":
					result = []any{}
				case "stop":
					running.Store(false)
					stops.Add(1)
					cancel() // The VM stopped, but the client never sees its acknowledgment.
					continue
				case "cont":
					running.Store(true)
				default:
					t.Errorf("unexpected recovery command %s", request.Execute)
				}
				data, _ := json.Marshal(result)
				fmt.Fprintf(conn, "{\"return\":%s,\"id\":%q}\n", data, request.ID)
			}
			conn.Close()
		}
	}()
	t.Cleanup(func() { listener.Close(); <-finished })
	c, err := dialQMPClient(context.Background(), listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	coordinator := &savedSessionCoordinator{}
	if _, err = coordinator.Save(operation, c, "unused", "unused", testSavedSessionIdentity(), nil); !errors.Is(err, context.Canceled) {
		t.Fatal("lost cancellation", err)
	}
	if !running.Load() {
		t.Fatal("guest remained paused after lost stop reply", err)
	}
	if stops.Load() != 1 {
		t.Fatal("recovery replayed the interrupted stop")
	}
}
