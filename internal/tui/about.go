package tui

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/MoonCC233/MCLauncher-TUI/internal/remote"
	"github.com/MoonCC233/MCLauncher-TUI/internal/update"
	"github.com/MoonCC233/MCLauncher-TUI/internal/util"
)

// aboutTab is one page of the About screen.
type aboutTab int

const (
	aboutSoftware aboutTab = iota
	aboutContributors
	aboutReleases
	aboutFeedback
	aboutTabCount
)

// aboutState holds everything the About screen shows. Each tab loads on first
// visit so opening the screen stays instant.
type aboutState struct {
	tab   aboutTab
	force map[aboutTab]bool

	contributors []remote.Contributor
	releases     []update.Release
	issues       []remote.Issue

	cursor  int
	loading aboutTab
	err     string
}

type contributorsMsg struct {
	items []remote.Contributor
	err   error
}

type releasesMsg struct {
	items []update.Release
	err   error
}

type issuesMsg struct {
	items []remote.Issue
	err   error
}

func (m *Model) aboutTabName(tab aboutTab) string {
	switch tab {
	case aboutContributors:
		return m.t("Contributors")
	case aboutReleases:
		return m.t("Releases")
	case aboutFeedback:
		return m.t("Feedback")
	default:
		return m.t("Software")
	}
}

func (m *Model) aboutLoaded(tab aboutTab) bool {
	switch tab {
	case aboutContributors:
		return len(m.about.contributors) > 0
	case aboutReleases:
		return len(m.about.releases) > 0
	case aboutFeedback:
		return len(m.about.issues) > 0
	default:
		return true
	}
}

// aboutLoadCmd fetches the data one tab needs.
func (m *Model) aboutLoadCmd(tab aboutTab, force bool) tea.Cmd {
	client := m.client
	layout := m.layout
	m.about.loading = tab
	m.about.err = ""
	switch tab {
	case aboutContributors:
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			items, err := remote.Contributors(ctx, client, layout.ContributorCache(), force)
			return contributorsMsg{items: items, err: err}
		}
	case aboutReleases:
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			items, err := remote.Releases(ctx, client, layout.ReleaseCache(), force)
			return releasesMsg{items: items, err: err}
		}
	case aboutFeedback:
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			items, err := remote.Issues(ctx, client, layout.IssueCache(), force)
			return issuesMsg{items: items, err: err}
		}
	}
	return nil
}

// switchAboutTab moves between tabs and loads the new one when needed.
func (m *Model) switchAboutTab(tab aboutTab) tea.Cmd {
	if tab < 0 {
		tab = aboutTabCount - 1
	}
	tab %= aboutTabCount
	m.about.tab = tab
	m.about.cursor = 0
	m.about.err = ""
	if m.aboutLoaded(tab) {
		return nil
	}
	return m.aboutLoadCmd(tab, false)
}

func (m *Model) updateAbout(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab", "right", "l":
		return m, m.switchAboutTab(m.about.tab + 1)
	case "shift+tab", "left", "h":
		return m, m.switchAboutTab(m.about.tab - 1)
	case "1", "2", "3", "4":
		index := int(msg.String()[0] - '1')
		return m, m.switchAboutTab(aboutTab(index))
	case "up", "k":
		m.about.cursor--
	case "down", "j":
		m.about.cursor++
	case "pgup":
		m.about.cursor -= m.listHeight()
	case "pgdown":
		m.about.cursor += m.listHeight()
	case "home":
		m.about.cursor = 0
	case "end":
		m.about.cursor = m.aboutRowCount() - 1
	case "enter":
		return m, m.openAboutRow()
	case "r":
		return m, m.aboutLoadCmd(m.about.tab, true)
	case "o":
		_ = util.OpenURL(remote.RepoURL())
	case "n":
		if m.about.tab == aboutFeedback {
			_ = util.OpenURL(remote.NewIssueURL())
		}
	case "a":
		return m, m.openAnnouncements()
	}
	m.about.cursor = clampInt(m.about.cursor, 0, max(0, m.aboutRowCount()-1))
	return m, nil
}

func (m *Model) aboutRowCount() int {
	switch m.about.tab {
	case aboutContributors:
		return len(m.about.contributors)
	case aboutReleases:
		return len(m.about.releases)
	case aboutFeedback:
		return len(m.about.issues)
	default:
		return 0
	}
}

// openAboutRow opens the selected contributor, release or issue in a browser.
func (m *Model) openAboutRow() tea.Cmd {
	row := clampInt(m.about.cursor, 0, max(0, m.aboutRowCount()-1))
	switch m.about.tab {
	case aboutContributors:
		if row < len(m.about.contributors) && m.about.contributors[row].HTMLURL != "" {
			_ = util.OpenURL(m.about.contributors[row].HTMLURL)
		}
	case aboutReleases:
		if row < len(m.about.releases) {
			tag := m.about.releases[row].TagName
			if tag != "" {
				_ = util.OpenURL("https://github.com/" + remote.Repository + "/releases/tag/" + tag)
			}
		}
	case aboutFeedback:
		if row < len(m.about.issues) {
			_ = util.OpenURL(m.about.issues[row].IssueURL())
		}
	}
	return nil
}

