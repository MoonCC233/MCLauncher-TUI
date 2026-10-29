//go:build windows

package gametitle

import (
	"os"
	"os/exec"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// TestSetRenamesARealWindow is a manual check: it starts notepad, renames its
// window and reads the title back. Run it with GAMETITLE_MANUAL=1.
func TestSetRenamesARealWindow(t *testing.T) {
	if os.Getenv("GAMETITLE_MANUAL") == "" {
		t.Skip("set GAMETITLE_MANUAL=1 to run this check")
	}
	// A message box is a real titled window owned by the child process, which is
	// what the game window looks like to the Win32 calls.
	cmd := exec.Command("powershell.exe", "-NoProfile", "-Command",
		"Add-Type -AssemblyName System.Windows.Forms; [System.Windows.Forms.MessageBox]::Show('wait','original title') | Out-Null")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start notepad: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()

	pid := cmd.Process.Pid
	if err := Watch(pid, "MCLauncher window test", Set, 40, 250*time.Millisecond); err != nil {
		t.Fatalf("Watch: %v", err)
	}
	title, err := windowTitleOf(pid)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if title != "MCLauncher window test" {
		t.Fatalf("window title = %q", title)
	}
}

// windowTitleOf reads the title of the first titled window owned by pid.
func windowTitleOf(pid int) (string, error) {
	var out string
	callback := windows.NewCallback(func(hwnd windows.HWND, _ uintptr) uintptr {
		var owner uint32
		if _, err := windows.GetWindowThreadProcessId(hwnd, &owner); err != nil || int(owner) != pid {
			return 1
		}
		if !windows.IsWindowVisible(hwnd) {
			return 1
		}
		length, _, _ := procGetWindowTextLength.Call(uintptr(hwnd))
		if length == 0 {
			return 1
		}
		out = windowText(hwnd)
		if out == "" {
			return 1
		}
		return 0
	})
	// EnumWindows reports an error when the callback stops the walk, so only trust
	// it when nothing was found.
	if err := windows.EnumWindows(callback, nil); err != nil && out == "" {
		return "", err
	}
	if out == "" {
		return "", ErrNotFound
	}
	return out, nil
}
