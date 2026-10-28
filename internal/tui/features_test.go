package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/MoonCC233/MCLauncher-TUI/internal/config"
	"github.com/MoonCC233/MCLauncher-TUI/internal/mojang"
	"github.com/MoonCC233/MCLauncher-TUI/internal/paths"
)

func TestVersionScopesAndAprilFools(t *testing.T) {
	m := newTestModel(t)
	m.manifest = &mojang.Manifest{Versions: []mojang.VersionSummary{
		{ID: "1.21.4", Type: "release"},
		{ID: "25w45a", Type: "snapshot"},
		{ID: "2.0", Type: "release"},
		{ID: "15w14a", Type: "snapshot"},
		{ID: "Beta 1.7.3", Type: "old_beta"},
		{ID: "Alpha 1.2.6", Type: "old_alpha"},
	}}

	cases := []struct {
		name  string
		scope int
		want  []string
	}{
		{"all", 0, []string{"1.21.4", "25w45a", "2.0", "15w14a", "Beta 1.7.3", "Alpha 1.2.6"}},
		{"releases", 1, []string{"1.21.4"}},
		{"snapshots", 2, []string{"25w45a"}},
		{"ancient", 3, []string{"Beta 1.7.3", "Alpha 1.2.6"}},
		{"april fools", 4, []string{"2.0", "15w14a"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m.versionScope = tc.scope
			m.rebuildVersions()
			ids := make([]string, 0, len(m.visible))
			for _, version := range m.visible {
				ids = append(ids, version.ID)
			}
			if strings.Join(ids, ",") != strings.Join(tc.want, ",") {
				t.Errorf("scope %d versions = %v, want %v", tc.scope, ids, tc.want)
			}
		})
	}
}

func TestAprilFoolsWhitelist(t *testing.T) {
	want := []string{
		"2.0",
		"15w14a",
		"1.RV-Pre1",
		"3D Shareware v1.34",
		"20w14infinite",
		"22w13oneblockatatime",
		"23w13a_or_b",
		"24w14potato",
		"25w14craftmine",
	}
	for _, id := range want {
		if !isAprilFoolsVersion(id) {
			t.Errorf("%q is not classified as an April Fools version", id)
		}
	}
	if len(aprilFoolsVersions) != len(want) {
		t.Errorf("whitelist has %d entries, want %d", len(aprilFoolsVersions), len(want))
	}
	for _, id := range []string{"1.4", "1.21.4", "25w45a", ""} {
		if isAprilFoolsVersion(id) {
			t.Errorf("%q should not be an April Fools version", id)
		}
	}
}

func TestResolutionPresets(t *testing.T) {
	m := newTestModel(t)
	m.cfg.Settings.Width = 1366
	m.cfg.Settings.Height = 768
	if got := m.currentResolutionPreset(); got != 2 {
		t.Errorf("currentResolutionPreset() = %d, want 2", got)
	}

	m.cfg.Settings.Width = 1024
	m.cfg.Settings.Height = 768
	if got := m.currentResolutionPreset(); got != len(resolutionPresets) {
		t.Errorf("custom resolution preset = %d, want %d", got, len(resolutionPresets))
	}

	m.openResolutionPicker()
	if m.modal != modalResolution || m.resolutionPick != len(resolutionPresets) {
		t.Fatalf("picker = modal %v, row %d", m.modal, m.resolutionPick)
	}
	m.resolutionPick = 4 // 1920 × 1080
	m.chooseResolution()
	if m.cfg.Settings.Width != 1920 || m.cfg.Settings.Height != 1080 {
		t.Errorf("resolution = %d×%d, want 1920×1080", m.cfg.Settings.Width, m.cfg.Settings.Height)
	}
	if m.modal != modalNone {
		t.Errorf("modal = %v, want closed", m.modal)
	}
}

func TestResolutionCustomPrompt(t *testing.T) {
	m := newTestModel(t)
	m.cfg.Settings.Width = 1280
	m.cfg.Settings.Height = 720
	m.openResolutionPicker()
	m.resolutionPick = len(resolutionPresets)
	m.chooseResolution()
	if m.modal != modalPrompt || m.prompt.Kind != promptResolution {
		t.Fatalf("modal = %v, prompt = %v, want custom resolution prompt", m.modal, m.prompt.Kind)
	}
	m.prompt.Input.SetValue("1024x768")
	m.submitPrompt()
	if m.cfg.Settings.Width != 1024 || m.cfg.Settings.Height != 768 {
		t.Errorf("resolution = %d×%d, want 1024×768", m.cfg.Settings.Width, m.cfg.Settings.Height)
	}
	if m.modal != modalNone {
		t.Errorf("modal = %v, want closed", m.modal)
	}
}

