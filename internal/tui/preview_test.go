package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/MoonCC233/MCLauncher-TUI/internal/i18n"
)

func frameLines(t *testing.T, out string, width, height int) []string {
	t.Helper()
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) != height {
		t.Fatalf("frame height = %d, want %d", len(lines), height)
	}
	for i, line := range lines {
		if got := lipgloss.Width(line); got != width {
			t.Fatalf("line %d width = %d, want %d: %q", i, got, width, line)
		}
	}
	return lines
}

var previewScreens = []string{
	"home",
	"instances",
	"versions",
	"accounts",
	"settings",
	"console",
	"downloads",
	"help",
	"form",
	"progress",
	"theme",
	"language",
	"java",
	"account-type",
	"microsoft",
	"microsoft-error",
	"resolution",
	"loader",
	"loader-versions",
	"loader-java",
	"content",
}

func TestPreviewFrames(t *testing.T) {
	for _, screen := range previewScreens {
		t.Run(screen, func(t *testing.T) {
			frameLines(t, Preview(screen, 120, 38), 120, 38)
		})
	}
}

func TestPreviewDefaults(t *testing.T) {
	frameLines(t, Preview("", 0, 0), 120, 38)
}

func TestPreviewNarrow(t *testing.T) {
	lines := frameLines(t, Preview("home", 90, 30), 90, 30)
	if !strings.Contains(strings.Join(lines, "\n"), "mclauncher") {
		t.Error("preview should still render the header")
	}
}

func TestPreviewLanguages(t *testing.T) {
	for _, lang := range i18n.Supported {
		code := string(lang.Code)
		for _, screen := range previewScreens {
			t.Run(code+"/"+screen, func(t *testing.T) {
				frameLines(t, Preview(screen, 100, 30, code), 100, 30)
			})
		}
	}
}

func TestPreviewNarrowLanguages(t *testing.T) {
	for _, code := range []string{"de", "ru", "zh-Hans", "ar", "pt-PT"} {
		for _, screen := range previewScreens {
			t.Run(code+"/"+screen, func(t *testing.T) {
				frameLines(t, Preview(screen, 80, 24, code), 80, 24)
			})
		}
	}
}
