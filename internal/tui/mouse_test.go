package tui

import (
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MoonCC233/MCLauncher-TUI/internal/config"
	"github.com/MoonCC233/MCLauncher-TUI/internal/i18n"
	"github.com/MoonCC233/MCLauncher-TUI/internal/paths"
)

func newTestModel(t *testing.T) *Model {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.Config{DataVersion: config.DataVersion, Settings: config.DefaultSettings()}
	cfg.Path = filepath.Join(dir, "config.json")
	layout := paths.New(dir)
	m := New(cfg, layout, nil, "test", "")
	m.width, m.height = 120, 38
	m.ready = true
	m.View()
	return m
}

func mouseClick(m *Model, x, y int) tea.Cmd {
	_, cmd := m.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: x, Y: y})
	return cmd
}

func mouseWheel(m *Model, button tea.MouseButton, x, y int) tea.Cmd {
	_, cmd := m.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: button, X: x, Y: y})
	return cmd
}

func findHit(t *testing.T, m *Model, match func(hitRegion) bool) hitRegion {
	t.Helper()
	for _, h := range m.hits {
		if match(h) {
			return h
		}
	}
	t.Fatal("hit region not found")
	return hitRegion{}
}

func TestMouseNavigationClick(t *testing.T) {
	m := newTestModel(t)
	hit := findHit(t, m, func(h hitRegion) bool {
		return h.Kind == hitNav && h.Screen == ScreenSettings
	})
	mouseClick(m, hit.X+2, hit.Y)
	if m.screen != ScreenSettings {
		t.Fatalf("screen = %v, want settings", m.screen)
	}
}

func TestMouseSingleClickSelectsWithoutActivating(t *testing.T) {
	m := newTestModel(t)
	m.switchScreen(ScreenSettings)
	m.settingsCursor = 0
	m.View()
	hit := findHit(t, m, func(h hitRegion) bool {
		return h.Kind == hitRow && h.Screen == ScreenSettings && h.Index == 5
	})
	mouseClick(m, hit.X+2, hit.Y)
	if m.settingsCursor != 5 {
		t.Fatalf("settingsCursor = %d, want 5", m.settingsCursor)
	}
	if m.modal != modalNone {
		t.Fatalf("a single click must not activate the row, modal = %v", m.modal)
	}
}

func TestMouseDoubleClickOpensLanguagePicker(t *testing.T) {
	m := newTestModel(t)
	m.switchScreen(ScreenSettings)
	m.settingsCursor = 0
	m.View()
	hit := findHit(t, m, func(h hitRegion) bool {
		return h.Kind == hitRow && h.Screen == ScreenSettings && h.Index == 5
	})
	x, y := hit.X+2, hit.Y
	mouseClick(m, x, y)
	mouseClick(m, x, y)
	if m.modal != modalLanguage {
		t.Fatalf("modal = %v, want language picker", m.modal)
	}
}

func TestMouseLanguageSelection(t *testing.T) {
	m := newTestModel(t)
	m.openLanguagePicker()
	m.View()
	hit := findHit(t, m, func(h hitRegion) bool {
		return h.Kind == hitLanguageRow && h.Index == 5 // zh-Hans
	})
	x, y := hit.X+2, hit.Y
	mouseClick(m, x, y)
	mouseClick(m, x, y)
	if m.modal != modalNone {
		t.Fatalf("modal = %v, want closed", m.modal)
	}
	if m.lang != i18n.ZHHans {
		t.Fatalf("lang = %q, want zh-Hans", m.lang)
	}
	if m.cfg.Settings.Language != string(i18n.ZHHans) {
		t.Fatalf("persisted language = %q", m.cfg.Settings.Language)
	}
}

func TestMouseWheelScrollsSettings(t *testing.T) {
	m := newTestModel(t)
	m.switchScreen(ScreenSettings)
	m.settingsCursor = 0
	m.View()
	mouseWheel(m, tea.MouseButtonWheelDown, 40, 10)
	if m.settingsCursor != 1 {
		t.Fatalf("settingsCursor = %d, want 1", m.settingsCursor)
	}
	mouseWheel(m, tea.MouseButtonWheelUp, 40, 10)
	if m.settingsCursor != 0 {
		t.Fatalf("settingsCursor = %d, want 0", m.settingsCursor)
	}
}

func TestMouseWheelScrollsHelp(t *testing.T) {
	m := newTestModel(t)
	m.modal = modalHelp
	m.View()
	if m.helpScroll != 0 {
		t.Fatalf("helpScroll = %d, want 0", m.helpScroll)
	}
	mouseWheel(m, tea.MouseButtonWheelDown, 40, 10)
	if m.helpScroll == 0 {
		t.Fatal("wheel should scroll the help modal")
	}
}

func TestHitAtUsesTopmostRegion(t *testing.T) {
	m := newTestModel(t)
	m.hits = nil
	m.addHit(0, 0, 10, 2, hitRegion{Kind: hitNav, Screen: ScreenHome})
	m.addHit(2, 1, 2, 1, hitRegion{Kind: hitKey, Key: keyMsg("?")})
	hit, ok := m.hitAt(2, 1)
	if !ok || hit.Kind != hitKey {
		t.Fatalf("hitAt = %+v, ok = %v, want topmost key region", hit, ok)
	}
	if _, ok := m.hitAt(20, 20); ok {
		t.Fatal("hitAt outside every region should fail")
	}
}
