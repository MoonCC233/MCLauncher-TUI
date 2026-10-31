package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/MoonCC233/MCLauncher-TUI/internal/i18n"
	"github.com/MoonCC233/MCLauncher-TUI/internal/loader"
	"github.com/MoonCC233/MCLauncher-TUI/internal/util"
)

func (m *Model) modalWidth() int {
	w := 76
	switch m.modal {
	case modalPackSearch, modalPackVersions:
		// The download center needs room for cards, filters and version
		// metadata, so it uses a wider panel than the other dialogs.
		w = 104
	case modalForm:
		if m.form.Kind == formInstance && m.form.EditID == "" {
			// The install sheet has a version card and one row per loader.
			w = 92
		}
	}
	if m.width-6 < w {
		w = m.width - 6
	}
	if w < 36 {
		w = 36
	}
	return w
}

func (m *Model) modalContentWidth() int {
	// The flat panel has two cells of horizontal padding and no border.
	return m.modalWidth() - 4
}

// instanceInstallBody renders the new-instance sheet: a version card, the name
// and memory fields, one row per supported loader, and a primary action.
func (m *Model) instanceInstallBody(contentW int) ([]string, []hitRegion) {
	const bodyStart = 3
	var body []string
	var hits []hitRegion

	cardLeft := stylePrimary.Render("← ") + styleGreen.Render("▦") + "  " + styleBold.Render(m.form.VersionID)
	cardRight := styleDim.Render(m.t("Vanilla"))
	if meta, ok := m.loaderMeta[m.form.VersionID]; ok {
		cardRight = styleDim.Render(loaderKindName(meta.Kind))
	}
	body = append(body, composeLR("  ", cardLeft, cardRight, contentW))
	hits = append(hits, hitRegion{Kind: hitKey, Key: keyMsg("esc"), Y: bodyStart + len(body) - 1, W: 3, H: 1})

	versionHint := m.tf("Version %s will be installed first", m.form.VersionID)
	if m.installed[m.form.VersionID] {
		versionHint = m.t("installed")
	}
	body = append(body, fitLine("     "+styleDim.Render(versionHint), contentW), "")

	for i := range m.form.Fields {
		m.form.Fields[i].Width = max(10, contentW-6)
		body = append(body, "  "+styleMuted.Render(m.form.Labels[i]))
		body = append(body, "  "+m.form.Fields[i].View())
		hits = append(hits, hitRegion{Kind: hitFormField, Index: i, Y: bodyStart + len(body) - 2, W: contentW, H: 2})
		if i < len(m.form.Hints) && m.form.Hints[i] != "" && i != 0 {
			body = append(body, "  "+styleDim.Render(m.form.Hints[i]))
		}
		body = append(body, "")
	}

	if m.form.LoaderRow {
		body = append(body, "  "+styleMuted.Render(m.t("Mod loader")), "")
		for i, kind := range formLoaderKinds {
			focused := m.form.Focus == len(m.form.Fields)+i
			bar := styleBorder.Render("▌")
			nameStyle := styleText
			if focused {
				bar = stylePrimary.Render("▌")
				nameStyle = stylePrimaryB
			}
			status := m.t("Can be added")
			if kind == loader.OptiFine {
				status = m.t("Needs a local jar")
			}
			row := composeLR("  "+bar+" ", nameStyle.Render(loaderKindName(kind)),
				styleDim.Render(status)+"  "+stylePrimary.Render("›"), contentW)
			body = append(body, row)
			hits = append(hits, hitRegion{Kind: hitFormLoaderRow, Index: i, Y: bodyStart + len(body) - 1, W: contentW, H: 1})
		}
		body = append(body, "  "+styleDim.Render(m.t("Pick a loader or create a vanilla instance.")), "")
	}

	if m.form.Err != "" {
		body = append(body, "  "+styleRed.Render("✗ "+m.form.Err), "")
	}

	submit := m.t("Start download")
	if m.installed[m.form.VersionID] {
		submit = m.t("create")
	}
	button := lipgloss.NewStyle().Background(colPrimary).Foreground(colBackground).Bold(true).
		Padding(0, 4).Render("  " + submit + "  ")
	body = append(body, "", lipgloss.PlaceHorizontal(contentW, lipgloss.Center, button), "")
	hits = append(hits, hitRegion{Kind: hitFormSubmit, Y: bodyStart + len(body) - 2, W: contentW, H: 1})
	return body, hits
}

