package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MoonCC233/MCLauncher-TUI/internal/remote"
	"github.com/MoonCC233/MCLauncher-TUI/internal/update"
)

// errTest stands in for a failed fetch.
var errTest = errors.New("boom")

func TestAboutTabsLoadOnDemand(t *testing.T) {
	m := testModel(t)
	if m.about.tab != aboutSoftware {
		t.Fatalf("default tab = %v, want the software tab", m.about.tab)
	}
	// Moving to a tab that has no data yet asks for a fetch.
	if cmd := m.switchAboutTab(aboutContributors); cmd == nil {
		t.Fatal("switching to contributors should start a fetch")
	}
	if m.about.tab != aboutContributors {
		t.Errorf("tab = %v", m.about.tab)
	}

	m.handleContributors(contributorsMsg{items: []remote.Contributor{{Login: "MoonCC233", Contributions: 12}}})
	if cmd := m.switchAboutTab(aboutSoftware); cmd != nil {
		t.Error("the software tab needs no fetch")
	}
	if cmd := m.switchAboutTab(aboutContributors); cmd != nil {
		t.Error("a tab with data should not fetch again")
	}
}

func TestAboutTabKeysAndWrapping(t *testing.T) {
	m := testModel(t)
	m.updateAbout(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("4")})
	if m.about.tab != aboutFeedback {
		t.Errorf("tab = %v, want feedback after pressing 4", m.about.tab)
	}
	m.updateAbout(tea.KeyMsg{Type: tea.KeyTab})
	if m.about.tab != aboutSoftware {
		t.Errorf("tab = %v, want it to wrap around", m.about.tab)
	}
	m.updateAbout(tea.KeyMsg{Type: tea.KeyShiftTab})
	if m.about.tab != aboutFeedback {
		t.Errorf("tab = %v, want it to wrap backwards", m.about.tab)
	}
}

func TestAboutSoftwareBodyShowsTheBuild(t *testing.T) {
	m := testModel(t)
	body := strings.Join(m.aboutSoftwareBody(80), "\n")
	for _, want := range []string{m.AppVersion, remote.RepoURL(), "MIT"} {
		if !strings.Contains(body, want) {
			t.Errorf("software body is missing %q:\n%s", want, body)
		}
	}
}

func TestAboutContributorsAndFeedbackRender(t *testing.T) {
	m := testModel(t)
	m.about.tab = aboutContributors
	m.handleContributors(contributorsMsg{items: []remote.Contributor{
		{Login: "MoonCC233", Contributions: 214},
		{Login: "TeAnli", Contributions: 37},
	}})
	body := strings.Join(m.aboutContributorsBody(80), "\n")
	if !strings.Contains(body, "MoonCC233") || !strings.Contains(body, "214") {
		t.Errorf("contributors body = \n%s", body)
	}

	m.about.tab = aboutFeedback
	m.handleIssues(issuesMsg{items: []remote.Issue{
		{Number: 12, Title: "Mods for 1.19 missing", State: "open", Comments: 3},
		{Number: 11, Title: "Old issue", State: "closed"},
	}})
	body = strings.Join(m.aboutFeedbackBody(80), "\n")
	if !strings.Contains(body, "#12") || !strings.Contains(body, "Mods for 1.19 missing") {
		t.Errorf("feedback body = \n%s", body)
	}
	if !strings.Contains(body, m.t("Press n to open a new issue.")) {
		t.Errorf("feedback body is missing the new-issue hint:\n%s", body)
	}
	if m.aboutRowCount() != 2 {
		t.Errorf("row count = %d, want 2", m.aboutRowCount())
	}
}

func TestAboutReleasesShowTheirNotes(t *testing.T) {
	m := testModel(t)
	m.about.tab = aboutReleases
	m.handleReleases(releasesMsg{items: []update.Release{
		{TagName: "v0.2.2", Body: "新功能：资源与模组浏览器", PublishedAt: time.Now()},
	}})
	body := strings.Join(m.aboutReleasesBody(80), "\n")
	if !strings.Contains(body, "v0.2.2") {
		t.Errorf("releases body = \n%s", body)
	}
	if !strings.Contains(body, "资源与模组浏览器") {
		t.Errorf("the selected release notes should be shown:\n%s", body)
	}
}

func TestAboutErrorsOfferARetry(t *testing.T) {
	m := testModel(t)
	m.about.tab = aboutFeedback
	m.handleIssues(issuesMsg{err: errTest})
	body := strings.Join(m.aboutFeedbackBody(80), "\n")
	if !strings.Contains(body, m.t("Could not load the data.")) {
		t.Errorf("error body = \n%s", body)
	}
	if !strings.Contains(body, m.t("Press r to retry.")) {
		t.Errorf("error body should offer a retry:\n%s", body)
	}
}

func TestAboutTabsAreClickable(t *testing.T) {
	m := testModel(t)
	m.width, m.height = 120, 38
	m.ready = true
	m.switchScreen(ScreenAbout)
	m.View() // render once so the tab row records its hit regions

	for _, tab := range []aboutTab{aboutReleases, aboutFeedback, aboutContributors, aboutSoftware} {
		hit := findHit(t, m, func(h hitRegion) bool {
			return h.Kind == hitAboutTab && h.Scope == int(tab)
		})
		mouseClick(m, hit.X+1, hit.Y)
		if m.about.tab != tab {
			t.Fatalf("clicking tab %v selected %v", tab, m.about.tab)
		}
	}
}

func TestAboutTabHitRegionsFollowTheRenderedRow(t *testing.T) {
	m := testModel(t)
	m.width, m.height = 120, 38
	m.ready = true
	m.switchScreen(ScreenAbout)
	m.View()

	first := findHit(t, m, func(h hitRegion) bool { return h.Kind == hitAboutTab && h.Scope == 0 })
	second := findHit(t, m, func(h hitRegion) bool { return h.Kind == hitAboutTab && h.Scope == 1 })
	if first.Y != second.Y {
		t.Errorf("tabs are not on the same row: %d vs %d", first.Y, second.Y)
	}
	if second.X <= first.X+first.W-1 {
		t.Errorf("second tab overlaps the first: %+v %+v", first, second)
	}
	if first.Y <= bodyTop || first.Y >= bodyTop+m.bodyHeight() {
		t.Errorf("tab row y = %d, want it inside the body", first.Y)
	}
}
