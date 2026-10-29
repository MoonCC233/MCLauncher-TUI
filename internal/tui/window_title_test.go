package tui

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestTerminalWindowTitleIsFixed(t *testing.T) {
	if TerminalWindowTitle == "" {
		t.Fatal("the terminal title should not be empty")
	}
	if got := windowTitleSequence(TerminalWindowTitle); got != "\x1b]2;MCLauncher TUI\x07" {
		t.Errorf("sequence = %q", got)
	}
}

func TestWindowTitleSequence(t *testing.T) {
	if got := windowTitleSequence("My Launcher"); got != "\x1b]2;My Launcher\x07" {
		t.Errorf("sequence = %q", got)
	}
	// Empty input falls back to the fixed terminal title.
	if got := windowTitleSequence("   "); got != "\x1b]2;"+TerminalWindowTitle+"\x07" {
		t.Errorf("sequence = %q", got)
	}
	// Control characters are stripped so a title cannot inject escapes.
	got := windowTitleSequence("bad\x1b]2;evil\x07name")
	if strings.Count(got, "\x1b") != 1 || strings.Count(got, "\x07") != 1 {
		t.Errorf("sequence = %q, want a single OSC wrapper", got)
	}
}

func TestGameWindowTitleSetting(t *testing.T) {
	m := testModel(t)
	if got := m.gameWindowTitle(); got != "" {
		t.Fatalf("default game title = %q, want empty", got)
	}
	row, ok := findSettingRow(m, "gametitle")
	if !ok {
		t.Fatal("settings are missing the game title row")
	}
	if row.Value != "—" {
		t.Errorf("row value = %q, want the empty marker", row.Value)
	}

	m.cfg.Path = filepath.Join(t.TempDir(), "config.json")
	m.openPrompt(promptGameWindowTitle, m.t("Game window title"), "", "", 80)
	m.prompt.Input.SetValue("  Survival  ")
	if _, _ = m.submitPrompt(); m.cfg.Settings.GameWindowTitle != "Survival" {
		t.Errorf("stored title = %q, want it trimmed", m.cfg.Settings.GameWindowTitle)
	}
	if row, _ := findSettingRow(m, "gametitle"); row.Value != "Survival" {
		t.Errorf("row value = %q, want the new title", row.Value)
	}

	// Clearing the value leaves the game's own title alone.
	m.openPrompt(promptGameWindowTitle, m.t("Game window title"), "", "", 80)
	m.prompt.Input.SetValue("")
	m.submitPrompt()
	if got := m.gameWindowTitle(); got != "" {
		t.Errorf("game title = %q, want empty", got)
	}
	if row, _ := findSettingRow(m, "gametitle"); row.Value != "—" {
		t.Errorf("row value = %q, want the empty marker", row.Value)
	}
}
