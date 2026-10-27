package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/MoonCC233/MCLauncher-TUI/internal/util"
)

func (m *Model) modalWidth() int {
	w := 76
	if m.width-6 < w {
		w = m.width - 6
	}
	if w < 36 {
		w = 36
	}
	return w
}

func (m *Model) modalContentWidth() int {
	return m.modalWidth() - 6
}

func (m *Model) modalView() string {
	var (
		title  string
		body   []string
		footer string
		accent = colPrimary
	)
	contentW := m.modalContentWidth()

	switch m.modal {
	case modalHelp:
		title = "Help"
		all := m.helpContent(contentW)
		visible := m.height - 11
		if visible < 6 {
			visible = 6
		}
		if visible > len(all) {
			visible = len(all)
		}
		maxScroll := len(all) - visible
		if maxScroll < 0 {
			maxScroll = 0
		}
		if m.helpScroll > maxScroll {
			m.helpScroll = maxScroll
		}
		start := m.helpScroll
		end := start + visible
		if end > len(all) {
			end = len(all)
		}
		body = append(body, all[start:end]...)
		footer = hints(keyHint("↑↓", "scroll"), keyHint("esc", "close"))
	case modalForm:
		accent = colPrimary
		title = m.form.Title
		if m.form.Kind == formInstance {
			body = append(body, styleMuted.Render("Version")+"  "+styleCyan.Render(m.form.VersionID))
			body = append(body, "")
		}
		for i := range m.form.Fields {
			m.form.Fields[i].Width = max(10, contentW-6)
			label := styleMuted.Render(m.form.Labels[i])
			body = append(body, label, "  "+m.form.Fields[i].View())
			if i < len(m.form.Hints) && m.form.Hints[i] != "" {
				body = append(body, styleDim.Render(m.form.Hints[i]))
			}
			body = append(body, styleBorder.Render(strings.Repeat("─", max(12, contentW-4))), "")
		}
		if m.form.Err != "" {
			body = append(body, styleRed.Render("✗ "+m.form.Err), "")
		}
		submit := "create"
		if m.form.Kind == formAccount {
			submit = "add account"
		} else if m.form.EditID != "" {
			submit = "save"
		}
		footer = hints(keyHint("tab", "next field"), keyHint("enter", submit), keyHint("esc", "cancel"))
	case modalConfirm:
		accent = colRed
		title = styleRed.Render(m.confirm.Title)
		body = []string{}
		for _, line := range wrapPlain(m.confirm.Body, contentW-2) {
			body = append(body, styleMuted.Render(line))
		}
		body = append(body, "")
		footer = hints(keyHint("y / enter", "confirm"), keyHint("esc", "cancel"))
	case modalPrompt:
		title = m.prompt.Title
		body = []string{
			styleMuted.Render(m.prompt.Help),
			"",
			m.prompt.Input.View(),
		}
		footer = hints(keyHint("enter", "save"), keyHint("esc", "cancel"))
	case modalJava:
		title = "Java runtime"
		body = m.javaPickerBody(contentW)
		footer = hints(keyHint("enter", "choose"), keyHint("r", "rescan"), keyHint("esc", "cancel"))
	case modalProgress:
		accent = colPrimary
		if m.task != nil {
			title = m.task.Title
		} else {
			title = "Working"
		}
		body = m.progressBody(contentW)
		footer = ""
	case modalAlert:
		accent = colRed
		title = styleRed.Render(m.alert.Title)
		body = []string{}
		wrapW := contentW - 2
		if wrapW < 10 {
			wrapW = 10
		}
		for _, line := range m.alertBodyLines(wrapW) {
			body = append(body, styleMuted.Render(line))
		}
		body = append(body, "")
		footer = hints(keyHint("enter", "dismiss"))
	default:
		return ""
	}

	lines := []string{
		styleBold.Render(title),
		styleBorder.Render(strings.Repeat("─", contentW)),
		"",
	}
	lines = append(lines, body...)
	if footer != "" {
		lines = append(lines, "", footer)
	}
	for i := range lines {
		lines[i] = fitLine(lines[i], contentW)
	}
	// Keep the box inside the terminal on very short windows.
	maxLines := m.height - 6
	if maxLines < 4 {
		maxLines = 4
	}
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accent).
		Padding(1, 2).
		Render(strings.Join(lines, "\n"))
	return clipLines(lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box), m.width, m.height)
}

func (m *Model) javaPickerBody(contentW int) []string {
	var body []string
	if m.javaScanning {
		body = append(body,
			stylePrimary.Render(spinnerFrame(m.spinner))+" "+styleMuted.Render("Scanning for Java runtimes…"),
			"",
		)
	}
	count := m.javaPickerCount()
	visible := m.height - 14
	if visible < 5 {
		visible = 5
	}
	if visible > count {
		visible = count
	}
	if m.javaPicker >= count {
		m.javaPicker = count - 1
	}
	if m.javaPicker < 0 {
		m.javaPicker = 0
	}
	start := m.javaPicker - visible/2
	if start < 0 {
		start = 0
	}
	if start+visible > count {
		start = count - visible
	}
	if start < 0 {
		start = 0
	}
	for i := start; i < start+visible; i++ {
		selected := i == m.javaPicker
		marker := "  "
		if selected {
			marker = stylePrimary.Render("▸ ")
		}
		n := len(m.javaRuntimes)
		switch {
		case i == 0:
			label := "Auto-detect (recommended)"
			right := "no runtime found"
			if summary, ok := m.javaSummary(); ok {
				right = summary
			}
			active := m.cfg.Settings.JavaPath == ""
			body = append(body, javaPickerRow(marker, label, right, active, selected, contentW))
		case i >= 1 && i <= n:
			rt := m.javaRuntimes[i-1]
			active := strings.EqualFold(m.cfg.Settings.JavaPath, rt.Path)
			body = append(body, javaPickerRow(marker, rt.Short(), util.ShortenPath(rt.Path, 34), active, selected, contentW))
		case i == n+1:
			right := "scan the system"
			if m.javaScanning {
				right = spinnerFrame(m.spinner) + " scanning"
			}
			body = append(body, javaPickerRow(marker, "Scan again", right, false, selected, contentW))
		default:
			body = append(body, javaPickerRow(marker, "Use a custom path…", "", false, selected, contentW))
		}
	}
	if count > visible {
		body = append(body, styleDim.Render(fmt.Sprintf("  %d / %d", m.javaPicker+1, count)))
	}
	return body
}

func javaPickerRow(marker, label, right string, active, selected bool, width int) string {
	labelStyle := styleText
	if selected {
		labelStyle = stylePrimaryB
	}
	left := marker + labelStyle.Render(label)
	if active {
		left += " " + styleGreen.Render("●")
	}
	right = util.Truncate(right, max(8, width-ansi.StringWidth(left)-2))
	return composeLR("", left, styleDim.Render(right), width)
}
