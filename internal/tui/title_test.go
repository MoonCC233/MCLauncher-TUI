package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MoonCC233/MCLauncher-TUI/internal/config"
)

func TestTitleAnimationSetting(t *testing.T) {
	m := testModel(t)
	if got := m.titleAnim(); got != config.DefaultTitleAnim {
		t.Fatalf("default animation = %q, want %q", got, config.DefaultTitleAnim)
	}

	row, ok := findSettingRow(m, "titleanim")
	if !ok {
		t.Fatal("settings are missing the title animation row")
	}
	if row.Value != m.t("Sheen") {
		t.Errorf("row value = %q, want %q", row.Value, m.t("Sheen"))
	}

	// Right/left on the settings row walk through the animations.
	m.cycleTitleAnim(1)
	if m.titleAnim() != "wave" {
		t.Errorf("after cycling forward = %q, want wave", m.titleAnim())
	}
	if m.cfg.Settings.TitleAnim != "wave" {
		t.Errorf("setting not stored: %q", m.cfg.Settings.TitleAnim)
	}
	m.cycleTitleAnim(-1)
	if m.titleAnim() != config.DefaultTitleAnim {
		t.Errorf("after cycling back = %q, want %q", m.titleAnim(), config.DefaultTitleAnim)
	}
	m.cycleTitleAnim(-1)
	if m.titleAnim() != "off" {
		t.Errorf("cycling before the first entry should wrap, got %q", m.titleAnim())
	}

	// An unknown value falls back instead of breaking the home screen.
	m.cfg.Settings.TitleAnim = "nope"
	if got := m.titleAnim(); got != config.DefaultTitleAnim {
		t.Errorf("invalid setting = %q, want %q", got, config.DefaultTitleAnim)
	}
}

func TestTitleAnimPickerChooses(t *testing.T) {
	m := testModel(t)
	m.openTitleAnimPicker()
	if m.modal != modalTitleAnim {
		t.Fatalf("modal = %v, want the title animation picker", m.modal)
	}
	body, hits := m.titleAnimPickerBody(60)
	if len(body) != len(config.TitleAnimIDs) || len(hits) != len(config.TitleAnimIDs) {
		t.Fatalf("picker has %d rows and %d hits, want %d", len(body), len(hits), len(config.TitleAnimIDs))
	}

	m.titleAnimPick = len(config.TitleAnimIDs) - 1
	m.chooseTitleAnim()
	if m.modal != modalNone {
		t.Errorf("modal = %v, want none after choosing", m.modal)
	}
	if m.cfg.Settings.TitleAnim != "off" {
		t.Errorf("chosen animation = %q, want off", m.cfg.Settings.TitleAnim)
	}

	// The picker also reacts to the keyboard.
	m.openTitleAnimPicker()
	m.updateTitleAnimPicker(tea.KeyMsg{Type: tea.KeyUp})
	if m.titleAnimPick != len(config.TitleAnimIDs)-2 {
		t.Errorf("picker cursor = %d, want %d", m.titleAnimPick, len(config.TitleAnimIDs)-2)
	}
	m.updateTitleAnimPicker(tea.KeyMsg{Type: tea.KeyEsc})
	if m.modal != modalNone {
		t.Errorf("esc should close the picker")
	}
}

func TestHomeTitleUsesConfiguredAnimation(t *testing.T) {
	m := testModel(t)
	m.width, m.height = 100, 30
	m.cfg.Settings.TitleAnim = "off"
	lines := m.titleLines(m.width)
	if len(lines) != 4 {
		t.Fatalf("title has %d lines, want 4", len(lines))
	}
	home := strings.Join(m.viewHome(m.width, m.height), "\n")
	for _, row := range titleArt(homeTitle, m.titleFont()) {
		if !strings.Contains(home, row) {
			t.Errorf("home view is missing wordmark row %q:\n%s", row, home)
		}
	}
}

func findSettingRow(m *Model, id string) (settingsRow, bool) {
	for _, row := range m.settingsRows() {
		if row.ID == id {
			return row, true
		}
	}
	return settingsRow{}, false
}

func TestTitleFontSetting(t *testing.T) {
	m := testModel(t)
	if got := m.titleFont(); got != config.DefaultTitleFont {
		t.Fatalf("default font = %q, want %q", got, config.DefaultTitleFont)
	}
	row, ok := findSettingRow(m, "titlefont")
	if !ok {
		t.Fatal("settings are missing the title style row")
	}
	if row.Value != m.t("Block") {
		t.Errorf("row value = %q, want %q", row.Value, m.t("Block"))
	}

	m.cycleTitleFont(1)
	if m.titleFont() != "line" {
		t.Errorf("after cycling forward = %q, want line", m.titleFont())
	}
	if m.cfg.Settings.TitleFont != "line" {
		t.Errorf("setting not stored: %q", m.cfg.Settings.TitleFont)
	}
	m.cycleTitleFont(-1)
	if m.titleFont() != config.DefaultTitleFont {
		t.Errorf("after cycling back = %q, want %q", m.titleFont(), config.DefaultTitleFont)
	}

	m.cfg.Settings.TitleFont = "nope"
	if got := m.titleFont(); got != config.DefaultTitleFont {
		t.Errorf("invalid setting = %q, want %q", got, config.DefaultTitleFont)
	}
}

func TestTitleFontPickerChooses(t *testing.T) {
	m := testModel(t)
	m.openTitleFontPicker()
	if m.modal != modalTitleFont {
		t.Fatalf("modal = %v, want the title style picker", m.modal)
	}
	body, hits := m.titleFontPickerBody(60)
	if len(hits) != len(config.TitleFontIDs) {
		t.Fatalf("picker has %d rows, want %d", len(hits), len(config.TitleFontIDs))
	}
	// The preview below the list shows the highlighted font.
	if len(body) <= len(config.TitleFontIDs) {
		t.Errorf("picker body has no preview lines: %v", body)
	}

	m.titleFontPick = len(config.TitleFontIDs) - 1
	m.chooseTitleFont()
	if m.modal != modalNone {
		t.Errorf("modal = %v, want none after choosing", m.modal)
	}
	if m.cfg.Settings.TitleFont != "line" {
		t.Errorf("chosen font = %q, want line", m.cfg.Settings.TitleFont)
	}

	m.openTitleFontPicker()
	m.updateTitleFontPicker(tea.KeyMsg{Type: tea.KeyUp})
	if m.titleFontPick != 0 {
		t.Errorf("picker cursor = %d, want 0", m.titleFontPick)
	}
	m.updateTitleFontPicker(tea.KeyMsg{Type: tea.KeyEsc})
	if m.modal != modalNone {
		t.Errorf("esc should close the picker")
	}
}