func (m *Model) modalView(base string) string {
	var (
		title  string
		body   []string
		footer []hintSpec
	)
	contentW := m.modalContentWidth()
	var pending []hitRegion
	// The dimmed screen is only scenery; clicks inside the modal must never
	// fall through to the rows behind it.
	m.hits = m.hits[:0]

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
		}
	case modalForm:
		title = m.form.Title
		if m.form.Kind == formInstance && m.form.EditID == "" {
			var hits []hitRegion
			body, hits = m.instanceInstallBody(contentW)
			pending = append(pending, hits...)
			footer = nil
			break
		}
		bodyStart := 3
		if m.form.Kind == formInstance {
			body = append(body, styleMuted.Render(m.t("Version"))+"  "+styleCyan.Render(m.form.VersionID))
			body = append(body, "")
		}
		for i := range m.form.Fields {
			m.form.Fields[i].Width = max(10, contentW-6)
			label := styleMuted.Render(m.form.Labels[i])
			body = append(body, label, "  "+m.form.Fields[i].View())
			pending = append(pending, hitRegion{Kind: hitFormField, Index: i, Y: bodyStart + len(body) - 2, W: contentW, H: 2})
			if i < len(m.form.Hints) && m.form.Hints[i] != "" {
				body = append(body, styleDim.Render(m.form.Hints[i]))
			}
			body = append(body, styleBorder.Render(strings.Repeat("─", max(12, contentW-4))), "")
		}
		if m.form.Err != "" {
			body = append(body, styleRed.Render("✗ "+m.form.Err), "")
		}
		submit := m.t("create")
		if m.form.Kind == formAccount {
			submit = m.t("add account")
		} else if m.form.EditID != "" {
			submit = m.t("save")
		}
		footer = []hintSpec{
			hint("tab", m.t("next field")),
			hint("enter", submit),
		}
	case modalConfirm:
		title = m.confirm.Title
		body = []string{}
		for _, line := range wrapPlain(m.confirm.Body, contentW-2) {
			body = append(body, styleMuted.Render(line))
		}
		body = append(body, "")
		footer = []hintSpec{
			hintAction("y / enter", m.t("confirm"), "enter"),
		}
	case modalPrompt:
		title = m.prompt.Title
		body = []string{
			styleMuted.Render(m.prompt.Help),
			"",
			m.prompt.Input.View(),
		}
		if m.prompt.Err != "" {
			body = append(body, "", styleRed.Render("✗ "+m.prompt.Err))
		}
		footer = []hintSpec{
			hint("enter", m.t("save")),
		}
	case modalJava:
		title = m.t("Java runtime")
		pickerBody, pickerHits := m.javaPickerBody(contentW)
		body = pickerBody
		pending = append(pending, pickerHits...)
		footer = []hintSpec{
			hint("enter", m.t("choose")),
			hint("r", m.t("rescan")),
		}
	case modalLanguage:
		title = m.t("Language")
		pickerBody, pickerHits := m.languagePickerBody(contentW)
		body = pickerBody
		pending = append(pending, pickerHits...)
		footer = []hintSpec{
			hint("enter", m.t("choose")),
		}
	case modalAccountAdd:
		title = m.t("Select account type")
		pickerBody, pickerHits := m.accountTypePickerBody(contentW)
		body = pickerBody
		pending = append(pending, pickerHits...)
		footer = []hintSpec{
			hint("enter", m.t("choose")),
		}
	case modalMicrosoft:
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
		}
	case modalTheme:
		title = m.t("Theme")
		pickerBody, pickerHits := m.themePickerBody(contentW)
		body = pickerBody
		pending = append(pending, pickerHits...)
		footer = []hintSpec{
			hint("enter", m.t("choose")),
		}
	case modalTitleAnim:
		title = m.t("Title animation")
		pickerBody, pickerHits := m.titleAnimPickerBody(contentW)
		body = pickerBody
		pending = append(pending, pickerHits...)
		footer = []hintSpec{
			hint("enter", m.t("choose")),
		}
	case modalTitleFont:
		title = m.t("Title style")
		pickerBody, pickerHits := m.titleFontPickerBody(contentW)
		body = pickerBody
		pending = append(pending, pickerHits...)
		footer = []hintSpec{
			hint("enter", m.t("choose")),
		}
	case modalLoader:
		title = m.tf("Install loader for %s", m.loaderPick.Minecraft)
		switch m.loaderPick.Step {
		case loaderStepVersion:
			title = m.tf("Choose a %s build", loaderKindName(m.loaderPick.Kind))
			pickerBody, pickerHits := m.loaderVersionBody(contentW)
			body = pickerBody
			pending = append(pending, pickerHits...)
		case loaderStepJava:
			title = m.t("Choose Java for the installer")
			pickerBody, pickerHits := m.loaderJavaBody(contentW)
			body = pickerBody
			pending = append(pending, pickerHits...)
		default:
			pickerBody, pickerHits := m.loaderKindBody(contentW)
			body = pickerBody
			pending = append(pending, pickerHits...)
		}
		footer = []hintSpec{
			hint("enter", m.t("choose")),
		}

	case modalPacks:
		title = m.t("Resources & mods")
		body = m.packListBody(contentW)
		footer = []hintSpec{
			hint("tab", m.t("switch type")),
			hint("enter", m.t("toggle")),
			hint("/", m.t("search")),
			hint("i", m.t("import")),
			hint("u", m.t("from a URL")),
			hint("d", m.t("remove")),
		}
	case modalPackSearch:
		title = m.packKindName(m.packs.kind) + " · " + m.packSourceName()
		body = m.packSearchBody(contentW)
		footer = []hintSpec{
			hint("tab", m.t("switch type")),
			hint("ctrl+t", m.t("source")),
			hint("ctrl+f", m.t("version")),
			hint("ctrl+k", m.t("loader")),
			hint("ctrl+o", m.t("sort")),
			hint("ctrl+p", m.t("prev page")),
			hint("ctrl+n", m.t("next page")),
			hint("enter", m.t("choose")),
		}
	case modalPackVersions:
		title = m.packs.project.Title
		if strings.TrimSpace(title) == "" {
			title = m.t("Choose a version")
		}
		body = m.packVersionsBody(contentW)
		footer = []hintSpec{
			hint("enter", m.t("install")),
			hint("esc", m.t("back")),
		}
	case modalAnnounce:
		title = m.t("Announcements")
		body = m.announceBody(contentW)
		footer = []hintSpec{
			hint("↑↓", m.t("scroll")),
		}
	case modalSkin:
		title = m.t("Offline skin")
		body = m.skinModalBody(contentW)
		footer = []hintSpec{
			hint("i", m.t("import")),
			hint("m", m.t("model")),
			hint("o", m.t("open")),
			hint("r", m.t("reload")),
		}
	case modalProgress:
		if m.task != nil {
			title = m.task.Title
		} else {
			title = m.t("Working")
		}
		body = m.progressBody(contentW)
		footer = []hintSpec{
			hintAction("b", m.t("background"), "b"),
			hintAction("d", m.t("downloads"), "d"),
		}
	case modalAlert:
		title = m.alert.Title
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

	titleStyle := styleBold
	if m.modal == modalConfirm || m.modal == modalAlert {
		titleStyle = lipgloss.NewStyle().Foreground(colRed).Bold(true)
	}
	header, escX := m.modalHeader(titleStyle.Render(title), contentW)
	footerRows := m.layoutButtonRows(contentW, footer)
	lines := []string{
		header,
		styleBorder.Render(strings.Repeat("─", contentW)),
		"",
	}
	lines = append(lines, body...)
	footerStart := -1
	if len(footerRows) > 0 {
		lines = append(lines, "")
		footerStart = len(lines)
		for range footerRows {
			lines = append(lines, "")
		}
		lines = append(lines, "")
	}
	for i := range lines {
		lines[i] = fitLine(lines[i], contentW)
	}
	// Keep the panel inside the terminal on very short windows.
	maxLines := m.height - 4
	if maxLines < 4 {
		maxLines = 4
	}
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}

	boxW := contentW + 4
	boxH := len(lines) + 2
	offX := (m.width - boxW) / 2
	offY := (m.height - boxH) / 2
	if offX < 0 {
		offX = 0
	}
	if offY < 0 {
		offY = 0
	}
	contentX := offX + 2
	contentY := offY + 1
	m.modalX, m.modalY, m.modalW, m.modalH = offX, offY, boxW, boxH
	m.addHit(contentX+escX, contentY, ansi.StringWidth("esc"), 1, hitRegion{Kind: hitKey, Key: keyMsg("esc")})

	for i, row := range footerRows {
		idx := footerStart + i
		if idx >= len(lines) {
			break
		}
		lines[idx] = fitLine(m.renderHintParts(contentX, contentY+idx, row...), contentW)
	}
	for _, h := range pending {
		if h.Y < 0 || h.Y >= len(lines) {
			continue
		}
		m.addHit(contentX+h.X, contentY+h.Y, h.W, h.H, h)
	}

	return m.overlayModal(base, paintPanel(lines, boxW), offX, offY, boxW)
}

