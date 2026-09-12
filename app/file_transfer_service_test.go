package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFileTransferServiceCapabilitiesRetryAndPublication(t *testing.T) {
	source := filepath.Join(t.TempDir(), "document.txt")
	original := []byte(strings.Repeat("original contents\n", 1000))
	if err := os.WriteFile(source, original, 0600); err != nil {
		t.Fatal(err)
	}
	service := newFileTransferService(t.TempDir(), transferTestLimits())
	defer service.Close()
	ticket, err := service.Offer(context.Background(), []string{source}, nil)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(service)
	defer server.Close()
	get := func(path string, start string) (int, []byte) {
		t.Helper()
		req, _ := http.NewRequest("GET", server.URL+path, nil)
		if start != "" {
			req.Header.Set("Range", start)
		}
		response, err := server.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		data, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		return response.StatusCode, data
	}
	if code, _ := get("/download/"+strings.Repeat("0", 64), ""); code != 404 {
		t.Fatalf("unauthorized: %d", code)
	}
	code, archive := get("/download/"+ticket.Token, "")
	if code != 200 || int64(len(archive)) != ticket.Offer.ArchiveBytes {
		t.Fatal("download failed", code)
	}
	if code, data := get("/download/"+ticket.Token, "bytes=7-19"); code != 206 || !bytes.Equal(data, archive[7:20]) {
		t.Fatal("range retry failed", code)
	}
	destination := filepath.Join(t.TempDir(), "Received")
	incoming, err := service.AcceptReceive(ticket.Offer, destination)
	if err != nil {
		t.Fatal(err)
	}
	post := func(data []byte) int {
		t.Helper()
		response, err := server.Client().Post(server.URL+"/upload/"+incoming.Token, "application/zip", bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, response.Body)
		response.Body.Close()
		return response.StatusCode
	}
	corrupt := append([]byte(nil), archive...)
	corrupt[0] ^= 1
	if post(corrupt) != 400 {
		t.Fatal("corruption accepted")
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatal("failed transfer published")
	}
	if post(archive) != 204 {
		t.Fatal("retry failed")
	}
	if post(archive) != 204 {
		t.Fatal("completed retry not idempotent")
	}
	received, err := os.ReadFile(filepath.Join(destination, "document.txt"))
	if err != nil || !bytes.Equal(received, original) {
		t.Fatal("received content changed", err)
	}
	if status, ok := service.Status(incoming.ID); !ok || status.State != "completed" {
		t.Fatal(status, ok)
	}
	if !service.Cancel(ticket.ID) {
		t.Fatal("cancel failed")
	}
	if code, _ := get("/download/"+ticket.Token, ""); code != 404 {
		t.Fatal("cancelled capability usable")
	}
	service.Close()
	if received, err := os.ReadFile(filepath.Join(destination, "document.txt")); err != nil || !bytes.Equal(received, original) {
		t.Fatal("close removed received data")
	}
}

func TestFileTransferServiceRejectsUnacceptedDestinations(t *testing.T) {
	service := newFileTransferService(t.TempDir(), transferTestLimits())
	defer service.Close()
	source := filepath.Join(t.TempDir(), "source")
	os.WriteFile(source, []byte("contents"), 0600)
	ticket, err := service.Offer(context.Background(), []string{source}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, destination := range []string{"relative", source, t.TempDir()} {
		if _, err := service.AcceptReceive(ticket.Offer, destination); err == nil {
			t.Fatalf("accepted %s", destination)
		}
	}
	service.Close()
	if _, err := service.AcceptReceive(ticket.Offer, filepath.Join(t.TempDir(), "new")); err == nil {
		t.Fatal("closed service accepted transfer")
	}
}

func TestFileTransferServiceCancelStalledUpload(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source")
	os.WriteFile(source, []byte("contents"), 0600)
	service := newFileTransferService(t.TempDir(), transferTestLimits())
	defer service.Close()
	ticket, err := service.Offer(context.Background(), []string{source}, nil)
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "Received")
	incoming, err := service.AcceptReceive(ticket.Offer, destination)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(service)
	defer server.Close()
	conn, err := net.Dial("tcp", strings.TrimPrefix(server.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	fmt.Fprintf(conn, "POST /upload/%s HTTP/1.1\r\nHost: localhost\r\nContent-Length: %d\r\n\r\nX", incoming.Token, ticket.Offer.ArchiveBytes)
	deadline := time.Now().Add(3 * time.Second)
	for {
		status, _ := service.Status(incoming.ID)
		if status.State == "transferring" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("upload never started")
		}
		time.Sleep(time.Millisecond)
	}
	if !service.Cancel(incoming.ID) {
		t.Fatal("cancel failed")
	}
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	response, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatal("cancel left upload blocked", err)
	}
	response.Body.Close()
	if response.StatusCode != 400 {
		t.Fatal(response.StatusCode)
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatal("cancel published destination")
	}
	// A cancelled upload must release the single disk-writing slot.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := service.Offer(ctx, []string{source}, nil); err != nil {
		t.Fatal("cancel retained disk slot", err)
	}
}

func TestFileTransferServiceExpiryAndRetryLimit(t *testing.T) {
	service := newFileTransferService(t.TempDir(), transferTestLimits())
	defer service.Close()
	source := filepath.Join(t.TempDir(), "source")
	os.WriteFile(source, []byte("contents"), 0600)
	ticket, err := service.Offer(context.Background(), []string{source}, nil)
	if err != nil {
		t.Fatal(err)
	}
	incoming, err := service.AcceptReceive(ticket.Offer, filepath.Join(t.TempDir(), "Received"))
	if err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 4; attempt++ {
		req := httptest.NewRequest("POST", "/upload/"+incoming.Token, bytes.NewReader(make([]byte, ticket.Offer.ArchiveBytes)))
		result := httptest.NewRecorder()
		service.ServeHTTP(result, req)
		want := 400
		if attempt == 3 {
			want = 409
		}
		if result.Code != want {
			t.Fatalf("attempt %d: %d", attempt, result.Code)
		}
	}
	service.mu.Lock()
	service.jobs[ticket.Token].expires = time.Now().Add(-time.Second)
	archive := service.jobs[ticket.Token].archive
	service.mu.Unlock()
	result := httptest.NewRecorder()
	service.ServeHTTP(result, httptest.NewRequest("GET", "/download/"+ticket.Token, nil))
	if result.Code != 404 {
		t.Fatal("expired token accepted")
	}
	if _, err := service.AcceptReceive(ticket.Offer, filepath.Join(t.TempDir(), "Another")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(archive); !os.IsNotExist(err) {
		t.Fatal("expired archive retained")
	}
}

func TestFileTransferServiceCloseStopsListener(t *testing.T) {
	service := newFileTransferService(t.TempDir(), transferTestLimits())
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	finished := make(chan error, 1)
	go func() { finished <- service.Serve(listener) }()
	service.Close()
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("listener survived service close")
	}
}
