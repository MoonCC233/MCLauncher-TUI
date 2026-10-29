//go:build !windows

package loader

import "os/exec"

func hideWindow(cmd *exec.Cmd) {}
