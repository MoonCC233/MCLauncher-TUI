package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

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

	// The flat panel has no border. Its title sits one row below the top and
	// two cells in from the left edge, so those offsets locate the panel.
	top, left := -1, -1
	for i, line := range lines {
		if x := cellIndex(line, "Language"); x >= 2 {
			top, left = i-1, x-2
			break
		}
	}
	if top < 0 {
		t.Fatal("modal title not found")
	}
	if left != m.modalX || top != m.modalY {
		t.Fatalf("modal box rendered at (%d,%d), hit regions assume (%d,%d)", left, top, m.modalX, m.modalY)
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
		if x := cellIndex(line, "browse versions"); x >= 0 {
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
	col := cellIndex(row, m.t("help"))
	if col < 0 {
		t.Fatal("footer help button not rendered")
	}
	mouseClick(m, col, m.height-1)
	if m.modal != modalHelp {
		t.Fatalf("modal = %v, want help after clicking the footer hint", m.modal)
	}
}

func TestVersionSearchBarSharesTitleRow(t *testing.T) {
	cases := []struct {
		name  string
		lang  i18n.Lang
		width int
	}{
		{"english-wide", i18n.EN, 120},
		{"english-narrow", i18n.EN, 80},
		{"chinese", i18n.ZHHans, 100},
		{"arabic", i18n.AR, 100},
		{"german-narrow", i18n.DE, 64},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestModel(t)
			m.width, m.height = tc.width, 32
			m.lang = tc.lang
			m.manifest = demoManifest(time.Now())
			m.manifestAt = time.Now().Add(-12 * time.Minute)
			m.switchScreen(ScreenVersions)
			lines := plainView(m)

			row := lines[bodyTop]
			if cellIndex(row, m.t("Versions")) < 0 {
				t.Fatalf("title is not on the first body row: %q", row)
			}
			placeholder := cellIndex(row, m.t("search versions"))
			if placeholder < 0 {
				t.Fatalf("search placeholder is not on the title row: %q", row)
			}
			hit := findHit(t, m, func(h hitRegion) bool { return h.Kind == hitSearch })
			if hit.Y != bodyTop {
				t.Errorf("search hit row = %d, want %d", hit.Y, bodyTop)
			}
			if placeholder < hit.X || placeholder >= hit.X+hit.W {
				t.Errorf("placeholder column %d is outside the search hit %+v", placeholder, hit)
			}
			if cellIndex(lines[bodyTop+2], " "+m.t("all")+" ") < 0 {
				t.Errorf("version chips did not move under the search row: %q", lines[bodyTop+2])
			}
		})
	}
}

func TestVersionSearchHitFocusAndClear(t *testing.T) {
	m := newTestModel(t)
	m.width, m.height = 120, 38
	m.lang = i18n.EN
	m.manifest = demoManifest(time.Now())
	m.manifestAt = time.Now().Add(-12 * time.Minute)
	m.switchScreen(ScreenVersions)
	plainView(m)

	hit := findHit(t, m, func(h hitRegion) bool { return h.Kind == hitSearch })
	mouseClick(m, hit.X+2, hit.Y)
	if !m.filterActive {
		t.Fatal("clicking the search bar did not focus the filter")
	}

	m.Update(keyMsg("1.21"))
	if m.filter != "1.21" {
		t.Fatalf("filter = %q, want 1.21 after typing", m.filter)
	}
	lines := plainView(m)
	if cellIndex(lines[bodyTop], "1.21") < 0 {
		t.Fatalf("typed query is not rendered in the title search bar: %q", lines[bodyTop])
	}

	m.Update(keyMsg("enter"))
	if m.filterActive || m.filter != "1.21" {
		t.Fatalf("after enter: active=%v filter=%q", m.filterActive, m.filter)
	}
	m.Update(keyMsg("esc"))
	if m.filter != "" || m.screen != ScreenVersions {
		t.Fatalf("esc should clear the search without leaving versions: screen=%v filter=%q", m.screen, m.filter)
	}
}

func TestVersionsHeaderKeepsSearchWidth(t *testing.T) {
	for _, width := range []int{56, 64, 80, 100, 120, 160} {
		for _, lang := range []i18n.Lang{i18n.EN, i18n.ZHHans, i18n.AR, i18n.DE} {
			t.Run(fmt.Sprintf("%s/%d", lang, width), func(t *testing.T) {
				m := newTestModel(t)
				m.width, m.height = width, 32
				m.lang = lang
				m.manifest = demoManifest(time.Now())
				m.manifestAt = time.Now().Add(-12 * time.Minute)
				m.switchScreen(ScreenVersions)

				contentW := m.contentWidth()
				titleW, barW, metaW, showMeta := m.versionsHeaderGeometry(contentW)
				if barW < 8 {
					t.Fatalf("search bar width = %d, want at least 8", barW)
				}
				if showMeta && barW < versionsBarMinWidth {
					t.Errorf("metadata shown with only %d search cells", barW)
				}
				used := titleW + versionsHeaderGap + barW
				if showMeta {
					used += versionsHeaderGap + metaW
				}
				if used > contentW {
					t.Errorf("title row uses %d cells, content width is %d", used, contentW)
				}
			})
		}
	}
}

func TestVersionsLayoutWithManifestWarning(t *testing.T) {
	for _, width := range []int{70, 100, 120} {
		for _, warning := range []string{"", "manifest warning: using the cached copy"} {
			t.Run(fmt.Sprintf("%d/%v", width, warning != ""), func(t *testing.T) {
				m := newTestModel(t)
				m.width, m.height = width, 34
				m.lang = i18n.EN
				m.manifest = demoManifest(time.Now())
				m.manifestAt = time.Now().Add(-12 * time.Minute)
				m.switchScreen(ScreenVersions)
				m.manifestErr = warning
				lines := plainView(m)

				if cellIndex(lines[m.height-2], m.t("create")) < 0 {
					t.Fatalf("version detail row was pushed out of the frame: %q", lines[m.height-2])
				}
				if warning != "" {
					if cellIndex(strings.Join(lines, "\n"), "manifest warning") < 0 {
						t.Error("manifest warning is not rendered")
					}
					if got := m.filterBarHeight(m.contentWidth()); got < 2 {
						t.Errorf("filter bar height = %d, want a row for chips plus the warning", got)
					}
				}
			})
		}
	}
}

func TestSearchBarKeepsExactWidth(t *testing.T) {
	m := newTestModel(t)
	m.lang = i18n.EN
	queries := []string{"", "1.21.4", strings.Repeat("snapshot-", 8), "快照版本"}
	for _, width := range []int{5, 8, 16, 26, 40, 80} {
		for _, query := range queries {
			for _, active := range []bool{false, true} {
				m.filter = query
				m.filterActive = active
				m.search.Focus()
				m.search.SetValue(query)
				m.search.CursorEnd()
				bar := m.searchBar(width)
				if got := ansi.StringWidth(bar); got != width {
					t.Errorf("width=%d query=%q active=%v: bar width=%d: %q", width, query, active, got, ansi.Strip(bar))
				}
				if strings.Contains(bar, "\n") {
					t.Errorf("width=%d query=%q active=%v: bar contains a newline", width, query, active)
				}
			}
		}
	}
}
