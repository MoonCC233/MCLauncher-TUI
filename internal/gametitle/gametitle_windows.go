//go:build windows

package gametitle

import (
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32                  = windows.NewLazySystemDLL("user32.dll")
	procSetWindowText       = user32.NewProc("SetWindowTextW")
	procGetWindowText       = user32.NewProc("GetWindowTextW")
	procGetWindowTextLength = user32.NewProc("GetWindowTextLengthW")
)

// windowText reads the title of one window.
func windowText(hwnd windows.HWND) string {
	length, _, _ := procGetWindowTextLength.Call(uintptr(hwnd))
	if length == 0 {
		return ""
	}
	buf := make([]uint16, length+1)
	if ok, _, _ := procGetWindowText.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf))); ok == 0 {
		return ""
	}
	return windows.UTF16ToString(buf)
}

// Set renames the Minecraft window of pid. The game window is the visible titled
// window whose title mentions Minecraft; when the process owns a single titled
// window it is used even if a mod customised the title.
func Set(pid int, title string) error {
	if pid <= 0 {
		return ErrNotFound
	}
	titlePtr, err := windows.UTF16PtrFromString(title)
	if err != nil {
		return err
	}

	var (
		game     windows.HWND
		single   windows.HWND
		titled   int
		finished bool
	)
	callback := windows.NewCallback(func(hwnd windows.HWND, _ uintptr) uintptr {
		var owner uint32
		if _, err := windows.GetWindowThreadProcessId(hwnd, &owner); err != nil {
			return 1
		}
		if int(owner) != pid || !windows.IsWindowVisible(hwnd) {
			return 1
		}
		text := windowText(hwnd)
		if text == "" {
			return 1
		}
		titled++
		single = hwnd
		if game == 0 && strings.Contains(strings.ToLower(text), "minecraft") {
			game = hwnd
		}
		// Stop as soon as the obvious candidate is known.
		if game != 0 {
			finished = true
			return 0
		}
		return 1
	})

	if err := windows.EnumWindows(callback, nil); err != nil && !finished {
		return err
	}

	target := game
	if target == 0 && titled == 1 {
		target = single
	}
	if target == 0 {
		return ErrNotFound
	}
	// Keep polls once a second, so skip the call when the title is already ours.
	if windowText(target) == title {
		return nil
	}
	if ok, _, _ := procSetWindowText.Call(uintptr(target), uintptr(unsafe.Pointer(titlePtr))); ok == 0 {
		return ErrNotFound
	}
	return nil
}
