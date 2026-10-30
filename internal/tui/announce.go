package tui

import (
	"context"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MoonCC233/MCLauncher-TUI/internal/remote"
)

// announceState holds the announcements fetched from the project, plus which one
// is being read.
type announceState struct {
	items     []remote.Announcement
	scroll    int
	loading   bool
	err       string
	forceOpen bool
}

// announcementsMsg carries the result of one fetch.
type announcementsMsg struct {
	items []remote.Announcement
	err   error
}

// loadAnnouncementsCmd reads the release notes of the running version, through
// the same cache the About screen uses.
func (m *Model) loadAnnouncementsCmd(force bool) tea.Cmd {
	client := m.client
	cache := m.layout.ReleaseCache()
	version := m.AppVersion
	m.announce.loading = true
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		items, err := remote.ReleaseNotes(ctx, client, cache, version, force)
		return announcementsMsg{items: items, err: err}
	}
}

// handleAnnouncements opens the popup when the newest entry has not been read.
func (m *Model) handleAnnouncements(msg announcementsMsg) (tea.Model, tea.Cmd) {
	m.announce.loading = false
	if msg.err != nil {
		m.announce.err = msg.err.Error()
		m.addLog("launcher", "announcements: "+msg.err.Error())
		return m, nil
	}
	m.announce.err = ""
	m.announce.items = msg.items
	if len(msg.items) == 0 {
		m.addLog("launcher", "announcements: none published")
		return m, nil
	}
	if m.announce.forceOpen || m.cfg.Settings.SeenAnnouncement != msg.items[0].ID {
		m.announce.forceOpen = false
		m.announce.scroll = 0
		m.modal = modalAnnounce
	}
	return m, nil
}

// updateAnnounce scrolls the popup and marks the announcements as read on close.
func (m *Model) updateAnnounce(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "enter", "q":
		m.markAnnouncementsSeen()
		m.modal = modalNone
		return m, nil
	case "up", "k":
		m.announce.scroll--
	case "down", "j":
		m.announce.scroll++
	case "pgup":
		m.announce.scroll -= m.announceRows()
	case "pgdown":
		m.announce.scroll += m.announceRows()
	case "home":
		m.announce.scroll = 0
	case "end":
		m.announce.scroll = len(m.announceLines(70))
	}
	m.announce.scroll = clampInt(m.announce.scroll, 0, max(0, len(m.announceLines(70))))
	return m, nil
}

// markAnnouncementsSeen remembers the newest announcement so it does not pop up
// again on the next start.
func (m *Model) markAnnouncementsSeen() {
	if len(m.announce.items) == 0 {
		return
	}
	newest := m.announce.items[0].ID
	if m.cfg.Settings.SeenAnnouncement == newest {
		return
	}
	m.cfg.Settings.SeenAnnouncement = newest
	if err := m.saveConfig(); err != nil {
		m.addLog("launcher", "could not remember the announcement: "+err.Error())
	}
}

// openAnnouncements shows the popup again, for players who want to re-read it.
// It opens even when the announcement was already marked as read.
func (m *Model) openAnnouncements() tea.Cmd {
	if len(m.announce.items) == 0 {
		m.announce.forceOpen = true
		return m.loadAnnouncementsCmd(true)
	}
	m.announce.scroll = 0
	m.modal = modalAnnounce
	return nil
}

// announceRows is how many body lines fit in the popup.
func (m *Model) announceRows() int {
	rows := m.height - 14
	if rows < 4 {
		rows = 4
	}
	return rows
}

// announceLines renders the whole announcement, without the window applied.
func (m *Model) announceLines(contentW int) []string {
	if m.announce.loading && len(m.announce.items) == 0 {
		return []string{"  " + styleMuted.Render(m.t("Loading…"))}
	}
	if len(m.announce.items) == 0 {
		return []string{"  " + styleMuted.Render(m.t("No announcements yet."))}
	}

	entry := m.announce.items[0]
	lines := []string{"  " + stylePrimary.Render(entry.Title)}
	if !entry.Published.IsZero() {
		lines = append(lines, "  "+styleDim.Render(entry.Published.Local().Format("2006-01-02")))
	}
	lines = append(lines, "")
	for _, line := range wrapText(plainNotes(entry.Body), contentW-4) {
		lines = append(lines, "  "+line)
	}
	return lines
}

// announceBody renders the popup content, windowed so long release notes can be
// scrolled instead of being cut off by the modal height.
func (m *Model) announceBody(contentW int) []string {
	lines := m.announceLines(contentW)
	rows := m.announceRows()
	if len(lines) <= rows {
		return lines
	}
	start := clampInt(m.announce.scroll, 0, len(lines)-rows)
	out := append([]string{}, lines[start:start+rows]...)
	if start+rows < len(lines) {
		out = append(out, "  "+styleDim.Render("…"))
	}
	return out
}

// plainNotes turns release notes markdown into something readable in a popup:
// heading markers, list bullets, emphasis and links are stripped, and HTML
// comments (the release workflow leaves a few) are dropped.
func plainNotes(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	var out []string
	skipping := false
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "<!--") {
			skipping = !strings.Contains(trimmed, "-->")
			continue
		}
		if skipping {
			if strings.Contains(trimmed, "-->") {
				skipping = false
			}
			continue
		}
		trimmed = strings.TrimLeft(trimmed, "#")
		trimmed = strings.TrimSpace(trimmed)
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			trimmed = "• " + strings.TrimSpace(trimmed[2:])
		}
		trimmed = strings.ReplaceAll(trimmed, "**", "")
		trimmed = stripMarkdownLinks(trimmed)
		out = append(out, trimmed)
	}
	joined := strings.Join(out, "\n")
	for strings.Contains(joined, "\n\n\n") {
		joined = strings.ReplaceAll(joined, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(joined)
}

// stripMarkdownLinks keeps the label of [label](url) pairs.
func stripMarkdownLinks(text string) string {
	for {
		open := strings.Index(text, "[")
		if open < 0 {
			return text
		}
		close := strings.Index(text[open:], "](")
		if close < 0 {
			return text
		}
		close += open
		end := strings.Index(text[close:], ")")
		if end < 0 {
			return text
		}
		end += close
		text = text[:open] + text[open+1:close] + text[end+1:]
	}
}
