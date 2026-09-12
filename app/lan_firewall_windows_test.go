//go:build windows

package main

import (
	"os"
	"testing"
)

func TestNativeLANFirewallLifecycle(t *testing.T) {
	if os.Getenv("TRYOMARCHY_FIREWALL_TEST") != "1" {
		t.Skip("explicit disposable Windows firewall test")
	}
	configureSetupCancellation(false)
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	forward, _ := parseForward("tcp:0.0.0.0:59188:8080")
	plan, err := makeLANFirewallPlan(t.TempDir(), program, false, []portForward{forward})
	if err != nil {
		t.Fatal(err)
	}
	sentinel, err := makeLANFirewallPlan(t.TempDir(), program, false, []portForward{forward})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		plan.Rules = nil
		sentinel.Rules = nil
		if err := executeLANFirewall(plan, true); err != nil {
			t.Error(err)
		}
		if err := executeLANFirewall(sentinel, true); err != nil {
			t.Error(err)
		}
	}()
	if err := executeLANFirewall(sentinel, true); err != nil {
		t.Fatal(err)
	}
	if err := executeLANFirewall(plan, true); err != nil {
		t.Fatal(err)
	}
	if err := executeLANFirewall(plan, false); err != nil {
		t.Fatal("new rules did not match:", err)
	}
	plan.Rules = nil
	if err := executeLANFirewall(plan, true); err != nil {
		t.Fatal(err)
	}
	if err := executeLANFirewall(plan, false); err != nil {
		t.Fatal("removed rules remain:", err)
	}
	if err := executeLANFirewall(sentinel, false); err != nil {
		t.Fatal("changed another installation's rules:", err)
	}
}

func TestNativeLANAdapterDiscovery(t *testing.T) {
	configureSetupCancellation(false)
	adapters, err := availableLANAdapters()
	if err != nil {
		t.Fatal(err)
	}
	if len(adapters) == 0 || adapters[len(adapters)-1].Address != "0.0.0.0" {
		t.Fatal("missing all-adapters choice")
	}
	for _, adapter := range adapters[:len(adapters)-1] {
		if adapter.Name == "" || adapter.Identity == "" {
			t.Fatalf("missing stable adapter identity: %+v", adapter)
		}
		if _, err := parseForward("tcp:" + adapter.Address + ":8080:80"); err != nil {
			t.Fatal(err)
		}
	}
}
