//go:build windows

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Run in its own interactive test process alongside smoke-guest.py's transfer
// round trip. It exercises the production bridge through QEMU user networking.
func TestGuestDesktopFileTransferRoundTrip(t *testing.T) {
	if os.Getenv("TRYOMARCHY_GUEST_TRANSFER_TEST") != "1" {
		t.Skip("complete guest desktop and isolated Windows test process required")
	}
	t.Setenv("LOCALAPPDATA", t.TempDir())
	if !clipboardSetItem(clipItem{Kind: clipText, Data: []byte("clipboard stays here")}) {
		t.Fatal("cannot initialize test clipboard")
	}
	sequence := clipboardSequence()
	payload := bytes.Repeat([]byte("Try Omarchy verified file drop\n"), 1<<20)
	source := filepath.Join(t.TempDir(), "from-windows-世界.txt")
	if err := os.WriteFile(source, payload, 0600); err != nil {
		t.Fatal(err)
	}
	runClipboardBridge()
	bridge := desktopClipboard.Load()
	t.Cleanup(bridge.transfers.Close)
	received := make(chan []string, 1)
	bridge.setDropPaths = func(paths []string) bool { go showFileDropWindow(paths); received <- paths; return true }
	deadline := time.Now().Add(7 * time.Minute)
	for {
		bridge.mu.Lock()
		connected := bridge.pullConn != nil && bridge.transferEnabled
		bridge.mu.Unlock()
		if connected {
			t.Log("guest negotiated streaming")
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("guest did not connect to the production transfer bridge")
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err := bridge.offerDroppedFiles([]string{source}); err != nil {
		t.Fatal(err)
	}
	t.Log("host sent file-drop ticket")
	select {
	case paths := <-received:
		if len(paths) != 1 || filepath.Base(paths[0]) != "from-omarchy-世界.txt" {
			t.Fatal("unexpected guest files", paths)
		}
		data, err := os.ReadFile(paths[0])
		if err != nil || !bytes.Equal(data, payload) {
			t.Fatal("guest return data changed", err)
		}
		// Wait for the control handler to write its completion acknowledgment.
		bridge.mu.Lock()
		bridge.mu.Unlock()
		if clipboardSequence() != sequence {
			t.Fatal("file drops replaced the Windows clipboard")
		}
		original, err := os.ReadFile(source)
		if err != nil || !bytes.Equal(original, payload) {
			t.Fatal("original Windows file changed", err)
		}
	case <-time.After(time.Until(deadline)):
		t.Fatal("guest did not return the dropped file")
	}
}
