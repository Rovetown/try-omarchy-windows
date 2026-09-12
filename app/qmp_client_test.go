package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

func qmpTestPeer(t *testing.T, after func(net.Conn, *bufio.Reader)) *qmpClient {
	t.Helper()
	client, server := net.Pipe()
	t.Cleanup(func() { client.Close(); server.Close() })
	go func() {
		defer server.Close()
		server.SetDeadline(time.Now().Add(5 * time.Second))
		fmt.Fprintln(server, `{"QMP":{"version":{"qemu":{"major":11}},"capabilities":[]}}`)
		r := bufio.NewReader(server)
		line, err := r.ReadBytes('\n')
		if err != nil {
			return
		}
		var request struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(line, &request) != nil {
			return
		}
		fmt.Fprintf(server, "{\"return\":{},\"id\":%q}\n", request.ID)
		after(server, r)
	}()
	c, err := newQMPClient(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func qmpReadRequest(r *bufio.Reader) (string, error) {
	line, err := r.ReadBytes('\n')
	if err != nil {
		return "", err
	}
	var request struct {
		ID string `json:"id"`
	}
	err = json.Unmarshal(line, &request)
	return request.ID, err
}

func TestQMPClientCorrelatesEventsAndCommands(t *testing.T) {
	c := qmpTestPeer(t, func(conn net.Conn, r *bufio.Reader) {
		id, err := qmpReadRequest(r)
		if err != nil {
			return
		}
		fmt.Fprintln(conn, `{"event":"STOP","data":{}}`)
		fmt.Fprintf(conn, "{\"return\":{\"running\":false,\"status\":\"paused\"},\"id\":%q}\n", id)
	})
	var status vmRuntimeStatus
	if err := c.Call(context.Background(), "query-status", nil, &status); err != nil {
		t.Fatal(err)
	}
	if status.Running || status.Status != "paused" {
		t.Fatalf("unexpected status %+v", status)
	}
}

func TestQMPClientRemoteErrorAllowsAnotherCommand(t *testing.T) {
	c := qmpTestPeer(t, func(conn net.Conn, r *bufio.Reader) {
		id, err := qmpReadRequest(r)
		if err != nil {
			return
		}
		fmt.Fprintf(conn, "{\"error\":{\"class\":\"GenericError\",\"desc\":\"device cannot migrate\"},\"id\":%q}\n", id)
		id, err = qmpReadRequest(r)
		if err != nil {
			return
		}
		fmt.Fprintf(conn, "{\"return\":{},\"id\":%q}\n", id)
	})
	err := c.Call(context.Background(), "migrate", nil, nil)
	var remote *qmpCommandError
	if !errors.As(err, &remote) || remote.Description != "device cannot migrate" {
		t.Fatalf("lost remote error: %v", err)
	}
	if err := c.Call(context.Background(), "query-status", nil, nil); err != nil {
		t.Fatal(err)
	}
}

func TestQMPClientRejectsProtocolDamage(t *testing.T) {
	for name, reply := range map[string]string{
		"wrong-id":   `{"return":{},"id":"another-command"}`,
		"missing-id": `{"return":{}}`,
		"ambiguous":  `{"return":{},"event":"STOP"}`,
		"invalid":    `not json`,
		"too-large":  strings.Repeat("x", maxQMPMessage+1),
	} {
		t.Run(name, func(t *testing.T) {
			c := qmpTestPeer(t, func(conn net.Conn, r *bufio.Reader) {
				if _, err := qmpReadRequest(r); err == nil {
					fmt.Fprintln(conn, reply)
				}
			})
			if err := c.Call(context.Background(), "stop", nil, nil); err == nil {
				t.Fatal("accepted damaged reply")
			}
			if err := c.Call(context.Background(), "cont", nil, nil); err == nil || !strings.Contains(err.Error(), "unusable") {
				t.Fatalf("reused damaged stream: %v", err)
			}
		})
	}
}

func TestQMPClientCancellationClosesInterruptedStream(t *testing.T) {
	started := make(chan struct{})
	c := qmpTestPeer(t, func(conn net.Conn, r *bufio.Reader) {
		if _, err := qmpReadRequest(r); err != nil {
			return
		}
		close(started)
		r.ReadByte()
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- c.Call(ctx, "stop", nil, nil) }()
	<-started
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancellation did not interrupt read")
	}
	if err := c.Call(context.Background(), "cont", nil, nil); err == nil {
		t.Fatal("reused cancelled stream")
	}
}

func TestQMPClientRejectsFailedNegotiation(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()
	go func() { fmt.Fprintln(server, `{"return":{}}`) }()
	if c, err := newQMPClient(context.Background(), client); err == nil {
		c.Close()
		t.Fatal("accepted missing greeting")
	}
}

func TestInspectVMRuntimeReportsMigrationBlocker(t *testing.T) {
	c := qmpTestPeer(t, func(conn net.Conn, r *bufio.Reader) {
		for _, response := range []string{`{"running":true,"status":"running"}`, `[{"name":"query-migrate"},{"name":"snapshot-save"}]`, `{"blocked-reasons":["virgl is not yet migratable"]}`} {
			id, err := qmpReadRequest(r)
			if err != nil {
				return
			}
			fmt.Fprintf(conn, "{\"return\":%s,\"id\":%q}\n", response, id)
		}
	})
	capabilities, err := inspectVMRuntime(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	if !capabilities.Commands["snapshot-save"] || len(capabilities.MigrationBlockers) != 1 {
		t.Fatalf("lost runtime facts: %+v", capabilities)
	}
}
