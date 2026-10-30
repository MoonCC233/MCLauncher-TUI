package tui

import (
	"path/filepath"
	"testing"

	"github.com/MoonCC233/MCLauncher-TUI/internal/config"
	"github.com/MoonCC233/MCLauncher-TUI/internal/mirror"
)

func TestDownloadMirrorToggle(t *testing.T) {
	m := testModel(t)
	m.cfg.Path = filepath.Join(t.TempDir(), "config.json")
	defer mirror.Set(mirror.Official)

	if got := m.downloadMirrorLabel(); got != m.t("Official") {
		t.Errorf("default label = %q, want the official source", got)
	}

	m.toggleDownloadMirror()
	if mirror.Current() != mirror.BMCLAPI {
		t.Errorf("mirror = %q, want bmclapi", mirror.Current())
	}
	if m.cfg.Settings.DownloadMirror != string(mirror.BMCLAPI) {
		t.Errorf("stored value = %q", m.cfg.Settings.DownloadMirror)
	}
	if got := m.downloadMirrorLabel(); got != "BMCLAPI" {
		t.Errorf("label = %q, want BMCLAPI", got)
	}

	// The choice has to survive a restart.
	reloaded, err := config.Load(m.cfg.Path)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Settings.DownloadMirror != string(mirror.BMCLAPI) {
		t.Errorf("reloaded value = %q", reloaded.Settings.DownloadMirror)
	}

	m.toggleDownloadMirror()
	if mirror.Current() != mirror.Official {
		t.Errorf("mirror = %q, want it back on the official source", mirror.Current())
	}
	if m.cfg.Settings.DownloadMirror != "" {
		t.Errorf("stored value = %q, want it cleared", m.cfg.Settings.DownloadMirror)
	}
}

func TestDownloadMirrorRowIsInSettings(t *testing.T) {
	m := testModel(t)
	row, ok := findSettingRow(m, "mirror")
	if !ok {
		t.Fatal("settings are missing the download mirror row")
	}
	if row.Value != m.t("Official") {
		t.Errorf("row value = %q", row.Value)
	}
}

// A failed save must not leave the running launcher on a source the next start
// would not load.
func TestDownloadMirrorRevertsWhenTheSaveFails(t *testing.T) {
	m := testModel(t)
	// A directory where the config file should be makes every save fail.
	dir := t.TempDir()
	m.cfg.Path = dir
	defer mirror.Set(mirror.Official)

	before := m.cfg.Settings.DownloadMirror
	m.toggleDownloadMirror()

	if m.cfg.Settings.DownloadMirror != before {
		t.Errorf("setting = %q, want it reverted to %q", m.cfg.Settings.DownloadMirror, before)
	}
	if mirror.Current() != mirror.Official {
		t.Errorf("mirror = %q, want the running launcher to stay on the official source", mirror.Current())
	}
}
