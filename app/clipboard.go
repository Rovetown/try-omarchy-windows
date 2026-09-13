//go:build windows

package main

import (
	"fmt"
	"net"
	"os"
)

func runClipboardBridge() {
	cache, err := clipboardFilesCache()
	if err != nil {
		fatal("Could not prepare file transfers: %v", err)
	}
	if err := validateMovePath(cache); err != nil {
		fatal("Could not prepare file transfers: %v", err)
	}
	if err := os.MkdirAll(cache, 0700); err != nil {
		fatal("Could not prepare file transfers: %v", err)
	}
	transfers := newFileTransferService(cache, clipboardTransferLimits)
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", transferPort))
	if err != nil {
		fatal("Try Omarchy file-transfer port %d is in use.", transferPort)
	}
	go func() {
		if err := transfers.Serve(listener); err != nil {
			logf("file transfers: %v", err)
		}
	}()
	b := &clipBridge{transferError: func(err error) { infoBox("These files could not be copied to Omarchy.\n\n" + err.Error()) }, showTransfer: showTransferProgress, transfers: transfers, getPaths: clipboardGetFilePaths, setPaths: clipboardSetFilePaths, getHost: clipboardGetItem, setHost: clipboardSetItem, sequence: clipboardSequence}
	// These listeners double as the single-instance check: a second copy of
	// the app (or a leftover QEMU on our QMP ports) must fail loudly, not
	// die 30 seconds later with an inscrutable QEMU port error.
	push, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", clipPushPort))
	if err != nil {
		fatal("Try Omarchy looks like it's already running (port %d is in use).", clipPushPort)
	}
	pull, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", clipPullPort))
	if err != nil {
		fatal("Try Omarchy looks like it's already running (port %d is in use).", clipPullPort)
	}
	logf("clipboard: guest->host on %d, host->guest on %d", clipPushPort, clipPullPort)

	go b.acceptPush(push)
	go b.acceptPull(pull)
	go b.pollHost()
}
