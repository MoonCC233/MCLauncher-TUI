package launch

import (
	"os/exec"
	"runtime"
	"sync"
	"testing"
	"time"
)

func startTestSession(t *testing.T) *Session {
	t.Helper()
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "ping -n 60 127.0.0.1 >nul")
	} else {
		cmd = exec.Command("sh", "-c", "sleep 60")
	}
	hideWindow(cmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	s := &Session{
		cmd:     cmd,
		lines:   make(chan Line, 16),
		done:    make(chan struct{}),
		started: time.Now(),
	}
	go func() {
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			pump(stdout, "stdout", s.lines)
		}()
		go func() {
			defer wg.Done()
			pump(stderr, "stderr", s.lines)
		}()
		wg.Wait()
		s.waitErr = cmd.Wait()
		close(s.lines)
		close(s.done)
	}()
	t.Cleanup(func() { _ = s.Kill() })
	return s
}

func waitForSession(t *testing.T, s *Session, timeout time.Duration) {
	t.Helper()
	select {
	case <-s.Done():
	case <-time.After(timeout):
		t.Fatal("game session did not terminate")
	}
}

func TestSessionStopTerminatesProcess(t *testing.T) {
	s := startTestSession(t)
	if err := s.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	waitForSession(t, s, 8*time.Second)
}

func TestSessionKillTerminatesProcess(t *testing.T) {
	s := startTestSession(t)
	if err := s.Kill(); err != nil {
		t.Fatalf("Kill: %v", err)
	}
	waitForSession(t, s, 8*time.Second)
}