// modalHeader draws the bold title on the left and the esc key on the right.
// It returns the line and the esc key's cell offset inside the content width.
func (m *Model) modalHeader(title string, width int) (string, int) {
	esc := styleMuted.Render("esc")
	escW := ansi.StringWidth(esc)
	titleW := ansi.StringWidth(title)
	if titleW+escW+1 > width {
		title = ansi.Truncate(title, max(1, width-escW-1), "…")
		titleW = ansi.StringWidth(title)
	}
	gap := width - titleW - escW
	if gap < 1 {
		gap = 1
	}
	if m.rtl() {
		return esc + strings.Repeat(" ", gap) + title, 0
	}
	return title + strings.Repeat(" ", gap) + esc, width - escW
}

// paintPanel fills a flat panel background and adds one row of vertical
// padding above and below the content.
func paintPanel(lines []string, width int) []string {
	bg := panelBackgroundSeq()
	out := make([]string, 0, len(lines)+2)
	out = append(out, panelRow("", width, bg))
	for _, line := range lines {
		out = append(out, panelRow(line, width, bg))
	}
	out = append(out, panelRow("", width, bg))
	return out
}

func panelRow(line string, width int, bg string) string {
	line = fitLine(line, max(1, width-4))
	line = "  " + line + "  "
	// Nested lipgloss styles end with a reset, which would also clear the panel
	// background. Re-apply it after every reset so the whole row stays opaque.
	line = strings.ReplaceAll(line, "\x1b[0m", "\x1b[0m"+bg)
	return bg + line + "\x1b[0m"
}

