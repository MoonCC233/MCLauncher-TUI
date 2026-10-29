package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/MoonCC233/MCLauncher-TUI/internal/util"
)

// The header always occupies two lines and the footer one, so the body starts
// at this screen row.
const bodyTop = 2

func (m *Model) View() string {
	m.hits = m.hits[:0]
	m.modalX, m.modalY, m.modalW, m.modalH = 0, 0, 0, 0
	if !m.ready || m.width <= 0 || m.height <= 0 {
		return m.t("Loading mclauncher…")
	}
	if m.modal != modalNone {
		return m.modalView()
	}
	header := m.headerView()
	body := m.bodyView()
	footer := m.footerView()
	return clipLines(header+"\n"+body+"\n"+footer, m.width, m.height)
}

func (m *Model) sidebarWidth() int {
	if m.width < 64 {
		return 0
	}
	w := 22
	if m.width < 88 {
		w = 20
	}
	if w > m.width/3 {
		w = m.width / 3
	}
	return w
}

func (m *Model) mainWidth() int {
	if sw := m.sidebarWidth(); sw > 0 {
		return m.width - sw - 1
	}
	return m.width
}

func (m *Model) contentWidth() int {
	w := m.mainWidth() - 2
	if w < 10 {
		w = 10
	}
	return w
}

// contentX is the screen column of the first content cell. In RTL the
// sidebar moves to the right, so the content stays on the left.
func (m *Model) contentX() int {
	if m.sidebarWidth() == 0 || m.rtl() {
		return 1
	}
	return m.sidebarWidth() + 2
}

// sidebarX is the screen column where the sidebar starts.
func (m *Model) sidebarX() int {
	if !m.rtl() {
		return 0
	}
	return m.mainWidth() + 1
}

func (m *Model) headerView() string {
	left := stylePrimary.Render("◇ ") + styleBold.Render("mclauncher") + " " + styleDim.Render("v"+m.AppVersion)
	var right string
	if m.game != nil {
		right = styleGreen.Render("● ") + styleText.Render(truncateRunes(m.game.InstanceName, 20)) +
			" " + styleDim.Render(elapsedSince(m.game.Started))
	} else {
		if acc := m.activeAccount(); acc != nil {
			right = styleMuted.Render("◉ " + acc.Username)
		} else {
			right = styleDim.Render("○ " + m.t("no account"))
		}
		java, ok := m.javaSummary()
		dot := styleGreen
		if !ok {
			dot = styleRed
		}
		javaPart := dot.Render("● ") + styleDim.Render("Java "+strings.TrimPrefix(java, "Java "))
		if ansi.StringWidth(right)+ansi.StringWidth(javaPart)+6 < m.width/2 {
			right += styleBorder.Render("  │  ") + javaPart
		}
	}
	avail := m.width - 2
	if avail < 10 {
		avail = 10
	}
	line := composeLR("", left, right, avail)
	if m.rtl() {
		line = composeLR("", right, left, avail)
	}
	line = " " + line + " "

	if m.sidebarWidth() > 0 {
		return line + "\n" + strings.Repeat(" ", m.width)
	}
	nav := m.compactNav(avail)
	return line + "\n" + " " + fitLine(nav, avail) + " "
}

func (m *Model) compactNav(width int) string {
	order := make([]int, 0, int(screenCount))
	for i := 0; i < int(screenCount); i++ {
		order = append(order, i)
	}
	if m.rtl() {
		for i, j := 0, len(order)-1; i < j; i, j = i+1, j-1 {
			order[i], order[j] = order[j], order[i]
		}
	}
	var parts []string
	sep := styleBorder.Render(" · ")
	sepW := ansi.StringWidth(sep)
	x := 1
	for _, idx := range order {
		label := fmt.Sprintf("%d %s", idx+1, m.t(screenNameKeys[idx]))
		var styled string
		if Screen(idx) == m.screen {
			styled = lipgloss.NewStyle().Foreground(colPrimary).Bold(true).Render(label)
		} else {
			styled = styleDim.Render(label)
		}
		labelW := ansi.StringWidth(styled)
		// The header adds one leading space, so nav cells occupy the screen
		// columns 1..width. Clamp the hit region to what fitLine can show.
		if x <= width && labelW > 0 {
			hitW := labelW
			if maxW := width - x + 1; hitW > maxW {
				hitW = maxW
			}
			m.addHit(x, 1, hitW, 1, hitRegion{Kind: hitNav, Screen: Screen(idx)})
		}
		parts = append(parts, styled)
		x += labelW + sepW
	}
	return strings.Join(parts, sep)
}

