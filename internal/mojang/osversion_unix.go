//go:build !windows

package mojang

import (
	"os/exec"
	"strings"
	"sync"
)

var (
	osVersionOnce sync.Once
	osVersionVal  string
)

func osVersion() string {
	osVersionOnce.Do(func() {
		out, err := exec.Command("uname", "-r").Output()
		if err == nil {
			osVersionVal = strings.TrimSpace(string(out))
		}
	})
	return osVersionVal
}