func panelBackgroundSeq() string {
	c := panelColor()
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", c.R, c.G, c.B)
}

// overlayModal draws the flat panel over the screen, leaving the surrounding
// content at its normal brightness.
func (m *Model) overlayModal(base string, panel []string, offX, offY, boxW int) string {
	baseLines := strings.Split(base, "\n")
	out := make([]string, 0, m.height)
	for y := 0; y < m.height; y++ {
		line := ""
		if y < len(baseLines) {
			line = baseLines[y]
		}
		line = fitLine(line, m.width)
		if y >= offY && y < offY+len(panel) {
			left := ansi.Cut(line, 0, offX)
			if w := ansi.StringWidth(left); w < offX {
				left += strings.Repeat(" ", offX-w)
			}
			right := ansi.Cut(line, offX+boxW, m.width)
			if w := ansi.StringWidth(right); w < m.width-(offX+boxW) {
				right += strings.Repeat(" ", m.width-(offX+boxW)-w)
			}
			line = left + "\x1b[0m" + panel[y-offY] + right + "\x1b[0m"
		}
		out = append(out, fitLine(line, m.width))
	}
	return strings.Join(out, "\n")
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
		case i == n+2:
			body = append(body, javaPickerRow(marker, m.t("Use a custom path…"), "", false, selected, contentW))
		case i >= n+3 && i <= n+5:
			major := []int{21, 17, 8}[i-(n+3)]
			body = append(body, javaPickerRow(marker, m.tf("Download Java %d", major), m.t("Adoptium Temurin JDK"), false, selected, contentW))
		default:
			body = append(body, javaPickerRow(marker, "", "", false, selected, contentW))
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
