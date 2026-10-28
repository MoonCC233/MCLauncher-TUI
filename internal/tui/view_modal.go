package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/MoonCC233/MCLauncher-TUI/internal/i18n"
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
		footer []hintSpec
		accent = colPrimary
	)
	contentW := m.modalContentWidth()
	var pending []hitRegion

	switch m.modal {
	case modalHelp:
		title = m.t("Help")
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
		footer = []hintSpec{
			hintAction("↑↓", m.t("scroll"), "down"),
			hint("esc", m.t("close")),
		}
	case modalForm:
		accent = colPrimary
		title = m.form.Title
		bodyStart := 3
		focusStart, focusEnd := -1, -1
		if m.form.Kind == formInstance {
			body = append(body, styleMuted.Render(m.t("Version"))+"  "+styleCyan.Render(m.form.VersionID))
			body = append(body, "")
		}
		for i := range m.form.Fields {
			rel := len(body)
			m.form.Fields[i].Width = max(10, contentW-6)
			label := styleMuted.Render(m.form.Labels[i])
			body = append(body, label, "  "+m.form.Fields[i].View())
			pending = append(pending, hitRegion{Kind: hitFormField, Index: i, Y: bodyStart + len(body) - 2, W: contentW, H: 2})
			if i < len(m.form.Hints) && m.form.Hints[i] != "" {
				body = append(body, styleDim.Render(m.form.Hints[i]))
			}
			body = append(body, styleBorder.Render(strings.Repeat("─", max(12, contentW-4))), "")
			if m.form.Focus == i {
				focusStart, focusEnd = bodyStart+rel, bodyStart+rel+3
			}
		}
		for i, choice := range m.form.Choices {
			rel := len(body)
			focused := m.form.Focus == len(m.form.Fields)+i
			marker := "  "
			labelStyle := styleMuted
			valueStyle := styleText
			if focused {
				marker = stylePrimary.Render("▸ ")
				labelStyle = stylePrimaryB
				valueStyle = styleBold
			}
			rowStart := bodyStart + len(body)
			body = append(body, marker+labelStyle.Render(choice.Label), "  "+valueStyle.Render(choice.Value))
			pending = append(pending, hitRegion{Kind: hitFormChoice, Index: i, Y: rowStart, W: contentW, H: 2})
			if choice.Hint != "" {
				body = append(body, "  "+styleDim.Render(choice.Hint))
			}
			body = append(body, styleBorder.Render(strings.Repeat("─", max(12, contentW-4))), "")
			if focused {
				focusStart, focusEnd = bodyStart+rel, bodyStart+rel+3
			}
		}
		if m.form.Err != "" {
			focusStart = bodyStart + len(body)
			body = append(body, styleRed.Render("✗ "+m.form.Err), "")
			focusEnd = bodyStart + len(body) - 1
		}
		maxLines := m.height - 6
		if maxLines < 4 {
			maxLines = 4
		}
		maxBody := maxLines - bodyStart - 2
		if maxBody < 1 {
			maxBody = 1
		}
		start := 0
		if focusStart >= 0 {
			if focusEnd >= start+maxBody {
				start = focusEnd - maxBody + 1
			}
			if focusStart < start {
				start = focusStart
			}
		}
		if start > len(body)-maxBody {
			start = len(body) - maxBody
		}
		if start < 0 {
			start = 0
		}
		if start > 0 {
			body = body[start:]
		}
		if len(body) > maxBody {
			body = body[:maxBody]
		}
		visible := pending[:0]
		for _, hit := range pending {
			hit.Y -= start
			if hit.Y < bodyStart || hit.Y+hit.H > bodyStart+len(body) {
				continue
			}
			visible = append(visible, hit)
		}
		pending = visible
		submit := m.t("create")
		if m.form.Kind == formAccount {
			submit = m.t("add account")
		} else if m.form.EditID != "" {
			submit = m.t("save")
		}
		footer = []hintSpec{
			hint("tab", m.t("next field")),
			hint("enter", submit),
			hint("esc", m.t("cancel")),
		}
	case modalConfirm:
		accent = colRed
		title = styleRed.Render(m.confirm.Title)
		body = []string{}
		for _, line := range wrapPlain(m.confirm.Body, contentW-2) {
			body = append(body, styleMuted.Render(line))
		}
		body = append(body, "")
		footer = []hintSpec{
			hintAction("y / enter", m.t("confirm"), "enter"),
			hint("esc", m.t("cancel")),
		}
	case modalPrompt:
		title = m.prompt.Title
		body = []string{
			styleMuted.Render(m.prompt.Help),
			"",
			m.prompt.Input.View(),
		}
		footer = []hintSpec{
			hint("enter", m.t("save")),
			hint("esc", m.t("cancel")),
		}
	case modalJava:
		title = m.t("Java runtime")
		pickerBody, pickerHits := m.javaPickerBody(contentW)
		body = pickerBody
		pending = append(pending, pickerHits...)
		footer = []hintSpec{
			hint("enter", m.t("choose")),
			hint("r", m.t("rescan")),
			hint("esc", m.t("cancel")),
		}
	case modalLanguage:
		title = m.t("Language")
		pickerBody, pickerHits := m.languagePickerBody(contentW)
		body = pickerBody
		pending = append(pending, pickerHits...)
		footer = []hintSpec{
			hint("enter", m.t("choose")),
			hint("esc", m.t("cancel")),
		}
	case modalAccountAdd:
		title = m.t("Select account type")
		pickerBody, pickerHits := m.accountTypePickerBody(contentW)
		body = pickerBody
		pending = append(pending, pickerHits...)
		footer = []hintSpec{
			hint("enter", m.t("choose")),
			hint("esc", m.t("cancel")),
		}
	case modalMicrosoft:
		accent = colCyan
		title = m.t("Microsoft account")
		body = m.microsoftModalBody(contentW)
		footer = m.microsoftModalHints()
	case modalResolution:
		title = m.t("Window size")
		pickerBody, pickerHits := m.resolutionPickerBody(contentW)
		body = pickerBody
		pending = append(pending, pickerHits...)
		footer = []hintSpec{
			hint("enter", m.t("choose")),
			hint("esc", m.t("cancel")),
		}
	case modalTheme:
		title = m.t("Theme")
		pickerBody, pickerHits := m.themePickerBody(contentW)
		body = pickerBody
		pending = append(pending, pickerHits...)
		footer = []hintSpec{
			hint("enter", m.t("choose")),
			hint("esc", m.t("cancel")),
		}
	case modalLoader:
		title = m.t("Mod loader")
		pickerBody, pickerHits := m.loaderKindPickerBody(contentW)
		body = pickerBody
		pending = append(pending, pickerHits...)
		footer = []hintSpec{
			hint("enter", m.t("choose")),
			hint("esc", m.t("cancel")),
		}
	case modalLoaderVersions:
		title = m.t("Loader version")
		pickerBody, pickerHits := m.loaderVersionPickerBody(contentW)
		body = pickerBody
		pending = append(pending, pickerHits...)
		footer = []hintSpec{
			hint("enter", m.t("choose")),
			hint("r", m.t("retry")),
			hint("esc", m.t("cancel")),
		}
	case modalProgress:
		accent = colPrimary
		if m.task != nil {
			title = m.task.Title
		} else {
			title = m.t("Working")
		}
		body = m.progressBody(contentW)
		footer = []hintSpec{
			hintAction("b", m.t("background"), "b"),
			hintAction("d", m.t("downloads"), "d"),
			hintAction("esc", m.t("cancel"), "esc"),
		}
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
		footer = []hintSpec{hint("enter", m.t("dismiss"))}
	default:
		return ""
	}

	lines := []string{
		styleBold.Render(title),
		styleBorder.Render(strings.Repeat("─", contentW)),
		"",
	}
	lines = append(lines, body...)
	footerIndex := -1
	if len(footer) > 0 {
		lines = append(lines, "")
		footerIndex = len(lines)
		lines = append(lines, "")
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

	boxW := contentW + 6
	boxH := len(lines) + 4
	// lipgloss.Place rounds the leftover gap down when centering, so the hit
	// regions have to use the same floor division or every click inside the
	// modal is shifted by one cell whenever the gap is odd.
	offX := (m.width - boxW) / 2
	offY := (m.height - boxH) / 2
	if offX < 0 {
		offX = 0
	}
	if offY < 0 {
		offY = 0
	}
	contentX := offX + 3
	contentY := offY + 2
	m.modalX, m.modalY, m.modalW, m.modalH = offX, offY, boxW, boxH

	if footerIndex >= 0 && footerIndex < len(lines) {
		// Drop hints that would be cut off by the box so every recorded hit
		// region still corresponds to visible text.
		lines[footerIndex] = fitLine(m.renderHintsWidth(contentX, contentY+footerIndex, contentW, footer...), contentW)
	}
	for _, h := range pending {
		if h.Y < 0 || h.Y >= len(lines) {
			continue
		}
		m.addHit(contentX+h.X, contentY+h.Y, h.W, h.H, h)
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accent).
		Padding(1, 2).
		Render(strings.Join(lines, "\n"))
	return clipLines(lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box), m.width, m.height)
}

