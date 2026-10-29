package tui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MoonCC233/MCLauncher-TUI/internal/remote"
)

func announcement(id, title string) remote.Announcement {
	return remote.Announcement{ID: id, Title: title, Body: title + " body", Published: time.Now()}
}

func TestUnseenAnnouncementOpensThePopup(t *testing.T) {
	m := testModel(t)
	items := []remote.Announcement{announcement("a2", "Second"), announcement("a1", "First")}

	m.handleAnnouncements(announcementsMsg{items: items})
	if m.modal != modalAnnounce {
		t.Fatalf("modal = %v, want the announcement popup", m.modal)
	}
	if m.announce.items[0].ID != "a2" {
		t.Fatalf("first item = %q, want the newest", m.announce.items[0].ID)
	}

	body := strings.Join(m.announceBody(70), "\n")
	if !strings.Contains(body, "Second") || !strings.Contains(body, "Second body") {
		t.Errorf("body is missing the announcement:\n%s", body)
	}
}

func TestSeenAnnouncementStaysClosed(t *testing.T) {
	m := testModel(t)
	m.cfg.Settings.SeenAnnouncement = "a2"

	m.handleAnnouncements(announcementsMsg{items: []remote.Announcement{announcement("a2", "Second")}})
	if m.modal == modalAnnounce {
		t.Fatal("an announcement that was already read must not pop up")
	}
}

func TestDismissingRemembersTheNewestID(t *testing.T) {
	m := testModel(t)
	m.cfg.Path = filepath.Join(t.TempDir(), "config.json")
	m.handleAnnouncements(announcementsMsg{items: []remote.Announcement{
		announcement("a2", "Second"),
		announcement("a1", "First"),
	}})

	m.updateAnnounce(tea.KeyMsg{Type: tea.KeyEsc})
	if m.modal != modalNone {
		t.Errorf("modal = %v, want it closed", m.modal)
	}
	if got := m.cfg.Settings.SeenAnnouncement; got != "a2" {
		t.Errorf("seen = %q, want the newest id", got)
	}
}

func TestAnnouncementScrollsLongNotes(t *testing.T) {
	m := testModel(t)
	m.width, m.height = 100, 20
	body := strings.Repeat("一行很长的更新说明。", 60)
	m.handleAnnouncements(announcementsMsg{items: []remote.Announcement{{
		ID: "v9.9.9", Title: "v9.9.9", Body: body,
	}}})

	lines := m.announceLines(70)
	if len(lines) <= m.announceRows() {
		t.Fatalf("the test body is too short: %d lines", len(lines))
	}
	if got := m.announceBody(70); len(got) > m.announceRows()+1 {
		t.Fatalf("popup shows %d lines, want at most %d plus the ellipsis", len(got), m.announceRows())
	}

	m.updateAnnounce(tea.KeyMsg{Type: tea.KeyDown})
	if m.announce.scroll != 1 {
		t.Errorf("scroll = %d, want 1", m.announce.scroll)
	}
	m.updateAnnounce(tea.KeyMsg{Type: tea.KeyEnd})
	if m.announce.scroll < len(lines)-m.announceRows() {
		t.Errorf("end -> scroll %d, want the last window", m.announce.scroll)
	}
	m.updateAnnounce(tea.KeyMsg{Type: tea.KeyHome})
	if m.announce.scroll != 0 {
		t.Errorf("home -> scroll %d, want 0", m.announce.scroll)
	}

	empty := testModel(t)
	if got := strings.Join(empty.announceBody(70), "\n"); !strings.Contains(got, empty.t("No announcements yet.")) {
		t.Errorf("empty body = %q", got)
	}
}

func TestPlainNotesStripsMarkdown(t *testing.T) {
	notes := "## 更新详情\n\n<!-- generated -->\n- 新增资源与模组\n- **窗口标题** 可改\n见 [文档](https://example.test/doc)。\n"
	got := plainNotes(notes)
	for _, unwanted := range []string{"##", "<!--", "**", "](", "https://example.test"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("plain notes still contain %q:\n%s", unwanted, got)
		}
	}
	for _, want := range []string{"更新详情", "• 新增资源与模组", "窗口标题 可改", "文档"} {
		if !strings.Contains(got, want) {
			t.Errorf("plain notes are missing %q:\n%s", want, got)
		}
	}
}
func TestAnnouncementFailureIsQuiet(t *testing.T) {
	m := testModel(t)
	m.handleAnnouncements(announcementsMsg{err: errTest})
	if m.modal == modalAnnounce {
		t.Error("a failed fetch must not open the popup")
	}
	if m.announce.err == "" {
		t.Error("the error should be recorded for the About screen")
	}
}

func TestAnnouncementsCanBeReopenedAfterBeingRead(t *testing.T) {
	m := testModel(t)
	m.cfg.Settings.SeenAnnouncement = "a2"
	m.handleAnnouncements(announcementsMsg{items: []remote.Announcement{announcement("a2", "Second")}})
	if m.modal == modalAnnounce {
		t.Fatal("a read announcement should stay closed on startup")
	}

	// The About screen offers a way back in even when the id was already seen.
	if cmd := m.openAnnouncements(); cmd != nil {
		t.Error("re-opening with cached items should not fetch again")
	}
	if m.modal != modalAnnounce {
		t.Fatalf("modal = %v, want the popup", m.modal)
	}
}

func TestOpenAnnouncementsFetchesWhenEmpty(t *testing.T) {
	m := testModel(t)
	if cmd := m.openAnnouncements(); cmd == nil {
		t.Fatal("re-opening without items should fetch")
	}
	if !m.announce.forceOpen {
		t.Error("the fetch should be allowed to open the popup")
	}
	// A forced fetch opens the popup even though nothing was seen yet.
	m.handleAnnouncements(announcementsMsg{items: []remote.Announcement{announcement("a9", "Fresh")}})
	if m.modal != modalAnnounce {
		t.Fatalf("modal = %v, want the popup", m.modal)
	}
}