func composeLR(leftPrefix, left, right string, width int) string {
	leftW := ansi.StringWidth(left)
	prefixW := ansi.StringWidth(leftPrefix)
	rightW := ansi.StringWidth(right)
	if rightW > width/2 {
		right = ansi.Truncate(right, width/2, "…")
		rightW = ansi.StringWidth(right)
	}
	gap := width - prefixW - leftW - rightW
	if gap < 1 {
		left = ansi.Truncate(left, max(1, width-prefixW-rightW-1), "…")
		leftW = ansi.StringWidth(left)
		gap = width - prefixW - leftW - rightW
		if gap < 1 {
			return fitLine(leftPrefix+left, width)
		}
	}
	return leftPrefix + left + strings.Repeat(" ", gap) + right
}

func (m *Model) bodyView() string {
	height := m.bodyHeight()
	mainW := m.mainWidth()
	contentW := m.contentWidth()
	lines := m.screenLines(contentW, height)
	main := padBlock(lines, contentW, height)
	for i := range main {
		main[i] = " " + main[i] + " "
	}
	if m.sidebarWidth() == 0 {
		return strings.Join(main, "\n")
	}
	side := m.sidebarView(height)
	sep := styleBorder.Render("│")
	rows := make([]string, height)
	for i := 0; i < height; i++ {
		if m.rtl() {
			rows[i] = fitLine(main[i], mainW) + sep + side[i]
		} else {
			rows[i] = side[i] + sep + fitLine(main[i], mainW)
		}
	}
	return strings.Join(rows, "\n")
}

func (m *Model) sidebarView(height int) []string {
	w := m.sidebarWidth()
	inner := w - 2
	if inner < 8 {
		inner = 8
	}
	x := m.sidebarX()
	var lines []string
	add := func(s string) {
		if m.rtl() {
			if pad := inner - ansi.StringWidth(s); pad > 0 {
				s = strings.Repeat(" ", pad) + s
			}
		}
		lines = append(lines, " "+fitLine(s, inner)+" ")
	}
	add(styleDim.Render(m.t("WORKSPACE")))
	add(styleBorder.Render(strings.Repeat("─", inner)))
	for i := 0; i < int(screenCount); i++ {
		label := m.t(screenNameKeys[i])
		lineIdx := len(lines)
		if Screen(i) == m.screen {
			active := lipgloss.NewStyle().
				Foreground(colPrimary).
				Background(colPanelAlt).
				Bold(true).
				Width(inner).
				Render("▌ " + label)
			add(active)
		} else {
			add(styleMuted.Render("  " + label))
		}
		if lineIdx < height {
			m.addHit(x, bodyTop+lineIdx, w, 1, hitRegion{Kind: hitNav, Screen: Screen(i)})
		}
	}
	add("")
	add(styleDim.Render(m.t("STATUS")))
	if acc := m.activeAccount(); acc != nil {
		add(styleGreen.Render("● ") + styleMuted.Render(truncateRunes(acc.Username, inner-3)))
	} else {
		add(styleDim.Render("○ " + m.t("no account")))
	}
	java, ok := m.javaSummary()
	dot := styleGreen
	if !ok {
		dot = styleRed
	}
	add(dot.Render("● ") + styleMuted.Render(truncateRunes(java, inner-3)))
	if m.game != nil {
		add(stylePurple.Render("● ") + styleMuted.Render(m.t("running")+" "+elapsedSince(m.game.Started)))
	} else if m.task != nil {
		add(stylePrimary.Render("● ") + styleMuted.Render(truncateRunes(m.trKnown(m.task.Stage), inner-3)))
	} else {
		add(styleDim.Render("○ " + m.t("idle")))
	}

	bottom := []string{
		"",
		styleDim.Render(m.t("DATA")),
		styleDim.Render(util.ShortenPath(m.layout.Root, inner)),
		styleDim.Render("v" + m.AppVersion),
	}
	for len(lines)+len(bottom) < height {
		lines = append(lines, "")
	}
	lines = append(lines, bottom...)
	out := make([]string, height)
	for i := 0; i < height; i++ {
		if i < len(lines) {
			out[i] = fitLine(lines[i], w)
		} else {
			out[i] = strings.Repeat(" ", w)
		}
	}
	return out
}

