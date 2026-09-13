package main

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

const usbSample = "  Bus 1, Addr 3, Port 2.4, Speed 480 Mb/s\n    Class 00: USB device 1234:5678, Sample device\n"

type usbFake struct {
	objects    []usbQOMEntry
	calls      []string
	added      map[string]any
	failAttach bool
	oldRuntime bool
	inventory  string
}

func (f *usbFake) Call(_ context.Context, command string, args any, result any) error {
	f.calls = append(f.calls, command)
	arguments, _ := args.(map[string]any)
	var value any
	switch command {
	case "human-monitor-command":
		value = f.inventory
	case "qom-list":
		value = f.objects
	case "device-list-properties":
		value = []usbQOMEntry{}
		if !f.oldRuntime {
			value = []usbQOMEntry{{Name: "auto-reconnect"}}
		}
	case "device_add":
		if arguments["driver"] == "usb-host" {
			f.added = arguments
			if f.failAttach {
				return errors.New("device busy")
			}
		}
		f.objects = append(f.objects, usbQOMEntry{Name: arguments["id"].(string), Type: "child<" + arguments["driver"].(string) + ">"})
	case "device_del":
		remaining := []usbQOMEntry{}
		for _, o := range f.objects {
			if o.Name != arguments["id"] {
				remaining = append(remaining, o)
			}
		}
		f.objects = remaining
	default:
		return errors.New("unexpected command: " + command)
	}
	if result != nil {
		data, err := json.Marshal(value)
		if err != nil {
			return err
		}
		return json.Unmarshal(data, result)
	}
	return nil
}
func TestUSBExplicitAttachAndRelease(t *testing.T) {
	devices, err := parseUSBHostDevices(usbSample)
	if err != nil || len(devices) != 1 {
		t.Fatal(devices, err)
	}
	selected := devices[0]
	fake := &usbFake{objects: []usbQOMEntry{{Name: usbControllerID, Type: "child<qemu-xhci>"}}, inventory: usbSample}
	broker := usbBroker{fake}
	if err := broker.Attach(context.Background(), selected); err != nil {
		t.Fatal(err)
	}
	if fake.added["auto-reconnect"] != false || fake.added["hostport"] != "2.4" || fake.added["vendorid"] != 0x1234 || fake.added["hostaddr"] != 3 {
		t.Fatal("attachment did not bind exact selection", fake.added)
	}
	if err := broker.Detach(context.Background(), selected.ID); err != nil {
		t.Fatal(err)
	}
	if len(fake.objects) != 1 || fake.objects[0].Name != usbControllerID {
		t.Fatal("removed wrong object", fake.objects)
	}
	if err := broker.Detach(context.Background(), "someone-elses-device"); err == nil {
		t.Fatal("accepted unowned device")
	}
}
func TestUSBRejectsChangedDeviceAndMissingRuntimeSupport(t *testing.T) {
	devices, _ := parseUSBHostDevices(usbSample)
	for _, mode := range []string{"missing", "old-runtime", "busy"} {
		t.Run(mode, func(t *testing.T) {
			fake := &usbFake{objects: []usbQOMEntry{{Name: usbControllerID, Type: "child<qemu-xhci>"}}, inventory: usbSample, oldRuntime: mode == "old-runtime", failAttach: mode == "busy"}
			if mode == "missing" {
				fake.inventory = ""
			}
			if err := (usbBroker{fake}).Attach(context.Background(), devices[0]); err == nil {
				t.Fatal("reported attachment success")
			}
			for _, o := range fake.objects {
				if o.Type == "child<usb-host>" {
					t.Fatal("left failed attachment")
				}
			}
			if mode != "busy" && fake.added != nil {
				t.Fatal("attempted attachment without verified support")
			}
		})
	}
}
func TestUSBInventoryAndStablePortIdentity(t *testing.T) {
	first, _ := parseUSBHostDevices(usbSample)
	second, _ := parseUSBHostDevices("  Bus 1, Addr 8, Port 2.4, Speed 480 Mb/s\r\n    Class 00: USB device 1234:5678\r\n")
	if len(second) != 1 || first[0].ID != second[0].ID {
		t.Fatal("identity changed with enumeration address")
	}
	if _, err := parseUSBHostDevices("unknown command: info usbhost"); err == nil {
		t.Fatal("silently accepted unsupported runtime")
	}
	if _, err := parseUSBHostDevices("  Bus 999, Addr 3, Port 2, Speed 480 Mb/s\n    Class 00: USB device 1234:5678\n"); err == nil {
		t.Fatal("accepted malformed USB identity")
	}
}

func TestUSBZeroIdentifiersRemainExact(t *testing.T) {
	devices, err := parseUSBHostDevices("  Bus 1, Addr 3, Port 2, Speed 12 Mb/s\n    Class 00: USB device 0000:0000\n")
	if err != nil || len(devices) != 1 {
		t.Fatal(devices, err)
	}
	fake := &usbFake{objects: []usbQOMEntry{{Name: usbControllerID, Type: "child<qemu-xhci>"}}, inventory: "  Bus 1, Addr 3, Port 2, Speed 12 Mb/s\n    Class 00: USB device 0000:0000\n"}
	if err := (usbBroker{fake}).Attach(context.Background(), devices[0]); err != nil {
		t.Fatal(err)
	}
	if fake.added["vendorid"] != 0 || fake.added["productid"] != 0 || fake.added["auto-reconnect"] != false {
		t.Fatal("zero IDs became wildcard selection")
	}
}

func TestUSBRequiresControllerCreatedAtLaunch(t *testing.T) {
	devices, err := parseUSBHostDevices(usbSample)
	if err != nil {
		t.Fatal(err)
	}
	fake := &usbFake{inventory: usbSample}
	if err := (usbBroker{fake}).Attach(context.Background(), devices[0]); err == nil {
		t.Fatal("attempted unsupported controller hotplug")
	}
	if fake.added != nil {
		t.Fatal("added a device without a startup controller")
	}
}
