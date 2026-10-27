//go:build !windows

package java

import "os/exec"

func hideWindow(cmd *exec.Cmd) {}
