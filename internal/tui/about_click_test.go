package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// The buttons have to be clickable exactly where they are drawn: the tab row
// moved down when the section header was added, and the hit regions did not.
func TestAboutTabHitsMatchTheRenderedButtons(t *testing.T) {
	m := testModel(t)
	m.width, m.height = 110, 34
	m.ready = true
	m.switchScreen(ScreenAbout)
	lines := strings.Split(ansi.Strip(m.View()), "\n")

	hits := map[int]hitRegion{}
	for _, hit := range m.hits {
		if hit.Kind == hitAboutTab {
			hits[hit.Scope] = hit
		}
	}
	if len(hits) != int(aboutTabCount) {
		t.Fatalf("found %d tab hit regions, want %d", len(hits), aboutTabCount)
	}

	for tab := aboutTab(0); tab < aboutTabCount; tab++ {
		hit := hits[int(tab)]
		label := "[ " + m.aboutTabName(tab)
		if hit.Y < 0 || hit.Y >= len(lines) {
			t.Fatalf("tab %v is clickable at y=%d, outside the frame", tab, hit.Y)
		}
		index := strings.Index(lines[hit.Y], label)
		if index < 0 {
			t.Fatalf("tab %v is clickable at row %d, but that row is %q", tab, hit.Y, lines[hit.Y])
		}
		// The pill is drawn with one space of padding on each side, and that
		// padding belongs to the button: the hit region starts one column before
		// the bracket and ends one column after it.
		pillStart := ansi.StringWidth(lines[hit.Y][:index]) - 1
		pillWidth := ansi.StringWidth(" " + label + " ] ")
		if pillStart != hit.X {
			t.Errorf("tab %v: pill starts at column %d, clickable at %d", tab, pillStart, hit.X)
		}
		if hit.W != pillWidth {
			t.Errorf("tab %v: width %d, want %d", tab, hit.W, pillWidth)
		}
	}
}

// Clicking each button must select exactly that tab.
func TestAboutTabsClickSelectsTheTab(t *testing.T) {
	m := testModel(t)
	m.width, m.height = 110, 34
	m.ready = true
	m.switchScreen(ScreenAbout)
	m.View()

	for _, tab := range []aboutTab{aboutFeedback, aboutContributors, aboutReleases, aboutSoftware} {
		hit := findHit(t, m, func(h hitRegion) bool {
			return h.Kind == hitAboutTab && h.Scope == int(tab)
		})
		mouseClick(m, hit.X+1, hit.Y)
		if m.about.tab != tab {
			t.Fatalf("clicking %v selected %v", tab, m.about.tab)
		}
	}
}
