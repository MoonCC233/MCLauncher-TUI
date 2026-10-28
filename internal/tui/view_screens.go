package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/MoonCC233/MCLauncher-TUI/internal/config"
	"github.com/MoonCC233/MCLauncher-TUI/internal/mojang"
	"github.com/MoonCC233/MCLauncher-TUI/internal/util"
)

func (m *Model) joinColumns(parts ...string) string {
	if m.rtl() {
		reversed := make([]string, len(parts))
		for i := range parts {
			reversed[len(parts)-1-i] = parts[i]
		}
		parts = reversed
	}
	return strings.Join(parts, " ")
}

func (m *Model) rowHit(screen Screen, index, line, width int) {
	m.addHit(m.contentX(), bodyTop+line, width, 1, hitRegion{Kind: hitRow, Screen: screen, Index: index})
}

// rowLine draws a list row. Markers and columns are mirrored in RTL layouts.
func (m *Model) rowLine(marker, pip string, cols []string, width int) string {
	body := m.joinColumns(cols...)
	if m.rtl() {
		return fitLine(body+" "+pip+marker, width)
	}
	return fitLine("  "+marker+pip+body, width)
}

func (m *Model) viewHome(w, h int) []string {
	titleRight := m.count(len(m.cfg.Instances), "instance", "instances")
	if len(m.installed) > 0 {
		titleRight += " · " + m.count(len(m.installed), "version installed", "versions installed")
	}
	lines := []string{
		m.sectionHeader(m.t("Home"), titleRight, w),
		"",
	}
	for _, logo := range renderLogo() {
		lines = append(lines, "  "+logo)
	}
	lines = append(lines,
		"",
		"  "+styleBold.Render(m.t("Minecraft, in your terminal.")),
		"  "+styleMuted.Render(m.t("Vanilla versions · offline accounts · shared libraries and assets")),
	)
	if acc := m.activeAccount(); acc == nil {
		lines = append(lines, "", "  "+styleYellow.Render(m.t("No account selected"))+styleDim.Render(" — "+m.t("press a to add an offline account")))
	}
	lines = append(lines, "")

	if m.game != nil {
		lines = append(lines,
			"  "+styleGreen.Render("● "+m.t("RUNNING"))+"  "+styleBold.Render(m.game.InstanceName)+
				styleDim.Render("  "+m.game.VersionID+" · "+elapsedSince(m.game.Started)+" · pid "+fmt.Sprintf("%d", m.game.Pid)),
			"  "+styleDim.Render(m.t("logs are streaming in Console · press x to stop")),
			"",
		)
	}

	recent := m.recentInstances()
	heading := styleDim.Render(m.t("CONTINUE"))
	if len(recent) > 0 {
		heading += styleDim.Render("  ·  " + m.recentLabel(recent))
	}
	lines = append(lines, "  "+heading)
	if len(recent) == 0 {
		lines = append(lines,
			"  "+styleMuted.Render(m.t("No instances yet.")),
			"  "+styleDim.Render(m.t("Pick a version on the Versions screen to create your first instance.")),
		)
	} else {
		m.clampHomeCursor()
		limit := m.homeListSize()
		for i := 0; i < limit && i < len(recent); i++ {
			lines = append(lines, m.homeRow(recent[i], i == m.homeCursor, w))
			m.rowHit(ScreenHome, i, len(lines)-1, w)
		}
	}
	lines = append(lines, "")

	action := m.t("play")
	if inst := m.homeInstance(); inst != nil {
		action = m.tf("play %s", util.Truncate(inst.Name, 22))
	}
	hintY := bodyTop + len(lines)
	lines = append(lines, "  "+m.renderHintsWidth(m.contentX()+2, hintY, w-4,
		hint("enter", action),
		hint("n", m.t("new instance")),
		hint("a", m.t("add account")),
		hint("tab", m.t("sections")),
	))
	lines = append(lines, "", "  "+styleDim.Render(m.t("Tip: on Versions, press / to search every release back to 2011.")))
	return lines
}

