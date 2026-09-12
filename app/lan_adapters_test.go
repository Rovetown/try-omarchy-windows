package main

import "testing"

func TestForwardAdapterFollowsDHCPWithoutChangingSavedAddress(t *testing.T) {
	saved, err := parseForward("tcp:192.168.1.5:8080:80")
	if err != nil {
		t.Fatal(err)
	}
	bindings := map[string]string{saved.String(): "adapter-A"}
	resolved, err := resolveForwardAdapters([]portForward{saved}, bindings, []lanAdapter{{Name: "Ethernet", Address: "192.168.1.20", Identity: "ADAPTER-a"}})
	if err != nil {
		t.Fatal(err)
	}
	if resolved[0].bind != "192.168.1.20" || saved.bind != "192.168.1.5" {
		t.Fatal("did not resolve DHCP address independently")
	}
	if _, err := resolveForwardAdapters([]portForward{saved}, bindings, nil); err == nil {
		t.Fatal("accepted missing adapter")
	}
}

func TestForwardAdapterPreservesSelectedAlias(t *testing.T) {
	saved, _ := parseForward("tcp:192.168.1.5:8080:80")
	resolved, err := resolveForwardAdapters([]portForward{saved}, map[string]string{saved.String(): "a"}, []lanAdapter{{Address: "192.168.1.2", Identity: "a"}, {Address: "192.168.1.5", Identity: "a"}, {Address: "192.168.1.8", Identity: "a"}})
	if err != nil || resolved[0].bind != saved.bind {
		t.Fatalf("lost selected alias: %v %v", resolved, err)
	}
}

func TestForwardAdapterRejectsResolvedCollision(t *testing.T) {
	first, _ := parseForward("tcp:192.168.1.5:8080:80")
	second, _ := parseForward("tcp:192.168.1.20:8080:81")
	if _, err := resolveForwardAdapters([]portForward{first, second}, map[string]string{first.String(): "a"}, []lanAdapter{{Address: "192.168.1.20", Identity: "a"}}); err == nil {
		t.Fatal("accepted conflicting resolved bindings")
	}
}
