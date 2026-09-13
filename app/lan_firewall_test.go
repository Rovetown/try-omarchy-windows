package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLANFirewallPlanOwnershipAndScope(t *testing.T) {
	dir := t.TempDir()
	program := filepath.Join(dir, "qemu.exe")
	local, _ := parseForward("tcp:8080:80")
	lan, _ := parseForward("tcp:192.168.1.5:9000:80")
	empty, err := makeLANFirewallPlan(dir, program, false, []portForward{local})
	if err != nil || empty.Group != "" {
		t.Fatal("created rules for a loopback-only install")
	}
	plan, err := makeLANFirewallPlan(dir, program, false, []portForward{local, lan})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Rules) != 1 || plan.Rules[0].Port != 9000 || plan.Public {
		t.Fatal("incorrect LAN exposure")
	}
	again, err := makeLANFirewallPlan(dir, program, false, []portForward{lan})
	if err != nil || again.Group != plan.Group || again.Generation != plan.Generation {
		t.Fatal("unstable rule ownership")
	}
	removed, err := makeLANFirewallPlan(dir, program, false, nil)
	if err != nil || removed.Group != plan.Group || len(removed.Rules) != 0 {
		t.Fatal("lost ownership when removing forwards")
	}
	if err := os.WriteFile(filepath.Join(dir, networkIdentityFilename), []byte("other-app"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := makeLANFirewallPlan(dir, program, false, nil); err == nil {
		t.Fatal("accepted malformed ownership")
	}
}
