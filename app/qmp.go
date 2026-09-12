package main

import (
	"bufio"
	"context"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

// qmpConn is a handshaken QMP connection. A wedged QEMU accepts a socket
// connection but its main loop never answers, so only a completed greeting +
// qmp_capabilities exchange counts as "QEMU is alive" (the launch watchdog
// depends on that distinction).
type qmpConn struct {
	tcp       net.Conn
	lines     *bufio.Scanner
	done      chan struct{}
	closeOnce sync.Once
}

func qmpConnect(port int, readTimeout time.Duration) *qmpConn {
	path, err := qmpControlPath(port)
	if err != nil {
		logf("qmp: %v", err)
		return nil
	}
	tcp, err := net.DialTimeout("unix", path, 3*time.Second)
	if err != nil {
		logf("qmp %d: dial: %v", port, err)
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), readTimeout)
	defer cancel()
	client, err := newQMPClient(ctx, tcp)
	if err != nil {
		logf("qmp %d: handshake: %v", port, err)
		return nil
	}
	// The typed client reads synchronously, so its bounded scanner can now
	// become the supervisor's continuous event stream without another reader.
	return &qmpConn{tcp: tcp, lines: client.lines, done: make(chan struct{})}
}

func (c *qmpConn) writeLine(s string) error {
	if err := c.tcp.SetWriteDeadline(time.Now().Add(8 * time.Second)); err != nil {
		return err
	}
	data := []byte(s + "\n")
	n, err := c.tcp.Write(data)
	if err == nil && n != len(data) {
		return io.ErrShortWrite
	}
	return err
}

func (c *qmpConn) close() {
	c.closeOnce.Do(func() { close(c.done); c.tcp.Close() })
}

// readLines pumps every QMP line (events and command returns alike) into the
// returned channel and closes it when the stream ends. Keeping a read
// permanently pending matters: with -no-reboot, QEMU can exit so fast after a
// guest reset that a poll-style reader loses the SHUTDOWN event to an abortive
// socket close (see docs/FINDINGS.md).
func (c *qmpConn) readLines() <-chan string {
	ch := make(chan string, 16)
	go func() {
		defer close(ch)
		for c.lines.Scan() {
			select {
			case ch <- c.lines.Text():
			case <-c.done:
				return
			}
		}
	}()
	return ch
}

func shutdownReason(line string) string {
	if !strings.Contains(line, "\"event\"") || !strings.Contains(line, "\"SHUTDOWN\"") {
		return ""
	}
	if strings.Contains(line, "guest-reset") {
		return "reboot"
	}
	return "poweroff"
}
