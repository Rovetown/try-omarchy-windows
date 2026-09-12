//go:build windows

package main

import "unsafe"

var (
	procSetScrollInfo = user32.NewProc("SetScrollInfo")
	procGetScrollInfo = user32.NewProc("GetScrollInfo")
	procGetFocus      = user32.NewProc("GetFocus")
)

type settingsScrollControl struct {
	handle     uintptr
	x, y, w, h int32
}
type settingsScroll struct {
	window                  uintptr
	height, content, offset int32
	controls                []settingsScrollControl
}
type settingsScrollInfo struct {
	size, mask uint32
	min, max   int32
	page       uint32
	pos, track int32
}

func (s *settingsScroll) move(offset int32) {
	maximum := s.content - s.height
	if maximum < 0 {
		maximum = 0
	}
	if offset < 0 {
		offset = 0
	}
	if offset > maximum {
		offset = maximum
	}
	s.offset = offset
	for _, c := range s.controls {
		procSetWindowPos.Call(c.handle, 0, uintptr(c.x), uintptr(c.y-offset), uintptr(c.w), uintptr(c.h), 0x0004|0x0010)
	} // no z-order change or activation
	info := settingsScrollInfo{mask: 0x7, max: s.content - 1, page: uint32(s.height), pos: offset}
	info.size = uint32(unsafe.Sizeof(info))
	procSetScrollInfo.Call(s.window, 1, uintptr(unsafe.Pointer(&info)), 1)
}

func (s *settingsScroll) handle(message, wParam uintptr) bool {
	switch message {
	case 0x0115: // WM_VSCROLL
		next := s.offset
		switch wParam & 0xffff {
		case 0:
			next -= 24
		case 1:
			next += 24
		case 2:
			next -= s.height - 24
		case 3:
			next += s.height - 24
		case 4, 5:
			info := settingsScrollInfo{mask: 0x10}
			info.size = uint32(unsafe.Sizeof(info))
			procGetScrollInfo.Call(s.window, 1, uintptr(unsafe.Pointer(&info)))
			next = info.track
		case 6:
			next = 0
		case 7:
			next = s.content
		}
		s.move(next)
		return true
	case 0x020A: // WM_MOUSEWHEEL
		s.move(s.offset - int32(int16(wParam>>16))/120*72)
		return true
	}
	return false
}

func (s *settingsScroll) revealFocus() {
	focus, _, _ := procGetFocus.Call()
	for _, c := range s.controls {
		if c.handle != focus {
			continue
		}
		if c.y < s.offset {
			s.move(c.y)
		} else if c.y+c.h > s.offset+s.height {
			s.move(c.y + c.h - s.height)
		}
		return
	}
}