func (m *Model) recentLabel(recent []config.Instance) string {
	if len(recent) == 0 {
		return m.t("none")
	}
	if len(recent) > 6 {
		return m.tf("latest 6 of %d", len(recent))
	}
	return m.count(len(recent), "instance", "instances")
}

func (m *Model) homeRow(inst config.Instance, selected bool, w int) string {
	marker := "  "
	nameStyle := styleText
	if selected {
		marker = stylePrimary.Render("▸ ")
		nameStyle = stylePrimaryB
	}
	running := "  "
	if m.game != nil && m.game.InstanceID == inst.ID {
		running = styleGreen.Render("● ")
	}
	const versionW, playedW, memW = 12, 14, 8
	showPlayed := w >= 54
	showMem := w >= 70
	nameW := w - 19
	if showPlayed {
		nameW -= playedW + 1
	}
	if showMem {
		nameW -= memW + 1
	}
	if nameW > 24 {
		nameW = 24
	}
	if nameW < 8 {
		nameW = 8
	}
	played := m.t("never")
	if inst.LastPlayedAt > 0 {
		played = m.relTime(inst.LastPlayedAt)
	}
	mem := m.t("global")
	if inst.MemoryMB > 0 {
		mem = fmt.Sprintf("%d MB", inst.MemoryMB)
	}
	cols := []string{
		fitLine(nameStyle.Render(util.Truncate(inst.Name, nameW)), nameW),
		fitLine(styleCyan.Render(util.Truncate(inst.VersionID, versionW)), versionW),
	}
	if showPlayed {
		cols = append(cols, fitLine(styleMuted.Render(played), playedW))
	}
	if showMem {
		cols = append(cols, fitLine(styleDim.Render(mem), memW))
	}
	return m.rowLine(marker, running, cols, w)
}

func (m *Model) viewInstances(w, h int) []string {
	lines := []string{
		m.sectionHeader(m.t("Instances"), m.count(len(m.cfg.Instances), "instance", "instances"), w),
		"",
	}
	if len(m.cfg.Instances) == 0 {
		lines = append(lines,
			"  "+styleBold.Render(m.t("No instances yet")),
			"  "+styleMuted.Render(m.t("Open Versions with n, choose a release and press Enter.")),
			"  "+styleDim.Render(m.t("The launcher installs the version, then creates an isolated game directory.")),
		)
		for len(lines) < h-4 {
			lines = append(lines, "")
		}
		hintY := bodyTop + len(lines)
		lines = append(lines, "", "  "+m.renderHintsWidth(m.contentX()+2, hintY, w-4,
			hint("n", m.t("browse versions")),
			hint("tab", m.t("sections")),
		))
		return lines
	}
	m.clampInstanceCursor()
	m.ensureInstanceVisible()
	start := m.instanceOffset
	end := start + m.instanceListHeight()
	if end > len(m.cfg.Instances) {
		end = len(m.cfg.Instances)
	}
	for i := start; i < end; i++ {
		lines = append(lines, m.instanceRow(m.cfg.Instances[i], i == m.instanceCursor, w))
		m.rowHit(ScreenInstances, i, len(lines)-1, w)
	}
	lines = append(lines, "")
	inst := m.currentInstance()
	if inst != nil {
		lines = append(lines,
			"  "+stylePrimary.Render("▌ ")+styleBold.Render(inst.Name),
			"  "+styleMuted.Render(m.tf("%s · %s · %s%s",
				inst.VersionID, m.memoryLabel(inst), m.tf("last played %s", m.relTime(inst.LastPlayedAt)), m.playedTotal(*inst))),
			"  "+styleDim.Render(util.ShortenPath(m.layout.InstanceDir(inst.ID), w-4)),
		)
	}
	return lines
}

func (m *Model) memoryLabel(inst *config.Instance) string {
	if inst.MemoryMB > 0 {
		return fmt.Sprintf("%d MB", inst.MemoryMB)
	}
	return m.tf("%d MB global", m.cfg.Settings.MemoryMB)
}

