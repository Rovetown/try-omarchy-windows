package main

import (
	"bufio"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func testQMPStream(t *testing.T) (*qmpConn, net.Conn) {
	t.Helper()
	client, server := net.Pipe()
	scanner := bufio.NewScanner(client)
	scanner.Buffer(make([]byte, 4096), maxQMPMessage)
	c := &qmpConn{tcp: client, lines: scanner, done: make(chan struct{})}
	t.Cleanup(func() { c.close(); server.Close() })
	return c, server
}

func TestQMPStreamCloseReleasesBlockedDelivery(t *testing.T) {
	c, server := testQMPStream(t)
	lines := c.readLines()
	sent := make(chan struct{})
	go func() { io.WriteString(server, strings.Repeat("event\n", 128)); close(sent) }()
	deadline := time.After(time.Second)
	for len(lines) < cap(lines) {
		select {
		case <-deadline:
			t.Fatal("stream never filled")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	c.close()
	c.close()
	select {
	case <-sent:
	case <-time.After(time.Second):
		t.Fatal("socket write remained blocked")
	}
	done := make(chan struct{})
	go func() {
		for range lines {
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("stream delivery survived close")
	}
}

func TestQMPStreamRetainsFinalShutdownWithoutNewline(t *testing.T) {
	c, server := testQMPStream(t)
	lines := c.readLines()
	go func() { io.WriteString(server, `{"event":"SHUTDOWN","data":{"reason":"guest-reset"}}`); server.Close() }()
	select {
	case line := <-lines:
		if shutdownReason(line) != "reboot" {
			t.Fatalf("lost final reset: %s", line)
		}
	case <-time.After(time.Second):
		t.Fatal("stream did not finish")
	}
}

func TestQMPStreamBoundsRuntimeMessages(t *testing.T) {
	c, server := testQMPStream(t)
	lines := c.readLines()
	go func() { io.WriteString(server, strings.Repeat("x", maxQMPMessage+1)+"\n"); server.Close() }()
	select {
	case _, ok := <-lines:
		if ok {
			t.Fatal("accepted oversized runtime message")
		}
	case <-time.After(time.Second):
		t.Fatal("oversized stream did not terminate")
	}
}
