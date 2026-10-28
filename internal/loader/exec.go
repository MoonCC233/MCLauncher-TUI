package loader

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

func runJavaInstaller(ctx context.Context, javaPath, installerPath, dataRoot string, onLog func(string)) error {
	cmd := exec.CommandContext(ctx, javaPath, "-jar", installerPath, "--installClient", dataRoot)
	cmd.Dir = dataRoot
	hideWindow(cmd)
	writer := &lineWriter{onLine: onLog}
	cmd.Stdout = writer
	cmd.Stderr = writer
	err := cmd.Run()
	writer.Flush()
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("NeoForge installer failed: %w", err)
	}
	return nil
}

type lineWriter struct {
	mu      sync.Mutex
	pending []byte
	onLine  func(string)
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.pending = append(w.pending, p...)
	for {
		index := bytes.IndexByte(w.pending, '\n')
		if index < 0 {
			break
		}
		w.emit(string(w.pending[:index]))
		w.pending = w.pending[index+1:]
	}
	if len(w.pending) > 64*1024 {
		w.emit(string(w.pending))
		w.pending = nil
	}
	return len(p), nil
}

func (w *lineWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.pending) > 0 {
		w.emit(string(w.pending))
		w.pending = nil
	}
}

func (w *lineWriter) emit(line string) {
	line = strings.TrimSpace(line)
	if line != "" && w.onLine != nil {
		w.onLine(line)
	}
}
