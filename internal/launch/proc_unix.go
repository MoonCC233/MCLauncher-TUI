//go:build !windows

package launch

import (
	"os/exec"
	"syscall"
	"time"
)

func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func terminate(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	pid := cmd.Process.Pid
	if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil {
		_ = cmd.Process.Kill()
	}
	go func() {
		time.Sleep(5 * time.Second)
		_ = syscall.Kill(-pid, syscall.SIGKILL)
	}()
	return nil
}
