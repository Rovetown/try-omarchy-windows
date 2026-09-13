package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestStreamingClipboardGuestRoundTrip(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("guest Python helper runs on Linux")
	}
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("Python interop runs on Linux")
	}
	service := newFileTransferService(t.TempDir(), clipboardTransferLimits)
	defer service.Close()
	httpServer := httptest.NewServer(service)
	defer httpServer.Close()
	host, port, _ := net.SplitHostPort(strings.TrimPrefix(httpServer.URL, "http://"))
	push, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer push.Close()
	_, pushPort, _ := net.SplitHostPort(push.Addr().String())
	delivered := make(chan []string, 1)
	bridge := &clipBridge{transfers: service, setPaths: func(paths []string) bool { delivered <- paths; return true }}
	go bridge.acceptPush(push)
	source := filepath.Join(t.TempDir(), "large file.txt")
	// Exceed the old 16 MiB clipboard bound with highly compressible contents.
	original := strings.Repeat("streamed clipboard content\n", 800000)
	if err := os.WriteFile(source, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	ticket, err := service.Offer(context.Background(), []string{source}, nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := filepath.Join(t.TempDir(), "ticket.json")
	data, _ := json.Marshal(ticket)
	os.WriteFile(metadata, data, 0600)
	script := `import importlib.machinery, importlib.util, pathlib, sys, json
loader=importlib.machinery.SourceFileLoader('transfer',sys.argv[1])
spec=importlib.util.spec_from_loader(loader.name,loader)
m=importlib.util.module_from_spec(spec); loader.exec_module(m)
root=pathlib.Path(sys.argv[2]); state=root/'state'; state.mkdir()
ticket=json.loads(pathlib.Path(sys.argv[3]).read_text())
host, port, push=sys.argv[4],int(sys.argv[5]),int(sys.argv[6])
received=[]
def copy(*args,**kwargs): received.append(kwargs['input'])
m.subprocess.run=copy
m.os.environ['XDG_CACHE_HOME']=str(root/'cache')
result=m.clipboard_receive(ticket,state,host,port)
assert received and b'file://' in received[0]
assert m.clipboard_send(received[0],state,host,port,push)['state']=='unchanged'
(state/'last_file_selection').unlink()
assert m.clipboard_send(received[0],state,host,port,push)['state']=='completed'
assert m.clipboard_send(received[0],state,host,port,push)['state']=='unchanged'
`
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, python, "-c", script, filepath.Join("..", "scripts", "guest", "file-transfer"), t.TempDir(), metadata, host, port, pushPort)
	if data, err := command.CombinedOutput(); err != nil {
		t.Fatalf("guest clipboard: %v: %s", err, data)
	}
	select {
	case paths := <-delivered:
		if len(paths) != 1 {
			t.Fatal(paths)
		}
		content, err := os.ReadFile(paths[0])
		if err != nil || string(content) != original {
			t.Fatal("clipboard contents changed", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Windows clipboard adapter was not called")
	}
}

func TestClipboardTransferNegotiation(t *testing.T) {
	service := newFileTransferService(t.TempDir(), clipboardTransferLimits)
	defer service.Close()
	source := filepath.Join(t.TempDir(), "file")
	os.WriteFile(source, []byte("hello"), 0600)
	bridge := &clipBridge{transfers: service, getPaths: func() ([]string, bool) { return []string{source}, true }, getHost: func() (clipItem, bool) { return textItem("legacy"), true }}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go bridge.acceptPull(listener)
	for index, capable := range []bool{false, true} {
		conn, err := net.Dial("tcp", listener.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		conn.SetDeadline(time.Now().Add(3 * time.Second))
		if capable {
			conn.Write([]byte("transfer-v1\n"))
		}
		line, err := bufio.NewReader(conn).ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		item, ok := decodeClipFrame(line)
		if !ok {
			t.Fatal("bad frame", strconv.Itoa(index))
		}
		if capable && item.Kind != clipTransfer || !capable && item.Kind != clipText {
			t.Fatal(item.Kind)
		}
		conn.Close()
	}
	bridge.mu.Lock()
	if bridge.pullConn != nil {
		bridge.pullConn.Close()
	}
	bridge.mu.Unlock()
}

func TestClipboardTransferRejectsUnfinishedAndSuperseded(t *testing.T) {
	for _, mode := range []string{"unfinished", "superseded", "complete"} {
		t.Run(mode, func(t *testing.T) {
			service := newFileTransferService(t.TempDir(), clipboardTransferLimits)
			defer service.Close()
			source := filepath.Join(t.TempDir(), "file")
			os.WriteFile(source, []byte("original"), 0600)
			offered, err := service.Offer(context.Background(), []string{source}, nil)
			if err != nil {
				t.Fatal(err)
			}
			service.mu.Lock()
			archive, err := os.ReadFile(service.jobs[offered.Token].archive)
			service.mu.Unlock()
			if err != nil {
				t.Fatal(err)
			}
			var sequence atomic.Uint32
			var calls atomic.Int32
			bridge := &clipBridge{transfers: service, sequence: sequence.Load, setPaths: func([]string) bool { calls.Add(1); return true }}
			server := httptest.NewServer(service)
			defer server.Close()
			host, guest := net.Pipe()
			defer guest.Close()
			guest.SetDeadline(time.Now().Add(5 * time.Second))
			data, _ := json.Marshal(offered.Offer)
			done := make(chan struct{})
			go func() {
				defer close(done)
				defer host.Close()
				bridge.receiveClipboardTransfer(host, "files-offer:"+base64.StdEncoding.EncodeToString(data)+"\n")
			}()
			reader := bufio.NewReader(guest)
			line, err := reader.ReadBytes('\n')
			if err != nil {
				t.Fatal(err)
			}
			var incoming fileTransferTicket
			if err := json.Unmarshal(line, &incoming); err != nil {
				t.Fatal(err)
			}
			if mode != "unfinished" {
				response, err := server.Client().Post(server.URL+"/upload/"+incoming.Token, "application/zip", bytes.NewReader(archive))
				if err != nil {
					t.Fatal(err)
				}
				response.Body.Close()
				if response.StatusCode != 204 {
					t.Fatal(response.StatusCode)
				}
			}
			if mode == "superseded" {
				sequence.Add(1)
			}
			fmt.Fprintln(guest, "complete")
			reader.ReadString('\n')
			<-done
			want := int32(0)
			if mode == "complete" {
				want = 1
			}
			if calls.Load() != want {
				t.Fatal("unexpected clipboard publication", calls.Load())
			}
			if _, ok := service.Status(incoming.ID); ok {
				t.Fatal("completed control left a live capability")
			}
		})
	}
}

func TestClipboardTransferDoesNotSendReplacedSelection(t *testing.T) {
	service := newFileTransferService(t.TempDir(), clipboardTransferLimits)
	defer service.Close()
	source := filepath.Join(t.TempDir(), "file")
	os.WriteFile(source, []byte("original"), 0600)
	var sequence atomic.Uint32
	host, guest := net.Pipe()
	defer host.Close()
	defer guest.Close()
	bridge := &clipBridge{transfers: service, transferEnabled: true, pullConn: host, sequence: sequence.Load, getPaths: func() ([]string, bool) { sequence.Add(1); return []string{source}, true }}
	done := make(chan struct{})
	go func() { bridge.sendCurrentHost(host); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("obsolete selection attempted delivery")
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	if len(service.jobs) != 0 {
		t.Fatal("obsolete clipboard archive retained")
	}
}
