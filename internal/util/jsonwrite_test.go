package util

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A save must survive the target being briefly locked, and must never leave the
// temporary file behind.
func TestWriteJSONAtomicOverwritesAndCleansUp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"old":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteJSONAtomic(path, map[string]string{"hello": "world"}); err != nil {
		t.Fatalf("WriteJSONAtomic: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) == `{"old":true}` {
		t.Error("the file was not replaced")
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("temporary file left behind: %v", err)
	}
}

func TestWriteJSONAtomicReportsThePath(t *testing.T) {
	// A directory in place of the file cannot be replaced, so the write fails; the
	// error has to name the target so the toast is actionable.
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	err := WriteJSONAtomic(path, map[string]string{"a": "b"})
	if err == nil {
		t.Fatal("writing over a directory should fail")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error %q does not name the path", err.Error())
	}
	if _, statErr := os.Stat(path + ".tmp"); !os.IsNotExist(statErr) {
		t.Errorf("temporary file left behind: %v", statErr)
	}
}