func (m *Model) playedTotal(inst config.Instance) string {
	if inst.PlaySeconds <= 0 {
		return ""
	}
	return " · " + m.tf("%s played", util.HumanDuration(time.Duration(inst.PlaySeconds)*time.Second))
}

func (m *Model) instanceRow(inst config.Instance, selected bool, w int) string {
	marker := "  "
	nameStyle := styleText
	versionStyle := styleCyan
	if selected {
		marker = stylePrimary.Render("▸ ")
		nameStyle = stylePrimaryB
		versionStyle = stylePrimary
	}
	running := "  "
	if m.game != nil && m.game.InstanceID == inst.ID {
		running = styleGreen.Render("● ")
	}
	const versionW, playedW, memW = 12, 16, 10
	showPlayed := w >= 56
	showMem := w >= 72
	nameW := w - 19
	if showPlayed {
		nameW -= playedW + 1
	}
	if showMem {
		nameW -= memW + 1
	}
	if nameW > 26 {
		nameW = 26
	}
	if nameW < 8 {
		nameW = 8
	}
	played := m.t("never")
	if inst.LastPlayedAt > 0 {
		played = m.tf("played %s", m.relTime(inst.LastPlayedAt))
	}
	mem := m.t("global")
	if inst.MemoryMB > 0 {
		mem = fmt.Sprintf("%d MB", inst.MemoryMB)
	}
	cols := []string{
		fitLine(nameStyle.Render(util.Truncate(inst.Name, nameW)), nameW),
		fitLine(versionStyle.Render(util.Truncate(inst.VersionID, versionW)), versionW),
	}
	if showPlayed {
		cols = append(cols, fitLine(styleMuted.Render(played), playedW))
	}
	if showMem {
		cols = append(cols, fitLine(styleDim.Render(mem), memW))
	}
	return m.rowLine(marker, running, cols, w)
}

func (m *Model) viewVersions(w, h int) []string {
	source := m.t("manifest")
	right := m.t("loading…")
	if m.manifest != nil {
		source = m.manifest.Source
		right = m.tf("%s · %s", source, m.count(len(m.visible), "version", "versions"))
		if m.manifestAt.After(time.Time{}) {
			right += " · " + m.relTime(m.manifestAt.Unix())
		}
	}
	lines := []string{
		m.sectionHeader(m.t("Versions"), right, w),
		"",
		m.filterBar(w),
		"",
	}
	if m.manifest == nil {
		if m.manifestLoading {
			lines = append(lines,
				"  "+stylePrimary.Render(spinnerFrame(m.spinner))+" "+styleMuted.Render(m.t("Fetching the Mojang version manifest…")),
				"  "+styleDim.Render(m.t("piston-meta.mojang.com · cached for offline use")),
			)
		} else {
			lines = append(lines,
				"  "+styleRed.Render(m.t("Could not load the version manifest")),
				"  "+styleDim.Render(truncateRunes(m.manifestErr, w-4)),
				"  "+styleMuted.Render(m.t("Press r to retry.")),
			)
		}
		for len(lines) < h-3 {
			lines = append(lines, "")
		}
		return lines
	}
	if len(m.visible) == 0 {
		lines = append(lines,
			"  "+styleMuted.Render(m.t("No versions match this filter.")),
			"  "+styleDim.Render(m.t("Press / to change the filter or t to change the type.")),
		)
		for len(lines) < h-3 {
			lines = append(lines, "")
		}
		return lines
	}

	m.clampVersionCursor()
	m.ensureVersionVisible()
	listH := m.listHeight()
	start := m.versionOffset
	end := start + listH
	if end > len(m.visible) {
		end = len(m.visible)
	}
	for i := start; i < end; i++ {
		lines = append(lines, m.versionRow(m.visible[i], i == m.versionCursor, w))
		m.rowHit(ScreenVersions, i, len(lines)-1, w)
	}
	lines = append(lines, "")
	v := m.currentVersion()
	if v != nil {
		state := styleDim.Render(m.t("not installed"))
		if m.installed[v.ID] {
			state = styleGreen.Render(m.t("installed"))
		}
		date := m.releaseDate(v.ReleaseTime)
		lines = append(lines,
			"  "+stylePrimary.Render("▌ ")+styleBold.Render(v.ID)+styleDim.Render("  "+m.versionTypeLabel(v.Type)+" · "+m.tf("released %s", date))+"  "+state,
			"  "+styleDim.Render(m.t("enter create instance · i install only · o open version folder")),
		)
	}
	return lines
}

