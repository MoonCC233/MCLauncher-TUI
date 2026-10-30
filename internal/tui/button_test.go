package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/MoonCC233/MCLauncher-TUI/internal/i18n"
)

func colorBG(hex string) string {
	c, _ := parseHexRGB(hex)
	return fmt.Sprintf("48;2;%d;%d;%dm", c.R, c.G, c.B)
}

func TestFooterButtonsAreMouseFirstAndHover(t *testing.T) {
	previous := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(previous)

	m := newTestModel(t)
	m.width, m.height = 120, 38
	m.lang = i18n.EN
	lines := plainView(m)
	footer := lines[m.height-1]

	if strings.Contains(footer, "? help") || strings.Contains(footer, "enter play") {
		t.Errorf("footer still shows key hints: %q", footer)
	}
	for _, label := range []string{m.t("play"), m.t("new instance"), m.t("add account"), m.t("sections"), m.t("help")} {
		if !strings.Contains(footer, label) {
			t.Errorf("footer button %q is missing: %q", label, footer)
		}
	}

	label := m.t("add account")
	col := cellIndex(footer, label)
	row := m.height - 1
	if col < 0 {
		t.Fatal("add account button not found")
	}

	normal := m.View()
	if _, _ = m.Update(tea.MouseMsg{X: col, Y: row, Action: tea.MouseActionMotion}); m.View() == normal {
		t.Error("moving the mouse over a button did not change its style")
	}
	hovered := m.View()
	if !strings.Contains(hovered, colorBG(string(colPrimary))) {
		t.Error("hovered button does not use the accent fill")
	}

	if _, _ = m.Update(tea.MouseMsg{X: col, Y: row, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}); m.modal != modalForm {
		t.Fatalf("modal = %v, want the offline account form after clicking the add account button", m.modal)
	}
}

func TestButtonPressedAndReleaseStates(t *testing.T) {
	previous := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(previous)

	m := newTestModel(t)
	m.width, m.height = 120, 38
	m.lang = i18n.EN
	m.switchScreen(ScreenConsole)
	lines := plainView(m)
	footer := lines[m.height-1]
	col := cellIndex(footer, m.t("clear"))
	row := m.height - 1
	if col < 0 {
		t.Fatal("clear button not found")
	}

	m.Update(tea.MouseMsg{X: col, Y: row, Action: tea.MouseActionMotion})
	if _, _ = m.Update(tea.MouseMsg{X: col, Y: row, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}); !m.mouseDown {
		t.Fatal("mouseDown was not set on press")
	}
	if !strings.Contains(m.View(), colorBG(string(colPrimaryDim))) {
		t.Error("pressed button does not use the dimmed accent fill")
	}
	if _, _ = m.Update(tea.MouseMsg{X: col, Y: row, Button: tea.MouseButtonLeft, Action: tea.MouseActionRelease}); m.mouseDown {
		t.Error("mouseDown was not cleared on release")
	}
}

func TestModalFooterButtonClick(t *testing.T) {
	m := newTestModel(t)
	m.width, m.height = 120, 38
	m.lang = i18n.EN
	m.openLanguagePicker()
	lines := plainView(m)
	row, col := -1, -1
	for i := len(lines) - 1; i >= 0; i-- {
		if x := cellIndex(lines[i], m.t("choose")); x >= 0 {
			row, col = i, x
			break
		}
	}
	if row < 0 {
		t.Fatal("modal choose button not found")
	}
	mouseClick(m, col, row)
	if m.modal != modalNone {
		t.Fatalf("modal = %v, want it closed after clicking choose", m.modal)
	}
}

func TestModalFooterButtonsWrapInsteadOfDisappearing(t *testing.T) {
	m := newTestModel(t)
	parts := []hintSpec{
		hint("1", "switch type"),
		hint("2", "toggle"),
		hint("3", "search"),
		hint("4", "import"),
		hint("5", "from a URL"),
		hint("6", "remove"),
	}
	rows := m.layoutButtonRows(40, parts)
	if len(rows) < 2 {
		t.Fatalf("rows = %d, want the footer to wrap", len(rows))
	}
	var got []string
	for _, row := range rows {
		for _, part := range row {
			got = append(got, part.Label)
		}
	}
	if len(got) != len(parts) {
		t.Fatalf("wrapped labels = %v, want all %d buttons", got, len(parts))
	}
	for i, part := range parts {
		if got[i] != part.Label {
			t.Errorf("label %d = %q, want %q", i, got[i], part.Label)
		}
	}
}
