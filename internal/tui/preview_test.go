package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
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

func TestPreviewFrames(t *testing.T) {
	screens := []string{
		"home",
		"instances",
		"versions",
		"accounts",
		"settings",
		"console",
		"help",
		"form",
		"progress",
	}
	for _, screen := range screens {
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
