package main

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestForwardBindingsRejectOccupiedTCPAndUDP(t *testing.T) {
	tcp, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer tcp.Close()
	udp, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer udp.Close()
	for _, forward := range []portForward{{proto: "tcp", hostPort: tcp.Addr().(*net.TCPAddr).Port, guestPort: 80}, {proto: "udp", hostPort: udp.LocalAddr().(*net.UDPAddr).Port, guestPort: 80}} {
		if err := checkForwardBindings([]portForward{forward}); err == nil {
			t.Fatal("accepted an occupied port")
		}
	}
}
func TestForwardBindingProbesReleaseSockets(t *testing.T) {
	probe, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := probe.Addr().String()
	port := probe.Addr().(*net.TCPAddr).Port
	probe.Close()
	if err := checkForwardBindings([]portForward{{proto: "tcp", hostPort: port, guestPort: 80}}); err != nil {
		t.Fatal(err)
	}
	probe, err = net.Listen("tcp4", address)
	if err != nil {
		t.Fatal("probe kept QEMU's port occupied", err)
	}
	probe.Close()
}
func TestNetworkStartupErrorDoesNotMasqueradeAsGPUFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "qemu-stderr.log")
	for _, test := range []struct {
		text  string
		match bool
	}{{"qemu: Could not set up host forwarding rule 'tcp:127.0.0.1:8080-:80'", true}, {"qemu: OpenGL initialization failed", false}} {
		if err := os.WriteFile(path, []byte(test.text), 0600); err != nil {
			t.Fatal(err)
		}
		if forwardStartupProblem(dir) != test.match {
			t.Fatal("incorrect startup diagnosis")
		}
	}
}
