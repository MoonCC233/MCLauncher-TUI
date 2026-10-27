package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/MoonCC233/MCLauncher-TUI/internal/util"
)

func (m *Model) View() string {
	if !m.ready || m.width <= 0 || m.height <= 0 {
		return "Loading mclauncher…"
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
			right = styleDim.Render("○ no account")
		}
		java, ok := m.javaSummary()
		dot := styleGreen
		if !ok {
			dot = styleRed
		}
		javaPart := dot.Render("● ") + styleDim.Render("java "+strings.TrimPrefix(java, "Java "))
		if ansi.StringWidth(right)+ansi.StringWidth(javaPart)+6 < m.width/2 {
			right += styleBorder.Render("  │  ") + javaPart
		}
	}
	avail := m.width - 2
	if avail < 10 {
		avail = 10
	}
	line := composeLR("", left, right, avail)
	line = " " + line + " "

	if m.sidebarWidth() > 0 {
		return line + "\n" + strings.Repeat(" ", m.width)
	}
	nav := m.compactNav(avail)
	return line + "\n" + " " + fitLine(nav, avail) + " "
}

func (m *Model) compactNav(width int) string {
	var parts []string
	for i := 0; i < int(screenCount); i++ {
		label := fmt.Sprintf("%d %s", i+1, screenNames[i])
		if Screen(i) == m.screen {
			parts = append(parts, lipgloss.NewStyle().Foreground(colPrimary).Bold(true).Render(label))
		} else {
			parts = append(parts, styleDim.Render(label))
		}
	}
	return strings.Join(parts, styleBorder.Render(" · "))
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
		rows[i] = side[i] + sep + fitLine(main[i], mainW)
	}
	return strings.Join(rows, "\n")
}

func (m *Model) sidebarView(height int) []string {
	w := m.sidebarWidth()
	inner := w - 2
	if inner < 8 {
		inner = 8
	}
	var lines []string
	add := func(s string) { lines = append(lines, " "+fitLine(s, inner)+" ") }
	add(styleDim.Render("WORKSPACE"))
	add(styleBorder.Render(strings.Repeat("─", inner)))
	for i := 0; i < int(screenCount); i++ {
		label := screenNames[i]
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
	}
	add("")
	add(styleDim.Render("STATUS"))
	if acc := m.activeAccount(); acc != nil {
		add(styleGreen.Render("● ") + styleMuted.Render(truncateRunes(acc.Username, inner-3)))
	} else {
		add(styleDim.Render("○ no account"))
	}
	java, ok := m.javaSummary()
	dot := styleGreen
	if !ok {
		dot = styleRed
	}
	add(dot.Render("● ") + styleMuted.Render(truncateRunes(java, inner-3)))
	if m.game != nil {
		add(stylePurple.Render("● ") + styleMuted.Render("running "+elapsedSince(m.game.Started)))
	} else if m.task != nil {
		add(stylePrimary.Render("● ") + styleMuted.Render(truncateRunes(m.task.Stage, inner-3)))
	} else {
		add(styleDim.Render("○ idle"))
	}

	bottom := []string{
		"",
		styleDim.Render("DATA"),
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

func (m *Model) footerView() string {
	var left string
	if m.toast.Text != "" {
		style := stylePrimaryB
		switch m.toast.Kind {
		case toastSuccess:
			style = styleGreen
		case toastError:
			style = styleRed
		}
		left = style.Render("● ") + styleMuted.Render(m.toast.Text)
	} else {
		left = m.screenHints()
	}
	right := ""
	if m.game != nil {
		right = styleGreen.Render("● running ") + styleDim.Render(elapsedSince(m.game.Started))
	} else {
		right = styleDim.Render("? help")
	}
	avail := m.width - 2
	if avail < 10 {
		avail = 10
	}
	line := composeLR("", left, right, avail)
	return " " + fitLine(line, avail) + " "
}

func (m *Model) screenHints() string {
	switch m.screen {
	case ScreenHome:
		return hints(keyHint("enter", "play"), keyHint("n", "new"), keyHint("a", "account"), keyHint("tab", "sections"))
	case ScreenInstances:
		return hints(keyHint("enter", "play"), keyHint("n", "new"), keyHint("e", "edit"), keyHint("d", "delete"), keyHint("o", "folder"))
	case ScreenVersions:
		return hints(keyHint("enter", "create"), keyHint("i", "install"), keyHint("/", "filter"), keyHint("t", "type"), keyHint("r", "refresh"))
	case ScreenAccounts:
		return hints(keyHint("enter", "activate"), keyHint("a", "add"), keyHint("d", "remove"))
	case ScreenSettings:
		return hints(keyHint("enter", "edit"), keyHint("←→", "adjust"), keyHint("s", "scan java"))
	case ScreenConsole:
		return hints(keyHint("x", "stop"), keyHint("c", "clear"), keyHint("o", "logs"), keyHint("G", "follow"))
	}
	return hints(keyHint("?", "help"))
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
	}
	return []string{""}
}
