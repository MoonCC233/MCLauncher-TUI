package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

	"github.com/MoonCC233/MCLauncher-TUI/internal/config"
	"github.com/MoonCC233/MCLauncher-TUI/internal/launch"
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

func TestQuitAppStopsRunningGame(t *testing.T) {
	m := newTestModel(t)
	m.game = &GameState{Session: &launch.Session{}}
	m.quitApp()
	if !m.quit {
		t.Error("quitApp did not mark the model as quitting")
	}
	if !m.game.Stopping {
		t.Error("quitApp did not ask the game session to stop")
	}
}

func TestHomeTitleIsAsciiArt(t *testing.T) {
	previous := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(previous)

	applyTheme(config.DefaultTheme, "")
	for _, tc := range []struct {
		font  string
		width int
		glyph string
	}{
		{"block", 21, "█"},
		{"line", 31, "|"},
	} {
		art := titleArt(homeTitle, tc.font)
		if len(art) != 4 {
			t.Fatalf("%s: wordmark has %d rows, want 4", tc.font, len(art))
		}
		if got := titleArtWidth(art); got != tc.width {
			t.Errorf("%s: wordmark width = %d, want %d", tc.font, got, tc.width)
		}
		if !strings.Contains(strings.Join(art, "\n"), tc.glyph) {
			t.Errorf("%s: wordmark has no %q strokes", tc.font, tc.glyph)
		}
		for i, row := range art {
			if strings.TrimSpace(row) == "" {
				t.Errorf("%s: row %d is empty", tc.font, i)
			}
		}
		painted := paintTitleArt(art, 0, "sheen")
		for i := range painted {
			if ansi.Strip(painted[i]) != art[i] {
				t.Errorf("%s: row %d = %q, want %q", tc.font, i, ansi.Strip(painted[i]), art[i])
			}
		}
	}
	if strings.Join(titleArt(homeTitle, "block"), "\n") == strings.Join(titleArt(homeTitle, "line"), "\n") {
		t.Error("the two fonts should render differently")
	}
	if got := strings.Join(titleArt(homeTitle, "unknown"), "\n"); got != strings.Join(titleArt(homeTitle, config.DefaultTitleFont), "\n") {
		t.Error("unknown font should fall back to the default")
	}
}

func TestHomeTitleAnimationsDiffer(t *testing.T) {
	previous := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(previous)

	applyTheme(config.DefaultTheme, "")
	for _, font := range config.TitleFontIDs {
		art := titleArt(homeTitle, font)
		for _, anim := range []string{"sheen", "wave", "breathe"} {
			first := strings.Join(paintTitleArt(art, 0, anim), "\n")
			later := strings.Join(paintTitleArt(art, 7, anim), "\n")
			if ansi.Strip(first) != strings.Join(art, "\n") || ansi.Strip(later) != strings.Join(art, "\n") {
				t.Fatalf("%s/%s: animation changed the glyphs", font, anim)
			}
			if first == later {
				t.Errorf("%s/%s: frame 7 should differ from frame 0", font, anim)
			}
		}
		if strings.Join(paintTitleArt(art, 0, "off"), "\n") != strings.Join(paintTitleArt(art, 7, "off"), "\n") {
			t.Errorf("%s: off should stay static across frames", font)
		}
	}
}

func TestHomeTitleFallsBackWhenNarrow(t *testing.T) {
	m := testModel(t)
	m.cfg.Settings.TitleAnim = "wave"
	// Ten columns of title plus the two-cell indent of viewHome is the smallest
	// width that still fits the whole letter-spaced wordmark.
	for _, width := range []int{11, 12, 20} {
		lines := m.titleLines(width)
		if len(lines) != 1 {
			t.Fatalf("width %d: title has %d lines, want 1", width, len(lines))
		}
		got := ansi.Strip(lines[0])
		if got != titleDisplay(homeTitle) {
			t.Errorf("width %d: title = %q, want %q", width, got, titleDisplay(homeTitle))
		}
		if ansi.StringWidth(lines[0])+titleIndent > width {
			t.Errorf("width %d: title is %d wide, overflows", width, ansi.StringWidth(lines[0])+titleIndent)
		}
	}
	// Below that the title is clipped, but it must stay visible and inside the
	// available columns.
	for _, width := range []int{3, 4, 5, 8} {
		lines := m.titleLines(width)
		if len(lines) != 1 {
			t.Fatalf("width %d: title has %d lines, want 1", width, len(lines))
		}
		got := ansi.Strip(lines[0])
		if strings.TrimSpace(got) == "" {
			t.Errorf("width %d: title is empty", width)
		}
		if ansi.StringWidth(lines[0])+titleIndent > width {
			t.Errorf("width %d: title is %d wide, overflows", width, ansi.StringWidth(lines[0])+titleIndent)
		}
	}
	if lines := m.titleLines(120); len(lines) != 4 {
		t.Errorf("wide title has %d lines, want 4", len(lines))
	}
}

func TestAnimationRunsOnlyWhenVisible(t *testing.T) {
	m := testModel(t)
	if !m.animVisible() {
		t.Fatal("the home title should animate")
	}
	if cmd := m.ensureAnimTick(); cmd == nil {
		t.Fatal("the first call should schedule a tick")
	}
	if cmd := m.ensureAnimTick(); cmd != nil {
		t.Error("a second tick should not be scheduled while one is pending")
	}

	m.Update(animTickMsg{})
	if m.animFrame != 1 {
		t.Errorf("frame = %d, want 1", m.animFrame)
	}
	if !m.animRunning {
		t.Error("Update should re-arm the tick while the title is visible")
	}

	// A dialog covers the title: no frames and no new tick.
	m.modal = modalHelp
	if m.animVisible() {
		t.Error("a covered title should not animate")
	}
	m.Update(animTickMsg{})
	if m.animFrame != 1 {
		t.Errorf("frame = %d, want it to stay unchanged while hidden", m.animFrame)
	}
	if m.animRunning {
		t.Error("the tick should stop while the title is hidden")
	}

	// Animations turned off: nothing is scheduled at all.
	m.modal = modalNone
	m.cfg.Settings.TitleAnim = "off"
	if m.animVisible() {
		t.Error("off should not animate")
	}
	if cmd := m.ensureAnimTick(); cmd != nil {
		t.Error("off should not schedule a tick")
	}
}

func TestTitleSettingReportsSaveFailure(t *testing.T) {
	m := testModel(t) // no config path, so every Save fails
	if err := m.setTitleAnim("wave"); err == nil {
		t.Fatal("expected the save to fail")
	}
	if m.toast.Kind != toastError {
		t.Errorf("toast kind = %v, want the error toast", m.toast.Kind)
	}
	if !strings.Contains(m.toast.Text, "save config") && !strings.Contains(m.toast.Text, "Could not save config") {
		t.Errorf("toast = %q, want a save error", m.toast.Text)
	}

	m.toast = ToastState{}
	m.openTitleAnimPicker()
	m.titleAnimPick = 1
	m.chooseTitleAnim()
	if m.toast.Kind == toastSuccess {
		t.Error("the picker must not report success when the save failed")
	}

	m.toast = ToastState{}
	m.cycleTitleAnim(1)
	if m.toast.Kind == toastSuccess {
		t.Error("cycling must not report success when the save failed")
	}

	m.toast = ToastState{}
	m.openTitleFontPicker()
	m.titleFontPick = 1
	m.chooseTitleFont()
	if m.toast.Kind == toastSuccess {
		t.Error("the font picker must not report success when the save failed")
	}
}
