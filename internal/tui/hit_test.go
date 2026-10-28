package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/MoonCC233/MCLauncher-TUI/internal/i18n"
)

// cellIndex returns the terminal cell column where substr starts in line.
func cellIndex(line, substr string) int {
	idx := strings.Index(line, substr)
	if idx < 0 {
		return -1
	}
	return ansi.StringWidth(line[:idx])
}

// plainView renders the frame and strips colors so tests can inspect the exact
// cells the user sees.
func plainView(m *Model) []string {
	return strings.Split(ansi.Strip(m.View()), "\n")
}

func TestCompactNavHitRegionsMatchRenderedLabels(t *testing.T) {
	cases := []struct {
		name string
		lang i18n.Lang
		w    int
	}{
		{"english", i18n.EN, 63},
		{"chinese", i18n.ZHHans, 60},
		{"arabic", i18n.AR, 60},
		{"narrow", i18n.EN, 56},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestModel(t)
			m.width, m.height = tc.w, 30
			m.lang = tc.lang
			m.switchScreen(ScreenHome)
			lines := plainView(m)
			if len(lines) < 2 {
				t.Fatal("frame has no header row")
			}
			row := lines[1]

			visible := 0
			for _, hit := range m.hits {
				if hit.Kind != hitNav || hit.Y != 1 {
					continue
				}
				label := fmt.Sprintf("%d %s", int(hit.Screen)+1, m.t(screenNameKeys[hit.Screen]))
				x := cellIndex(row, label)
				if x < 0 {
					continue
				}
				if hit.X != x {
					t.Errorf("nav %q hit.X = %d, rendered at %d", label, hit.X, x)
				}
				visible++
			}
			if visible < 2 {
				t.Fatalf("only %d nav labels fully visible", visible)
			}

			hit := findHit(t, m, func(h hitRegion) bool {
				return h.Kind == hitNav && h.Screen == ScreenSettings
			})
			mouseClick(m, hit.X, hit.Y)
			if m.screen != ScreenSettings {
				t.Fatalf("screen = %v, want settings after clicking the compact nav", m.screen)
			}
		})
	}
}

func TestSidebarNavHitRegionsMatchRenderedRows(t *testing.T) {
	for _, lang := range []i18n.Lang{i18n.EN, i18n.AR} {
		t.Run(string(lang), func(t *testing.T) {
			m := newTestModel(t)
			m.width, m.height = 120, 38
			m.lang = lang
			m.switchScreen(ScreenHome)
			lines := plainView(m)

			for _, hit := range m.hits {
				if hit.Kind != hitNav {
					continue
				}
				label := m.t(screenNameKeys[hit.Screen])
				if hit.Y < 0 || hit.Y >= len(lines) {
					t.Fatalf("nav %q is outside the frame", label)
				}
				if cellIndex(lines[hit.Y], label) < 0 {
					t.Errorf("nav %q hit row %d does not render that label", label, hit.Y)
				}
				if hit.X+hit.W > m.width {
					t.Errorf("nav %q hit region ends at %d, past width %d", label, hit.X+hit.W, m.width)
				}
			}

			hit := findHit(t, m, func(h hitRegion) bool {
				return h.Kind == hitNav && h.Screen == ScreenSettings
			})
			mouseClick(m, hit.X+2, hit.Y)
			if m.screen != ScreenSettings {
				t.Fatalf("screen = %v, want settings after clicking the sidebar", m.screen)
			}
		})
	}
}

func TestModalHitRegionsMatchRenderedBox(t *testing.T) {
	m := newTestModel(t)
	m.width, m.height = 121, 41
	m.lang = i18n.EN
	m.openLanguagePicker()
	lines := plainView(m)

	top, left := -1, -1
	for i, line := range lines {
		if x := cellIndex(line, "╭"); x >= 0 {
			top, left = i, x
			break
		}
	}
	if top < 0 {
		t.Fatal("modal border not found")
	}
	if left != m.modalX || top != m.modalY {
		t.Fatalf("modal box rendered at (%d,%d), hit regions assume (%d,%d)", left, top, m.modalX, m.modalY)
	}
	if (m.width-m.modalW)%2 == 0 || (m.height-m.modalH)%2 == 0 {
		t.Fatalf("test needs odd centering gaps, got x=%d y=%d", m.width-m.modalW, m.height-m.modalH)
	}

	row, col := -1, -1
	for i, line := range lines {
		if x := cellIndex(line, "Simplified Chinese"); x >= 0 {
			row, col = i, x
			break
		}
	}
	if row < 0 {
		t.Fatal("language row not rendered")
	}
	if col <= m.modalX {
		t.Fatalf("language row starts at %d, outside the modal at %d", col, m.modalX)
	}
	hit, ok := m.hitAt(col, row)
	if !ok || hit.Kind != hitLanguageRow || hit.Index != 5 {
		t.Fatalf("hit at rendered row = %+v, ok = %v, want language row 5", hit, ok)
	}
	mouseClick(m, col, row)
	if m.languagePick != 5 {
		t.Fatalf("languagePick = %d, want 5 after clicking the rendered row", m.languagePick)
	}

	lines = plainView(m)
	escRow, escCol := -1, -1
	for i, line := range lines {
		if x := cellIndex(line, "esc"); x >= 0 {
			escRow, escCol = i, x
			break
		}
	}
	if escRow < 0 {
		t.Fatal("modal footer hint not rendered")
	}
	if escCol < m.modalX || escCol >= m.modalX+m.modalW {
		t.Fatalf("footer hint at %d is outside the modal", escCol)
	}
	mouseClick(m, escCol, escRow)
	if m.modal != modalNone {
		t.Fatalf("modal = %v, want closed after clicking the rendered esc hint", m.modal)
	}
}

