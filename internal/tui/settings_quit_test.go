package tui

import (
	"path/filepath"
	"testing"
)

func TestKeepGameRunningSetting(t *testing.T) {
	m := testModel(t)
	m.cfg.Path = filepath.Join(t.TempDir(), "config.json")

	// Stopping the game on quit stays the default, so existing configs keep the
	// behaviour they had before the option existed.
	if !m.stopsGameOnQuit() {
		t.Fatal("quitting should stop the game by default")
	}
	row, ok := findSettingRow(m, "quitgame")
	if !ok {
		t.Fatal("settings are missing the quit row")
	}
	if row.Value != m.t("Off") {
		t.Errorf("row value = %q, want Off", row.Value)
	}

	m.toggleKeepGameRunning()
	if !m.cfg.Settings.KeepGameRunning {
		t.Error("the setting was not stored")
	}
	if m.stopsGameOnQuit() {
		t.Error("quitting should keep the game running now")
	}
	if row, _ := findSettingRow(m, "quitgame"); row.Value != m.t("On") {
		t.Errorf("row value = %q, want On", row.Value)
	}

	m.toggleKeepGameRunning()
	if m.cfg.Settings.KeepGameRunning || !m.stopsGameOnQuit() {
		t.Error("toggling back should stop the game on quit again")
	}
}

func TestQuitSettingFromSettingsScreen(t *testing.T) {
	m := testModel(t)
	m.cfg.Path = filepath.Join(t.TempDir(), "config.json")
	m.switchScreen(ScreenSettings)
	for i, row := range m.settingsRows() {
		if row.ID == "quitgame" {
			m.settingsCursor = i
		}
	}

	// Enter toggles the row, and so do the arrow keys.
	m.editSetting()
	if !m.cfg.Settings.KeepGameRunning {
		t.Fatal("enter on the row should toggle the setting")
	}
	m.adjustSetting("left")
	if m.cfg.Settings.KeepGameRunning {
		t.Error("left should toggle the setting back")
	}
	m.adjustSetting("right")
	if !m.cfg.Settings.KeepGameRunning {
		t.Error("right should toggle the setting on")
	}
}
