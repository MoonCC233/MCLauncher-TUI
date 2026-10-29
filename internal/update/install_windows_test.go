//go:build windows

package update

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestInstallBinaryAtRunningImage(t *testing.T) {
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = `C:\Windows`
	}
	ping := filepath.Join(root, "System32", "ping.exe")
	source, err := os.ReadFile(ping)
	if err != nil {
		t.Skipf("ping.exe is unavailable: %v", err)
	}
	dir := t.TempDir()
	current := filepath.Join(dir, "mclauncher.exe")
	if err := os.WriteFile(current, source, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(current, "-n", "30", "127.0.0.1")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		_ = os.Remove(current + ".old")
	}()
	time.Sleep(200 * time.Millisecond)

	newBinary := filepath.Join(dir, "new-binary")
	if err := os.WriteFile(newBinary, []byte("new-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := installBinaryAt(current, newBinary); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(current)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new-binary" {
		t.Errorf("current binary = %q", data)
	}
}
