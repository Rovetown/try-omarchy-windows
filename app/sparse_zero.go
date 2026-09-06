package main

import "bytes"

// Compare zero-filled regions without allocating a fresh comparison buffer.
var zeroScanBlock [64 << 10]byte

func zeroBytes(b []byte) bool {
	if len(b) > 0 && b[0] != 0 {
		return false
	}
	for len(b) > 0 {
		n := min(len(b), len(zeroScanBlock))
		if !bytes.Equal(b[:n], zeroScanBlock[:n]) {
			return false
		}
		b = b[n:]
	}
	return true
}
