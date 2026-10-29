package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MoonCC233/MCLauncher-TUI/internal/config"
	"github.com/MoonCC233/MCLauncher-TUI/internal/paths"
)

func TestSettingsAreWrittenToDisk(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "config.json")
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	m := New(cfg, paths.New(root), nil, "test", "")

	m.toggleKeepGameRunning()
	m.setTheme("forest", "")

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config was not written: %v", err)
	}
	reloaded, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reloaded.Settings.KeepGameRunning {
		t.Error("keep_game_running was not persisted")
	}
	if reloaded.Settings.Theme != "forest" {
		t.Errorf("theme = %q, want forest", reloaded.Settings.Theme)
	}
}