func (m *Model) handleContributors(msg contributorsMsg) (tea.Model, tea.Cmd) {
	m.about.loading = -1
	if msg.err != nil {
		m.about.err = msg.err.Error()
		return m, nil
	}
	m.about.contributors = msg.items
	return m, nil
}

func (m *Model) handleReleases(msg releasesMsg) (tea.Model, tea.Cmd) {
	m.about.loading = -1
	if msg.err != nil {
		m.about.err = msg.err.Error()
		return m, nil
	}
	m.about.releases = msg.items
	return m, nil
}

func (m *Model) handleIssues(msg issuesMsg) (tea.Model, tea.Cmd) {
	m.about.loading = -1
	if msg.err != nil {
		m.about.err = msg.err.Error()
		return m, nil
	}
	m.about.issues = msg.items
	return m, nil
}

// viewAbout renders the About screen.
func (m *Model) viewAbout(width, height int) []string {
	lines := []string{
		m.sectionHeader(m.t("About"), m.aboutHeaderRight(), width),
		"",
	}
	// The tab row is clickable, so it needs its real screen row, not the row it
	// used to sit on before the header was added.
	tabRow := bodyTop + len(lines)
	lines = append(lines, m.aboutTabs(width, tabRow), "")
	lines = append(lines, m.aboutTabBody(width)...)
	for len(lines) < height {
		lines = append(lines, "")
	}
	return lines
}

// aboutHeaderRight shows the running version next to the title.
func (m *Model) aboutHeaderRight() string {
	version := strings.TrimSpace(m.AppVersion)
	if version == "" {
		version = "dev"
	}
	return version
}

// aboutTabs renders the four tabs as buttons and records where each one can be
// clicked, so the row works with the mouse as well as with tab and 1-4.
func (m *Model) aboutTabs(contentW, row int) string {
	parts := make([]string, 0, aboutTabCount)
	x := m.contentX() + 2
	clickable := row >= bodyTop && row < bodyTop+m.bodyHeight()
	for tab := aboutTab(0); tab < aboutTabCount; tab++ {
		label := " [ " + m.aboutTabName(tab) + m.aboutTabCount(tab) + " ] "
		rendered := styleDim.Render(label)
		if tab == m.about.tab {
			rendered = lipgloss.NewStyle().
				Background(colPanelAlt).
				Foreground(colPrimary).
				Bold(true).
				Render(label)
		}
		width := ansi.StringWidth(rendered)
		if clickable {
			m.addHit(x, row, width, 1, hitRegion{Kind: hitAboutTab, Scope: int(tab), Screen: ScreenAbout})
		}
		x += width + 1
		parts = append(parts, rendered)
	}
	return "  " + strings.Join(parts, " ")
}

func (m *Model) aboutTabBody(contentW int) []string {
	switch m.about.tab {
	case aboutContributors:
		return m.aboutContributorsBody(contentW)
	case aboutReleases:
		return m.aboutReleasesBody(contentW)
	case aboutFeedback:
		return m.aboutFeedbackBody(contentW)
	default:
		return m.aboutSoftwareBody(contentW)
	}
}

func (m *Model) aboutSoftwareBody(contentW int) []string {
	version := m.AppVersion
	if strings.TrimSpace(version) == "" {
		version = "dev"
	}
	rows := [][2]string{
		{m.t("Version"), version},
		{m.t("Go version"), runtime.Version()},
		{m.t("Platform"), runtime.GOOS + "/" + runtime.GOARCH},
		{m.t("Data directory"), m.layout.Root},
		{m.t("Config file"), m.cfg.Path},
		{m.t("Repository"), remote.RepoURL()},
		{m.t("License"), "MIT"},
	}
	body := make([]string, 0, len(rows)+6)
	for _, row := range rows {
		label := styleMuted.Render(padLabel(row[0], 16))
		value := row[1]
		if value == "" {
			value = styleDim.Render("—")
		} else if ansi.StringWidth(value) > contentW-22 {
			value = truncateRunes(value, max(8, contentW-22))
		}
		body = append(body, "  "+label+" "+value)
	}
	body = append(body, "", "  "+styleBorder.Render(strings.Repeat("─", max(8, contentW-4))), "")
	body = append(body, "  "+styleDim.Render(m.t("Press a to read the announcements again.")))
	body = append(body, "  "+styleDim.Render(m.t("Press o to open the project on GitHub.")))
	return body
}

func (m *Model) aboutContributorsBody(contentW int) []string {
	body := []string{}
	if m.about.err != "" && len(m.about.contributors) == 0 {
		return m.aboutErrorBody()
	}
	if len(m.about.contributors) == 0 {
		return m.aboutPendingBody(aboutContributors)
	}
	visible := m.listHeight()
	start, end := windowIndex(m.about.cursor, len(m.about.contributors), visible)
	for i := start; i < end; i++ {
		person := m.about.contributors[i]
		marker := "  "
		if i == m.about.cursor {
			marker = stylePrimary.Render("▸ ")
		}
		right := m.tf("%d commits", person.Contributions)
		title := fmt.Sprintf("%2d. %s", i+1, person.Login)
		body = append(body, javaPickerRow(marker, title, right, false, i == m.about.cursor, contentW))
	}
	body = append(body, m.aboutPosition(start, end, len(m.about.contributors))...)
	return body
}