func (m *Model) filterBar(w int) string {
	chips := []string{m.t("all"), m.t("releases"), m.t("snapshots")}
	order := []int{0, 1, 2}
	if m.rtl() {
		order = []int{2, 1, 0}
	}
	var parts []string
	x := m.contentX() + 2
	for _, idx := range order {
		chip := chips[idx]
		var rendered string
		if idx == m.versionScope {
			rendered = lipgloss.NewStyle().
				Background(colPanelAlt).
				Foreground(colPrimary).
				Bold(true).
				Render(" " + chip + " ")
		} else {
			rendered = styleDim.Render(" " + chip + " ")
		}
		m.addHit(x, bodyTop+2, ansi.StringWidth(rendered), 1, hitRegion{Kind: hitChip, Scope: idx})
		x += ansi.StringWidth(rendered) + 1
		parts = append(parts, rendered)
	}
	bar := "  " + strings.Join(parts, styleBorder.Render(" "))
	if m.filterActive {
		m.search.Width = util.Clamp(w-44, 12, 40)
		bar += "  " + m.search.View()
	} else if m.filter != "" {
		bar += "  " + styleYellow.Render("/ "+m.filter) + styleDim.Render("  "+m.t("esc clear"))
	} else {
		bar += "  " + styleDim.Render("/ "+m.t("filter"))
	}
	if m.manifestErr != "" {
		bar += "  " + styleRed.Render("· "+util.Truncate(m.manifestErr, max(8, w-ansi.StringWidth(bar)-8)))
	}
	return fitLine(bar, w)
}

func (m *Model) versionRow(v mojang.VersionSummary, selected bool, w int) string {
	sel := " "
	if selected {
		sel = stylePrimary.Render("▸")
	}
	installedMark := " "
	if m.installed[v.ID] {
		installedMark = styleGreen.Render("●")
	}
	idStyle := styleText
	if selected {
		idStyle = stylePrimaryB
	}
	const typeW, dateW, statusW = 10, 10, 13
	showStatus := w >= 59
	idW := w - 43
	if !showStatus {
		idW = w - 28
	}
	if idW > 22 {
		idW = 22
	}
	if idW < 8 {
		idW = 8
	}
	status := styleDim.Render(m.t("not installed"))
	if m.installed[v.ID] {
		status = styleGreen.Render(m.t("installed"))
	} else if v.Latest {
		status = styleYellow.Render(m.t("latest release"))
	}
	cols := []string{
		fitLine(idStyle.Render(util.Truncate(v.ID, idW)), idW),
		fitLine(versionTypeStyle(v.Type).Render(util.Truncate(m.versionTypeLabel(v.Type), typeW)), typeW),
		fitLine(styleDim.Render(m.releaseDate(v.ReleaseTime)), dateW),
	}
	if showStatus {
		cols = append(cols, fitLine(status, statusW))
	}
	return m.rowLine(sel+" ", installedMark+" ", cols, w)
}

func (m *Model) versionTypeLabel(t string) string {
	switch t {
	case "release":
		return m.t("release")
	case "snapshot":
		return m.t("snapshot")
	case "old_beta":
		return m.t("old beta")
	case "old_alpha":
		return m.t("old alpha")
	}
	return t
}

func versionTypeStyle(t string) lipgloss.Style {
	color := colDim
	switch t {
	case "release":
		color = colGreen
	case "snapshot":
		color = colYellow
	case "old_beta", "old_alpha":
		color = colMuted
	}
	return lipgloss.NewStyle().Foreground(color)
}

