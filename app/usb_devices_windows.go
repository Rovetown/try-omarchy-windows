//go:build windows

package main

import (
	"context"
	"fmt"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

type usbUIResult struct {
	devices []usbDevice
	err     error
}
type usbUIState struct {
	window, list, status uintptr
	buttons              []uintptr
	devices              []usbDevice
	busy, closing, done  bool
	cancel               context.CancelFunc
	results              chan usbUIResult
}

var usbUI *usbUIState
var usbUIRegistered bool
var usbUICallback = syscall.NewCallback(usbWindowProc)

const usbResultMessage = 0x8031

func usbSetText(handle uintptr, text string) {
	p, _ := syscall.UTF16PtrFromString(text)
	procSetWindowTextW.Call(handle, uintptr(unsafe.Pointer(p)))
}
func (s *usbUIState) start(action string) {
	if s.busy {
		return
	}
	var selected usbDevice
	if action != "refresh" {
		index, _, _ := procSendMessageW.Call(s.list, 0x188, 0, 0)
		if index >= uintptr(len(s.devices)) {
			usbSetText(s.status, "Select a USB device first.")
			return
		}
		selected = s.devices[index]
		if action == "attach" && selected.Claimed {
			usbSetText(s.status, "This device is already attached.")
			return
		}
		if action == "detach" && !selected.Claimed {
			usbSetText(s.status, "This device is already available to Windows.")
			return
		}
		if action == "attach" && msgBox("Attach "+selected.Name+" to Omarchy?\n\nWindows applications will lose access until you release it. Eject mounted storage before switching it.", mbYesNo|mbIconQuestion|mbDefbutton2) != idYes {
			return
		}
	}
	s.busy = true
	for _, button := range s.buttons {
		procEnableWindow.Call(button, 0)
	}
	usbSetText(s.status, "Reading USB devices...")
	if action == "attach" {
		usbSetText(s.status, "Attaching device...")
	} else if action == "detach" {
		usbSetText(s.status, "Releasing device to Windows...")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	s.cancel = cancel
	go func() {
		defer cancel()
		var result usbUIResult
		client, err := dialQMPClient(ctx, fmt.Sprintf("127.0.0.1:%d", qmpToolsPort))
		if err == nil {
			defer client.Close()
			broker := usbBroker{qmp: client}
			switch action {
			case "attach":
				err = broker.Attach(ctx, selected)
			case "detach":
				err = broker.Detach(ctx, selected.ID)
			}
			if err == nil {
				result.devices, err = broker.Devices(ctx)
			}
		}
		result.err = err
		s.results <- result
		procPostMessageW.Call(s.window, usbResultMessage, 0, 0)
	}()
}
func usbWindowProc(hwnd, message, w, l uintptr) uintptr {
	s := usbUI
	if s == nil {
		r, _, _ := procDefWindowProcW.Call(hwnd, message, w, l)
		return r
	}
	switch message {
	case usbResultMessage:
		result := <-s.results
		s.busy = false
		if s.closing {
			procDestroyWindow.Call(hwnd)
			return 0
		}
		for _, button := range s.buttons {
			procEnableWindow.Call(button, 1)
		}
		if result.err != nil {
			usbSetText(s.status, result.err.Error())
			return 0
		}
		s.devices = result.devices
		procSendMessageW.Call(s.list, 0x184, 0, 0)
		for _, device := range s.devices {
			state := "Available to Windows"
			if device.Claimed {
				state = "Attached to Omarchy"
				if !device.Connected {
					state = "Unplugged; release to clear"
				}
			}
			label := fmt.Sprintf("%s   [%s]   USB %d/%s", device.Name, state, device.Bus, device.Port)
			p, _ := syscall.UTF16PtrFromString(label)
			procSendMessageW.Call(s.list, 0x180, 0, uintptr(unsafe.Pointer(p)))
		}
		if len(s.devices) > 0 {
			procSendMessageW.Call(s.list, 0x186, 0, 0)
		}
		usbSetText(s.status, fmt.Sprintf("%d devices. Refresh after connecting or unplugging a device.", len(s.devices)))
		return 0
	case wmCommand:
		switch w & 0xffff {
		case 4301:
			s.start("refresh")
		case 4302:
			s.start("attach")
		case 4303:
			s.start("detach")
		case 2:
			procPostMessageW.Call(hwnd, wmClose, 0, 0)
		}
		return 0
	case wmClose:
		if s.busy {
			s.closing = true
			s.cancel()
			usbSetText(s.status, "Finishing device operation...")
		} else {
			procDestroyWindow.Call(hwnd)
		}
		return 0
	case wmDestroy:
		s.done = true
		procPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, message, w, l)
	return r
}
func runUSBDeviceUI() error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	s := &usbUIState{results: make(chan usbUIResult, 1)}
	usbUI = s
	defer func() { usbUI = nil }()
	instance, _, _ := procGetModuleHandleW.Call(0)
	class, _ := syscall.UTF16PtrFromString("TryOmarchyUSBDevices")
	if !usbUIRegistered {
		type windowClass struct {
			size, style                   uint32
			callback                      uintptr
			classExtra, windowExtra       int32
			instance, icon, cursor, brush uintptr
			menu, class                   *uint16
			smallIcon                     uintptr
		}
		cursor, _, _ := procLoadCursorW.Call(0, idcArrow)
		wc := windowClass{size: uint32(unsafe.Sizeof(windowClass{})), callback: usbUICallback, instance: instance, cursor: cursor, brush: colorBtnface + 1, class: class}
		if result, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); result == 0 {
			return err
		}
		usbUIRegistered = true
	}
	title, _ := syscall.UTF16PtrFromString("USB devices")
	style := uintptr(wsCaption | wsSysmenu)
	rect := [4]int32{0, 0, 660, 350}
	procAdjustWindowRectEx.Call(uintptr(unsafe.Pointer(&rect[0])), style, 0, 0)
	var err error
	s.window, _, err = procCreateWindowExW.Call(0, uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(title)), style|wsVisible, 100, 100, uintptr(rect[2]-rect[0]), uintptr(rect[3]-rect[1]), 0, 0, instance, 0)
	if s.window == 0 {
		return err
	}
	font, _, _ := procGetStockObject.Call(defaultGuiFont)
	var controlErr error
	control := func(class, label string, x, y, width, height int, style, id uintptr) uintptr {
		c, _ := syscall.UTF16PtrFromString(class)
		p, _ := syscall.UTF16PtrFromString(label)
		h, _, err := procCreateWindowExW.Call(0, uintptr(unsafe.Pointer(c)), uintptr(unsafe.Pointer(p)), wsVisible|wsChild|style, uintptr(x), uintptr(y), uintptr(width), uintptr(height), s.window, id, instance, 0)
		if h == 0 {
			controlErr = err
		}
		procSendMessageW.Call(h, wmSetfont, font, 1)
		return h
	}
	control("STATIC", "Attach a device to Omarchy, then release it when you want to use it in Windows.", 16, 16, 628, 24, ssNoprefix, 0)
	s.list = control("LISTBOX", "", 16, 48, 628, 182, wsTabstop|wsBorder|wsVscroll|1, 4300)
	s.status = control("STATIC", "", 16, 240, 628, 54, ssNoprefix, 0)
	for _, button := range []struct {
		text string
		x    int
		id   uintptr
	}{{"Refresh", 16, 4301}, {"Attach", 128, 4302}, {"Release", 240, 4303}} {
		s.buttons = append(s.buttons, control("BUTTON", button.text, button.x, 306, 100, 28, wsTabstop, button.id))
	}
	control("BUTTON", "Close", 544, 306, 100, 28, wsTabstop, 2)
	if controlErr != nil {
		procDestroyWindow.Call(s.window)
		return controlErr
	}
	procSetFocus.Call(s.list)
	s.start("refresh")
	var message msgStruct
	for !s.done {
		result, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if result == 0 || int32(result) == -1 {
			break
		}
		if handled, _, _ := procIsDialogMessageW.Call(s.window, uintptr(unsafe.Pointer(&message))); handled != 0 {
			continue
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&message)))
	}
	return nil
}