func TestInstanceEmptyHintHitMatchesRenderedRow(t *testing.T) {
	m := newTestModel(t)
	m.width, m.height = 100, 32
	m.lang = i18n.EN
	m.switchScreen(ScreenInstances)
	lines := plainView(m)

	row, col := -1, -1
	for i, line := range lines {
		if x := cellIndex(line, "n browse versions"); x >= 0 {
			row, col = i, x
			break
		}
	}
	if row < 0 {
		t.Fatal("inline instance hint not rendered")
	}
	hit, ok := m.hitAt(col, row)
	if !ok || hit.Kind != hitKey || hit.Key.String() != "n" {
		t.Fatalf("hit at rendered hint = %+v, ok = %v, want the n hint", hit, ok)
	}
	mouseClick(m, col, row)
	if m.screen != ScreenVersions {
		t.Fatalf("screen = %v, want versions after clicking the inline hint", m.screen)
	}
}

func TestFormFieldHitMatchesRenderedLabel(t *testing.T) {
	m := newTestModel(t)
	m.width, m.height = 120, 38
	m.lang = i18n.EN
	m.switchScreen(ScreenVersions)
	m.openInstanceForm("1.21.4")
	m.View()
	if m.modal != modalForm || len(m.form.Fields) < 2 {
		t.Fatalf("form not open with two fields: modal=%v fields=%d", m.modal, len(m.form.Fields))
	}
	if m.form.Focus != 0 {
		t.Fatalf("initial focus = %d, want 0", m.form.Focus)
	}

	lines := plainView(m)
	row, col := -1, -1
	for i, line := range lines {
		if x := cellIndex(line, "Memory (MB, optional)"); x >= 0 {
			row, col = i, x
			break
		}
	}
	if row < 0 {
		t.Fatal("second form field label not rendered")
	}
	hit, ok := m.hitAt(col, row)
	if !ok || hit.Kind != hitFormField || hit.Index != 1 {
		t.Fatalf("hit at field label = %+v, ok = %v, want form field 1", hit, ok)
	}
	mouseClick(m, col, row)
	if m.form.Focus != 1 {
		t.Fatalf("form focus = %d, want 1 after clicking the second field", m.form.Focus)
	}
}

func TestVersionChipHitRegionsMatchRenderedChips(t *testing.T) {
	m := newTestModel(t)
	m.width, m.height = 70, 38
	m.lang = i18n.EN
	m.switchScreen(ScreenVersions)
	m.versionScope = 0
	lines := plainView(m)
	chips := m.filterChips()

	found := 0
	for _, hit := range m.hits {
		if hit.Kind != hitChip {
			continue
		}
		if hit.Y < 0 || hit.Y >= len(lines) {
			t.Errorf("chip %d hit row %d is outside the frame", hit.Scope, hit.Y)
			continue
		}
		line := lines[hit.Y]
		if hit.X < 0 || hit.X >= ansi.StringWidth(line) {
			t.Errorf("chip %d hit column %d is outside the frame", hit.Scope, hit.X)
			continue
		}
		text := " " + chips[hit.Scope] + " "
		if cellIndex(line, text) != hit.X {
			t.Errorf("chip %q hit.X = %d, rendered at %d", text, hit.X, cellIndex(line, text))
		}
		found++
	}
	if found != len(chips) {
		t.Fatalf("found %d chip hit regions, want %d", found, len(chips))
	}

	hit := findHit(t, m, func(h hitRegion) bool {
		return h.Kind == hitChip && h.Scope == 3
	})
	mouseClick(m, hit.X+2, hit.Y)
	if m.versionScope != 3 {
		t.Fatalf("versionScope = %d, want 3 after clicking the ancient chip", m.versionScope)
	}
}

func TestFooterHelpHitMatchesRenderedPosition(t *testing.T) {
	m := newTestModel(t)
	m.width, m.height = 120, 38
	m.lang = i18n.EN
	lines := plainView(m)
	if len(lines) != m.height {
		t.Fatalf("frame has %d rows, want %d", len(lines), m.height)
	}
	row := lines[m.height-1]
	col := cellIndex(row, "? help")
	if col < 0 {
		t.Fatal("footer help hint not rendered")
	}
	mouseClick(m, col, m.height-1)
	if m.modal != modalHelp {
		t.Fatalf("modal = %v, want help after clicking the footer hint", m.modal)
	}
}
