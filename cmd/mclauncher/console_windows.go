//go:build windows

package main

import "golang.org/x/sys/windows"

// configureConsole switches the Windows console to UTF-8 so translated labels,
// box drawing and CJK text render correctly in cmd.exe and Windows Terminal.
func configureConsole() {
	_ = windows.SetConsoleOutputCP(65001)
}