func TestApplyDataDir(t *testing.T) {
	m := newTestModel(t)
	newRoot := filepath.Join(t.TempDir(), "game-data")
	m.manifest = &mojang.Manifest{Versions: []mojang.VersionSummary{{ID: "1.21.4", Type: "release"}}}
	m.rebuildVersions()

	model, cmd := m.applyDataDir(newRoot)
	_ = model
	want, err := paths.ResolveDataDir(newRoot)
	if err != nil {
		t.Fatal(err)
	}
	if m.layout.Root != want || m.DataRoot != want {
		t.Errorf("layout root = %q / DataRoot = %q, want %q", m.layout.Root, m.DataRoot, want)
	}
	if m.cfg.Settings.DataDir != want {
		t.Errorf("Settings.DataDir = %q, want %q", m.cfg.Settings.DataDir, want)
	}
	if m.modal != modalNone {
		t.Errorf("modal = %v, want closed", m.modal)
	}
	if m.manifest != nil || !m.manifestLoading {
		t.Errorf("manifest = %v, loading = %v, want a fresh reload", m.manifest, m.manifestLoading)
	}
	if cmd == nil {
		t.Error("switching directories should reload the manifest")
	}
	if _, err := os.Stat(m.layout.Versions); err != nil {
		t.Errorf("new data directory was not created: %v", err)
	}

	reloaded, err := config.Load(m.cfg.Path)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Settings.DataDir != want {
		t.Errorf("persisted DataDir = %q, want %q", reloaded.Settings.DataDir, want)
	}
}

func TestApplyDataDirBlockedWhileBusy(t *testing.T) {
	t.Run("task", func(t *testing.T) {
		m := newTestModel(t)
		m.task = &TaskState{}
		oldRoot := m.layout.Root
		m.applyDataDir(filepath.Join(t.TempDir(), "game-data"))
		if m.modal != modalAlert {
			t.Errorf("modal = %v, want alert", m.modal)
		}
		if m.layout.Root != oldRoot || m.cfg.Settings.DataDir != "" {
			t.Errorf("data directory changed while a task was running")
		}
	})
	t.Run("game", func(t *testing.T) {
		m := newTestModel(t)
		m.game = &GameState{}
		oldRoot := m.layout.Root
		m.applyDataDir(filepath.Join(t.TempDir(), "game-data"))
		if m.modal != modalAlert {
			t.Errorf("modal = %v, want alert", m.modal)
		}
		if m.layout.Root != oldRoot || m.cfg.Settings.DataDir != "" {
			t.Errorf("data directory changed while the game was running")
		}
	})
}

func logoWidth(lines []string) int {
	widest := 0
	for _, line := range lines {
		if w := ansi.StringWidth(line); w > widest {
			widest = w
		}
	}
	return widest
}

func TestRenderLogoFallback(t *testing.T) {
	full := renderLogo(200)
	if len(full) != 5 {
		t.Fatalf("wide logo has %d lines, want 5", len(full))
	}
	if !strings.Contains(ansi.Strip(strings.Join(full, "\n")), "██") {
		t.Error("wide logo has no pixel glyphs")
	}
	if got := logoWidth(full); got != 68 {
		t.Errorf("wide logo width = %d, want 68 with double spacing", got)
	}

	compact := renderLogo(69)
	if got := logoWidth(compact); got != 59 {
		t.Errorf("compact logo width = %d, want 59 with single spacing", got)
	}

	narrow := renderLogo(20)
	if len(narrow) != 1 {
		t.Fatalf("narrow logo has %d lines, want 1", len(narrow))
	}
	if got := ansi.Strip(narrow[0]); got != "MCLAUNCHER" {
		t.Errorf("narrow logo = %q, want MCLAUNCHER", got)
	}
}

func TestLogoUppercaseGlyphs(t *testing.T) {
	for _, r := range "MCLAUNCHER" {
		glyph, ok := logoGlyphs[r]
		if !ok {
			t.Fatalf("missing glyph for %q", r)
		}
		if len(glyph) != 5 {
			t.Errorf("glyph %q has %d rows, want 5", r, len(glyph))
		}
		for _, row := range glyph {
			if len(row) != 5 {
				t.Errorf("glyph %q row %q has width %d, want 5", r, row, len(row))
			}
		}
	}
	for r := range logoGlyphs {
		if r < 'A' || r > 'Z' {
			t.Errorf("logo glyph %q is not uppercase", r)
		}
	}
}