func (m *Model) javaPickerBody(contentW int) ([]string, []hitRegion) {
	var body []string
	var hits []hitRegion
	if m.javaScanning {
		body = append(body,
			stylePrimary.Render(spinnerFrame(m.spinner))+" "+styleMuted.Render(m.t("Scanning for Java runtimes…")),
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
		lineIdx := 3 + len(body)
		switch {
		case i == 0:
			label := m.t("Auto-detect (recommended)")
			right := m.t("no runtime found")
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
			right := m.t("scan the system")
			if m.javaScanning {
				right = spinnerFrame(m.spinner) + " " + m.t("scanning")
			}
			body = append(body, javaPickerRow(marker, m.t("Scan again"), right, false, selected, contentW))
		default:
			body = append(body, javaPickerRow(marker, m.t("Use a custom path…"), "", false, selected, contentW))
		}
		hits = append(hits, hitRegion{Kind: hitJavaRow, Index: i, Y: lineIdx, W: contentW, H: 1})
	}
	if count > visible {
		body = append(body, styleDim.Render(fmt.Sprintf("  %d / %d", m.javaPicker+1, count)))
	}
	return body, hits
}

func (m *Model) languagePickerCount() int {
	return len(i18n.Supported) + 1
}

func (m *Model) languagePickerBody(contentW int) ([]string, []hitRegion) {
	count := m.languagePickerCount()
	visible := m.height - 14
	if visible < 5 {
		visible = 5
	}
	if visible > count {
		visible = count
	}
	m.languagePick = clampInt(m.languagePick, 0, count-1)
	start := m.languagePick - visible/2
	if start < 0 {
		start = 0
	}
	if start+visible > count {
		start = count - visible
	}
	if start < 0 {
		start = 0
	}
	var body []string
	var hits []hitRegion
	for i := start; i < start+visible; i++ {
		selected := i == m.languagePick
		marker := "  "
		if selected {
			marker = stylePrimary.Render("▸ ")
		}
		label := m.t("System default")
		right := i18n.NativeName(i18n.Detect())
		active := m.cfg.Settings.Language == ""
		if i > 0 {
			info := i18n.Supported[i-1]
			label = info.Native
			right = info.English + " · " + string(info.Code)
			active = m.cfg.Settings.Language == string(info.Code)
		}
		left := marker + label
		if active {
			left += " " + styleGreen.Render("●")
		}
		right = util.Truncate(right, max(8, contentW-ansi.StringWidth(left)-2))
		line := composeLR("", left, styleDim.Render(right), contentW)
		body = append(body, line)
		hits = append(hits, hitRegion{Kind: hitLanguageRow, Index: i, Y: 3 + len(body) - 1, W: contentW, H: 1})
	}
	if count > visible {
		body = append(body, styleDim.Render(fmt.Sprintf("  %d / %d", m.languagePick+1, count)))
	}
	return body, hits
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