func (m *Model) shiftHits(from, dx int) {
	if dx == 0 {
		return
	}
	for i := from; i < len(m.hits); i++ {
		m.hits[i].X += dx
	}
}

func (m *Model) footerView() string {
	y := m.height - 1
	avail := m.width - 2
	if avail < 10 {
		avail = 10
	}
	right := ""
	if m.game != nil {
		right = styleGreen.Render("● "+m.t("running")) + styleDim.Render(" "+elapsedSince(m.game.Started))
	} else {
		right = styleDim.Render("? " + m.t("help"))
	}
	rightW := ansi.StringWidth(right)

	var left string
	leftW := 0
	if m.toast.Text != "" {
		style := stylePrimaryB
		switch m.toast.Kind {
		case toastSuccess:
			style = styleGreen
		case toastError:
			style = styleRed
		}
		left = style.Render("● ") + styleMuted.Render(m.toast.Text)
		leftW = ansi.StringWidth(left)
	} else if maxHints := avail - rightW - 1; maxHints > 0 {
		// Render at zero first so the hint row can be measured, then move the
		// hit regions to their final column.
		from := len(m.hits)
		left = m.renderHintsWidth(0, y, maxHints, m.screenHints()...)
		leftW = ansi.StringWidth(left)
		hintX := 1
		if m.rtl() {
			hintX = 1 + avail - leftW
		}
		m.shiftHits(from, hintX)
	}

	gap := max(1, avail-leftW-rightW)
	line := left + strings.Repeat(" ", gap) + right
	helpX := 1 + max(0, avail-rightW)
	if m.rtl() {
		line = right + strings.Repeat(" ", gap) + left
		helpX = 1
	}
	if m.game == nil {
		hitW := rightW
		if maxW := 1 + avail - helpX; hitW > maxW {
			hitW = maxW
		}
		m.addHit(helpX, y, hitW, 1, hitRegion{Kind: hitKey, Key: keyMsg("?")})
	}
	return " " + fitLine(line, avail) + " "
}

func (m *Model) screenHints() []hintSpec {
	switch m.screen {
	case ScreenHome:
		return []hintSpec{
			hint("enter", m.t("play")),
			hint("n", m.t("new instance")),
			hint("a", m.t("add account")),
			hint("tab", m.t("sections")),
		}
	case ScreenInstances:
		return []hintSpec{
			hint("enter", m.t("play")),
			hint("n", m.t("new")),
			hint("e", m.t("edit")),
			hint("d", m.t("delete")),
			hint("o", m.t("folder")),
		}
	case ScreenVersions:
		return []hintSpec{
			hint("enter", m.t("create")),
			hint("i", m.t("install")),
			hint("/", m.t("filter")),
			hint("t", m.t("type")),
			hint("r", m.t("refresh")),
		}
	case ScreenAccounts:
		return []hintSpec{
			hint("enter", m.t("activate")),
			hint("a", m.t("add")),
			hint("d", m.t("remove")),
		}
	case ScreenSettings:
		return []hintSpec{
			hint("enter", m.t("edit")),
			hintAction("←→", m.t("adjust"), "right"),
			hint("s", m.t("scan java")),
		}
	case ScreenConsole:
		return []hintSpec{
			hint("x", m.t("stop")),
			hint("c", m.t("clear")),
			hint("o", m.t("logs")),
			hint("G", m.t("follow")),
		}
	case ScreenDownloads:
		if m.task != nil {
			return []hintSpec{
				hint("enter", m.t("open progress")),
				hint("x", m.t("cancel")),
			}
		}
		return []hintSpec{hint("esc", m.t("back to Home"))}
	}
	return []hintSpec{hint("?", m.t("help"))}
}

func elapsedSince(t time.Time) string {
	d := time.Since(t)
	if d < 0 {
		d = 0
	}
	seconds := int(d.Seconds())
	return fmt.Sprintf("%02d:%02d", seconds/60, seconds%60)
}

func (m *Model) screenLines(width, height int) []string {
	switch m.screen {
	case ScreenHome:
		return m.viewHome(width, height)
	case ScreenInstances:
		return m.viewInstances(width, height)
	case ScreenVersions:
		return m.viewVersions(width, height)
	case ScreenAccounts:
		return m.viewAccounts(width, height)
	case ScreenSettings:
		return m.viewSettings(width, height)
	case ScreenConsole:
		return m.viewConsole(width, height)
	case ScreenDownloads:
		return m.viewDownloads(width, height)
	}
	return []string{""}
}