func (m *Model) aboutReleasesBody(contentW int) []string {
	if m.about.err != "" && len(m.about.releases) == 0 {
		return m.aboutErrorBody()
	}
	if len(m.about.releases) == 0 {
		return m.aboutPendingBody(aboutReleases)
	}
	body := []string{}
	visible := m.listHeight()
	start, end := windowIndex(m.about.cursor, len(m.about.releases), visible)
	for i := start; i < end; i++ {
		release := m.about.releases[i]
		marker := "  "
		if i == m.about.cursor {
			marker = stylePrimary.Render("▸ ")
		}
		right := ""
		if !release.PublishedAt.IsZero() {
			right = release.PublishedAt.Local().Format("2006-01-02")
		}
		title := release.TagName
		if sameRelease(release.TagName, m.AppVersion) {
			// Mark the release this build belongs to.
			title = stylePrimary.Render("● ") + title
		}
		body = append(body, javaPickerRow(marker, title, right, false, i == m.about.cursor, contentW))
	}
	body = append(body, m.aboutPosition(start, end, len(m.about.releases))...)
	// The selected release's notes are the "update announcement" players want.
	if m.about.cursor < len(m.about.releases) {
		notes := strings.TrimSpace(plainNotes(m.about.releases[m.about.cursor].Body))
		if notes != "" {
			body = append(body, "", "  "+styleBorder.Render(strings.Repeat("─", max(8, contentW-4))), "")
			for _, line := range wrapText(notes, contentW-4) {
				body = append(body, "  "+styleDim.Render(line))
			}
		}
	}
	return body
}

func (m *Model) aboutFeedbackBody(contentW int) []string {
	if m.about.err != "" && len(m.about.issues) == 0 {
		return m.aboutErrorBody()
	}
	if len(m.about.issues) == 0 {
		return m.aboutPendingBody(aboutFeedback)
	}
	body := []string{}
	visible := m.listHeight()
	start, end := windowIndex(m.about.cursor, len(m.about.issues), visible)
	for i := start; i < end; i++ {
		issue := m.about.issues[i]
		marker := "  "
		if i == m.about.cursor {
			marker = stylePrimary.Render("▸ ")
		}
		title := fmt.Sprintf("#%d %s", issue.Number, issue.Title)
		right := fmt.Sprintf("@%s · %d", issue.User.Login, issue.Comments)
		open := strings.EqualFold(issue.State, "open")
		if !open {
			title = styleDim.Render(title)
			right = styleDim.Render(right)
		}
		body = append(body, javaPickerRow(marker, title, right, open, i == m.about.cursor, contentW))
	}
	body = append(body, m.aboutPosition(start, end, len(m.about.issues))...)
	body = append(body, "", "  "+styleDim.Render(m.t("Press n to open a new issue.")))
	return body
}

// aboutTabCount is the number shown inside a tab button, once its data arrived.
func (m *Model) aboutTabCount(tab aboutTab) string {
	var n int
	switch tab {
	case aboutContributors:
		n = len(m.about.contributors)
	case aboutReleases:
		n = len(m.about.releases)
	case aboutFeedback:
		n = len(m.about.issues)
	default:
		return ""
	}
	if n == 0 {
		return ""
	}
	return fmt.Sprintf(" %d", n)
}

// aboutPosition tells the player where they are in a long list.
func (m *Model) aboutPosition(start, end, total int) []string {
	if total <= end-start {
		return nil
	}
	return []string{"", "  " + styleDim.Render(fmt.Sprintf("%d-%d / %d", start+1, end, total))}
}

// sameRelease reports whether a release tag belongs to the running build.
func sameRelease(tag, current string) bool {
	tag = strings.TrimSpace(tag)
	current = strings.TrimSpace(current)
	if tag == "" || current == "" || current == "dev" {
		return false
	}
	if i := strings.IndexAny(current, " ("); i > 0 {
		current = current[:i]
	}
	return strings.TrimPrefix(strings.ToLower(tag), "v") == strings.TrimPrefix(strings.ToLower(current), "v")
}

func (m *Model) aboutPendingBody(tab aboutTab) []string {
	if m.about.loading == tab {
		return []string{"  " + stylePrimary.Render(spinnerFrame(m.spinner)) + " " + styleMuted.Render(m.t("Loading…"))}
	}
	return []string{"  " + styleMuted.Render(m.t("Nothing loaded yet.")), "  " + styleDim.Render(m.t("Press r to load."))}
}

func (m *Model) aboutErrorBody() []string {
	return []string{
		"  " + styleRed.Render(m.t("Could not load the data.")),
		"  " + styleDim.Render(truncateRunes(m.about.err, 60)),
		"  " + styleMuted.Render(m.t("Press r to retry.")),
	}
}