func (m *Model) releaseDate(s string) string {
	if s == "" {
		return m.t("unknown")
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.Format("2006-01-02")
	}
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

func (m *Model) viewAccounts(w, h int) []string {
	lines := []string{
		m.sectionHeader(m.t("Accounts"), m.t("offline mode"), w),
		"",
	}
	if len(m.cfg.Accounts) == 0 {
		lines = append(lines,
			"  "+styleBold.Render(m.t("No accounts yet")),
			"  "+styleMuted.Render(m.t("Offline accounts need only a username. Mojang authentication is never contacted.")),
			"  "+styleDim.Render(m.t("Press a to add one.")),
		)
		for len(lines) < h-2 {
			lines = append(lines, "")
		}
		return lines
	}
	m.clampAccountCursor()
	m.ensureAccountVisible()
	start := m.accountOffset
	end := start + m.accountListHeight()
	if end > len(m.cfg.Accounts) {
		end = len(m.cfg.Accounts)
	}
	for i := start; i < end; i++ {
		lines = append(lines, m.accountRow(m.cfg.Accounts[i], i == m.accountCursor, w))
		m.rowHit(ScreenAccounts, i, len(lines)-1, w)
	}
	lines = append(lines, "")
	acc := m.currentAccount()
	if acc != nil {
		active := m.t("not active")
		if acc.UUID == m.cfg.ActiveAccountUUID {
			active = m.t("active account")
		}
		created := util.FormatUnix(acc.CreatedAt)
		if acc.CreatedAt <= 0 {
			created = m.t("never")
		}
		lines = append(lines,
			"  "+stylePrimary.Render("▌ ")+styleBold.Render(acc.Username)+styleDim.Render("  "+active),
			"  "+styleMuted.Render("UUID  "+acc.UUID),
			"  "+styleDim.Render(m.tf("created %s · offline player · deterministic Java UUID", created)),
		)
	}
	return lines
}

func (m *Model) accountRow(acc config.Account, selected bool, w int) string {
	marker := "  "
	nameStyle := styleText
	if selected {
		marker = stylePrimary.Render("▸ ")
		nameStyle = stylePrimaryB
	}
	active := " "
	if acc.UUID == m.cfg.ActiveAccountUUID {
		active = styleGreen.Render("●")
	}
	badge := styleDim.Render(m.t("offline"))
	if acc.UUID == m.cfg.ActiveAccountUUID {
		badge = styleGreen.Render(m.t("active"))
	}
	nameW := 20
	uuidW := w - 2 - 2 - 1 - nameW - 10 - 3
	if uuidW < 12 {
		uuidW = 12
	}
	cols := []string{
		fitLine(nameStyle.Render(util.Truncate(acc.Username, nameW)), nameW),
		fitLine(styleDim.Render(acc.UUID), uuidW),
		fitLine(badge, 8),
	}
	return m.rowLine(marker, active+" ", cols, w)
}

func (m *Model) settingsRowLine(marker, label, value string, labelW, width int) string {
	label = fitLine(label, labelW)
	if m.rtl() {
		right := label + " " + marker
		gap := width - 2 - ansi.StringWidth(right) - ansi.StringWidth(value)
		if gap < 1 {
			value = ansi.Truncate(value, max(4, width-2-ansi.StringWidth(right)-1), "…")
			gap = width - 2 - ansi.StringWidth(right) - ansi.StringWidth(value)
		}
		if gap < 0 {
			gap = 0
		}
		return "  " + value + strings.Repeat(" ", gap) + right
	}
	return fitLine("  "+marker+label+"  "+value, width)
}

func (m *Model) viewSettings(w, h int) []string {
	lines := []string{
		m.sectionHeader(m.t("Settings"), m.t("saved automatically"), w),
		"",
	}
	rows := m.settingsRows()
	labelW := 20
	for _, row := range rows {
		if width := ansi.StringWidth(row.Label); width > labelW {
			labelW = width
		}
	}
	if labelW > 24 {
		labelW = 24
	}
	if labelW > w-24 {
		labelW = max(12, w-24)
	}
	for i, row := range rows {
		marker := "  "
		labelStyle := styleMuted
		valueStyle := styleText
		if i == m.settingsCursor {
			marker = stylePrimary.Render("▸ ")
			labelStyle = stylePrimaryB
			valueStyle = styleBold
		}
		lines = append(lines, m.settingsRowLine(marker, labelStyle.Render(row.Label), valueStyle.Render(row.Value), labelW, w))
		m.rowHit(ScreenSettings, i, len(lines)-1, w)
	}
	if m.settingsCursor >= 0 && m.settingsCursor < len(rows) && rows[m.settingsCursor].Description != "" {
		lines = append(lines, "")
		for _, line := range wrapPlain(rows[m.settingsCursor].Description, w-4) {
			lines = append(lines, "  "+styleDim.Render(line))
		}
	}
	lines = append(lines, "")
	javaLine := "  " + styleDim.Render(m.t("JAVA"))
	if m.javaScanning {
		javaLine += "  " + stylePrimary.Render(spinnerFrame(m.spinner)) + " " + styleMuted.Render(m.t("scanning…"))
	} else if len(m.javaRuntimes) > 0 {
		javaLine += "  " + styleMuted.Render(m.tf("%s detected · press s to rescan", m.count(len(m.javaRuntimes), "runtime", "runtimes")))
	} else {
		javaLine += "  " + styleMuted.Render(m.t("no runtime detected · press s to scan"))
	}
	lines = append(lines, fitLine(javaLine, w))
	lines = append(lines, "")
	lines = append(lines, "  "+styleDim.Render(m.t("Instances can override memory. JVM flags and window size apply to every launch.")))
	return lines
}

func (m *Model) viewConsole(w, h int) []string {
	right := m.t("idle")
	if m.game != nil {
		right = styleGreen.Render("● "+m.t("running")) + styleDim.Render(" · "+m.game.VersionID+" · "+elapsedSince(m.game.Started))
	}
	lines := []string{
		m.sectionHeader(m.t("Console"), right, w),
		"",
	}
	tail := m.consoleLines()
	if len(tail) == 0 {
		lines = append(lines,
			"  "+styleMuted.Render(m.t("No output yet.")),
			"  "+styleDim.Render(m.t("Install or launch an instance and its output will appear here.")),
			"  "+styleDim.Render(m.t("This console also collects launcher, downloader and Java messages.")),
		)
	} else {
		for _, entry := range tail {
			lines = append(lines, m.consoleRow(entry, w))
		}
	}
	for len(lines) < h-2 {
		lines = append(lines, "")
	}
	lines = append(lines, "")
	state := m.t("following tail")
	if !m.logFollow {
		state = m.tf("scrolled %d lines · G to follow", m.logScroll)
	}
	left := "  " + styleDim.Render(m.count(len(m.logs), "line", "lines")) + " · " + styleMuted.Render(state)
	if m.game != nil {
		left += " · " + stylePrimary.Render(m.t("x stop game"))
	}
	lines = append(lines, fitLine(left, w))
	return lines
}

func (m *Model) consoleRow(entry LogLine, w int) string {
	source := entry.Source
	color := colCyan
	label := m.t("game")
	switch source {
	case "launcher":
		color = colPrimary
		label = m.t("launch")
	case "stderr":
		color = colRed
		label = m.t("stderr")
	case "error":
		color = colRed
		label = m.t("error")
	case "system":
		color = colPurple
		label = m.t("system")
	}
	tag := lipgloss.NewStyle().Foreground(color).Render(fitLine(label, 7))
	textW := w - 2 - 8 - 2 - 7 - 2
	if textW < 8 {
		textW = 8
	}
	text := util.Truncate(entry.Text, textW)
	if m.rtl() {
		return fitLine("  "+styleText.Render(text)+"  "+tag+"  "+styleDim.Render(entry.At.Format("15:04:05"))+"  ", w)
	}
	return fitLine("  "+styleDim.Render(entry.At.Format("15:04:05"))+"  "+tag+"  "+styleText.Render(text), w)
}
