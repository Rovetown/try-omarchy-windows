package main

import "testing"

func TestCheckpointBootPreservesVersionsUntilGuestReady(t *testing.T) {
	dir, _ := portableRecoveryFixture(t)
	if err := markCheckpointBoot(dir); err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		guest, pin, runtime, runtimePin := "new guest", "new pin", "new runtime", "new runtime pin"
		pending, err := pinCheckpointBoot(dir, nil, &guest, &pin, &runtime, &runtimePin)
		if err != nil || !pending || guest != "https://example.invalid/original-guest" || runtime != "https://example.invalid/original-runtime" {
			t.Fatal("snapshot versions replaced before successful boot", pending, guest, runtime, err)
		}
	}
	guest, pin, runtime, runtimePin := "explicit guest", "explicit pin", "explicit runtime", "explicit runtime pin"
	pending, err := pinCheckpointBoot(dir, map[string]bool{"release": true}, &guest, &pin, &runtime, &runtimePin)
	if err != nil || pending || guest != "explicit guest" {
		t.Fatal("ignored explicit override", pending, guest, err)
	}
	commitCheckpointBoot(dir)
	pending, err = pinCheckpointBoot(dir, nil, &guest, &pin, &runtime, &runtimePin)
	if err != nil || pending || guest != "explicit guest" {
		t.Fatal("successful boot retained pin", pending, guest, err)
	}
}
