package main

import (
	"os"
	"strings"
	"testing"
)

func TestUSBRealRuntimeControllerAndMissingDevice(t *testing.T) {
	c, ctx, _ := startSavedSessionTestQEMU(t, "-device", "qemu-xhci,id="+usbControllerID)
	var properties []struct {
		Name string `json:"name"`
	}
	err := c.Call(ctx, "device-list-properties", map[string]any{"typename": "usb-host"}, &properties)
	found := false
	for _, property := range properties {
		if property.Name == "auto-reconnect" {
			found = true
		}
	}
	if err != nil || !found {
		if os.Getenv("QEMU_SYSTEM") == "" {
			t.Skip("requires the explicit-attachment runtime")
		}
		t.Fatalf("missing explicit USB attachment support: %v", err)
	}
	broker := usbBroker{c}
	objects, err := broker.objects(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found = false
	for _, object := range objects {
		if object.Name == usbControllerID && object.Type == "child<qemu-xhci>" {
			found = true
		}
	}
	if !found {
		t.Fatal("startup controller missing")
	}
	if _, err := broker.Devices(ctx); err != nil {
		t.Fatal(err)
	}
	// libusb bus numbers are uint8_t, so this cannot match host hardware.
	err = c.Call(ctx, "device_add", map[string]any{"driver": "usb-host", "id": "missing-usb-test", "bus": usbControllerID + ".0", "hostbus": 65535, "hostaddr": 127, "hostport": "127", "vendorid": 65535, "productid": 0, "auto-reconnect": false}, nil)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "failed to find host usb device") {
		t.Fatalf("unexpected missing-device result: %v", err)
	}
	objects, err = broker.objects(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, object := range objects {
		if object.Name == "missing-usb-test" {
			t.Fatal("failed attachment left a device")
		}
	}
}
