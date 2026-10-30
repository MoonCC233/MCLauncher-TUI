package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/MoonCC233/MCLauncher-TUI/internal/i18n"
)

func TestModalStyleIsFlatAndDimsBackground(t *testing.T) {
	m := newTestModel(t)
	m.width, m.height = 120, 38
	m.lang = i18n.EN
	m.openLanguagePicker()

	raw := m.View()
	for _, corner := range []string{"╭", "╮", "╰", "╯"} {
		if strings.Contains(ansi.Strip(raw), corner) {
			t.Errorf("flat modal still renders rounded corner %q", corner)
		}
	}
	if !strings.Contains(raw, "\x1b[2m") {
		t.Error("background content is not dimmed")
	}

	lines := strings.Split(ansi.Strip(raw), "\n")
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "mclauncher") {
		t.Error("background screen was discarded instead of dimmed")
	}
	header := ""
	for _, line := range lines {
		if strings.Contains(line, m.t("Language")) {
			header = line
			break
		}
	}
	if header == "" {
		t.Fatal("modal header was not rendered")
	}
	if !strings.HasSuffix(strings.TrimRight(header, " "), "esc") {
		t.Errorf("modal header does not end with the esc hint: %q", header)
	}
	if left := cellIndex(header, m.t("Language")) - 2; left != m.modalX {
		t.Errorf("header starts at %d, modalX = %d", left, m.modalX)
	}
	if m.modalW != m.modalWidth() {
		t.Errorf("modalW = %d, want %d", m.modalW, m.modalWidth())
	}
}
