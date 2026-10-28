package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/MoonCC233/MCLauncher-TUI/internal/config"
	"github.com/MoonCC233/MCLauncher-TUI/internal/paths"
)

func TestThemeHelpers(t *testing.T) {
	if got := normalizeThemeID("bogus"); got != config.DefaultTheme {
		t.Errorf("normalizeThemeID(bogus) = %q", got)
	}
	if got := normalizeThemeID("forest"); got != "forest" {
		t.Errorf("normalizeThemeID(forest) = %q", got)
	}
	if got := themeName("midnight"); got != "Midnight" {
		t.Errorf("themeName(midnight) = %q", got)
	}
	if got := themePickerCountForTest(); got != len(themePresets)+2 {
		t.Errorf("theme picker count = %d", got)
	}
}

func TestApplyThemeCustomAccent(t *testing.T) {
	defer applyTheme(config.DefaultTheme, "")
	applyTheme("forest", "#112233")
	if got := string(colPrimary); got != "#112233" {
		t.Errorf("primary = %q", got)
	}
	if len(accentGradientColors) == 0 || string(accentGradientColors[0]) != "#112233" {
		t.Errorf("gradient = %+v", accentGradientColors)
	}
	applyTheme("midnight", "")
	if got := string(colPrimary); got != "#7aa2f7" {
		t.Errorf("midnight primary = %q", got)
	}
}

func testModel(t *testing.T) *Model {
	t.Helper()
	settings := config.DefaultSettings()
	settings.Language = "en"
	cfg := &config.Config{DataVersion: config.DataVersion, Settings: settings}
	return New(cfg, paths.New(t.TempDir()), nil, "test", "")
}

func TestBackgroundTaskAndHistory(t *testing.T) {
	m := testModel(t)
	m.task = &TaskState{
		Kind:      taskInstall,
		Title:     "Installing 1.21.4",
		VersionID: "1.21.4",
		Stage:     "Downloading game files",
		Started:   time.Now(),
	}
	m.backgroundTask(false)
	if !m.task.Backgrounded {
		t.Error("backgroundTask should mark the task backgrounded")
	}
	if m.modal != modalNone {
		t.Errorf("modal = %v, want none", m.modal)
	}

	for i := 0; i < 25; i++ {
		m.recordTask(&TaskState{
			Kind:      taskInstall,
			Title:     "Installing 1.21.4",
			VersionID: "1.21.4",
			Started:   time.Now(),
			Done:      int64(i),
		}, "completed", nil)
	}
	if len(m.taskHistory) != 20 {
		t.Errorf("history length = %d, want 20", len(m.taskHistory))
	}
	if m.taskHistory[0].Done != 24 {
		t.Errorf("history should be newest first, got done=%d", m.taskHistory[0].Done)
	}

	m.task = nil
	view := strings.Join(m.viewDownloads(90, 30), "\n")
	if !strings.Contains(view, "HISTORY") || !strings.Contains(view, "No active task.") {
		t.Errorf("downloads view missing expected sections:\n%s", view)
	}
}

func themePickerCountForTest() int {
	m := &Model{}
	return m.themePickerCount()
}
