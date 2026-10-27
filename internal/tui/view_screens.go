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

func (m *Model) viewHome(w, h int) []string {
	titleRight := plural(len(m.cfg.Instances), "instance", "instances")
	if len(m.installed) > 0 {
		titleRight += " · " + plural(len(m.installed), "version installed", "versions installed")
	}
	lines := []string{
		sectionHeader("Home", titleRight, w),
		"",
	}
	for _, logo := range renderLogo() {
		lines = append(lines, "  "+logo)
	}
	lines = append(lines,
		"",
		"  "+styleBold.Render("Minecraft, in your terminal."),
		"  "+styleMuted.Render("Vanilla versions · offline accounts · shared libraries and assets"),
	)
	if acc := m.activeAccount(); acc == nil {
		lines = append(lines, "", "  "+styleYellow.Render("No account selected")+styleDim.Render(" — press a to add an offline account"))
	}
	lines = append(lines, "")

	if m.game != nil {
		lines = append(lines,
			"  "+styleGreen.Render("● RUNNING")+"  "+styleBold.Render(m.game.InstanceName)+
				styleDim.Render("  "+m.game.VersionID+" · "+elapsedSince(m.game.Started)+" · pid "+fmt.Sprintf("%d", m.game.Pid)),
			"  "+styleDim.Render("logs are streaming in Console · press x to stop"),
			"",
		)
	}

	recent := m.recentInstances()
	heading := styleDim.Render("CONTINUE")
	if len(recent) > 0 {
		heading += styleDim.Render("  ·  " + recentLabel(recent))
	}
	lines = append(lines, "  "+heading)
	if len(recent) == 0 {
		lines = append(lines,
			"  "+styleMuted.Render("No instances yet."),
			"  "+styleDim.Render("Pick a version on the Versions screen to create your first instance."),
		)
	} else {
		m.clampHomeCursor()
		limit := m.homeListSize()
		for i := 0; i < limit && i < len(recent); i++ {
			lines = append(lines, m.homeRow(recent[i], i == m.homeCursor, w))
		}
	}
	lines = append(lines, "")

	action := "play"
	if inst := m.homeInstance(); inst != nil {
		action = "play " + util.Truncate(inst.Name, 22)
	}
	lines = append(lines, "  "+hints(
		keyHint("enter", action),
		keyHint("n", "new instance"),
		keyHint("a", "add account"),
		keyHint("tab", "sections"),
	))
	lines = append(lines, "", "  "+styleDim.Render("Tip: on Versions, press / to search every release back to 2011."))
	return lines
}

func recentLabel(recent []config.Instance) string {
	if len(recent) == 0 {
		return "none"
	}
	if len(recent) > 6 {
		return fmt.Sprintf("latest 6 of %d", len(recent))
	}
	return fmt.Sprintf("%d", len(recent))
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
	played := "never"
	if inst.LastPlayedAt > 0 {
		played = util.RelativeTime(inst.LastPlayedAt)
	}
	mem := "global"
	if inst.MemoryMB > 0 {
		mem = fmt.Sprintf("%d MB", inst.MemoryMB)
	}
	row := "  " + marker + running +
		fitLine(nameStyle.Render(util.Truncate(inst.Name, nameW)), nameW) + " " +
		fitLine(styleCyan.Render(util.Truncate(inst.VersionID, versionW)), versionW)
	if showPlayed {
		row += " " + fitLine(styleMuted.Render(played), playedW)
	}
	if showMem {
		row += " " + fitLine(styleDim.Render(mem), memW)
	}
	return fitLine(row, w)
}

func (m *Model) viewInstances(w, h int) []string {
	lines := []string{
		sectionHeader("Instances", plural(len(m.cfg.Instances), "instance", "instances"), w),
		"",
	}
	if len(m.cfg.Instances) == 0 {
		lines = append(lines,
			"  "+styleBold.Render("No instances yet"),
			"  "+styleMuted.Render("Open Versions with n, choose a release and press Enter."),
			"  "+styleDim.Render("The launcher installs the version, then creates an isolated game directory."),
		)
		for len(lines) < h-4 {
			lines = append(lines, "")
		}
		lines = append(lines, "", "  "+hints(keyHint("n", "browse versions"), keyHint("tab", "sections")))
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
	}
	lines = append(lines, "")
	inst := m.currentInstance()
	if inst != nil {
		lines = append(lines,
			"  "+stylePrimary.Render("▌ ")+styleBold.Render(inst.Name),
			"  "+styleMuted.Render(fmt.Sprintf("%s · %s · last played %s%s",
				inst.VersionID, memoryLabel(m, inst), util.RelativeTime(inst.LastPlayedAt), playedTotal(*inst))),
			"  "+styleDim.Render(util.ShortenPath(m.layout.InstanceDir(inst.ID), w-4)),
		)
	}
	return lines
}

func memoryLabel(m *Model, inst *config.Instance) string {
	if inst.MemoryMB > 0 {
		return fmt.Sprintf("%d MB", inst.MemoryMB)
	}
	return fmt.Sprintf("%d MB global", m.cfg.Settings.MemoryMB)
}

func playedTotal(inst config.Instance) string {
	if inst.PlaySeconds <= 0 {
		return ""
	}
	return " · " + util.HumanDuration(time.Duration(inst.PlaySeconds)*time.Second) + " played"
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
	played := "never"
	if inst.LastPlayedAt > 0 {
		played = "played " + util.RelativeTime(inst.LastPlayedAt)
	}
	mem := "global"
	if inst.MemoryMB > 0 {
		mem = fmt.Sprintf("%d MB", inst.MemoryMB)
	}
	row := "  " + marker + running +
		fitLine(nameStyle.Render(util.Truncate(inst.Name, nameW)), nameW) + " " +
		fitLine(versionStyle.Render(util.Truncate(inst.VersionID, versionW)), versionW)
	if showPlayed {
		row += " " + fitLine(styleMuted.Render(played), playedW)
	}
	if showMem {
		row += " " + fitLine(styleDim.Render(mem), memW)
	}
	return fitLine(row, w)
}

func (m *Model) viewVersions(w, h int) []string {
	source := "manifest"
	right := "loading…"
	if m.manifest != nil {
		source = m.manifest.Source
		right = fmt.Sprintf("%s · %d versions", source, len(m.visible))
		if m.manifestAt.After(time.Time{}) {
			right += " · " + util.RelativeTime(m.manifestAt.Unix())
		}
	}
	lines := []string{
		sectionHeader("Versions", right, w),
		"",
		m.filterBar(w),
		"",
	}
	if m.manifest == nil {
		if m.manifestLoading {
			lines = append(lines,
				"  "+stylePrimary.Render(spinnerFrame(m.spinner))+" "+styleMuted.Render("Fetching the Mojang version manifest…"),
				"  "+styleDim.Render("piston-meta.mojang.com · cached for offline use"),
			)
		} else {
			lines = append(lines,
				"  "+styleRed.Render("Could not load the version manifest"),
				"  "+styleDim.Render(truncateRunes(m.manifestErr, w-4)),
				"  "+styleMuted.Render("Press r to retry."),
			)
		}
		for len(lines) < h-3 {
			lines = append(lines, "")
		}
		return lines
	}
	if len(m.visible) == 0 {
		lines = append(lines,
			"  "+styleMuted.Render("No versions match this filter."),
			"  "+styleDim.Render("Press / to change the filter or t to change the type."),
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
	}
	lines = append(lines, "")
	v := m.currentVersion()
	if v != nil {
		state := styleDim.Render("not installed")
		if m.installed[v.ID] {
			state = styleGreen.Render("installed")
		}
		date := releaseDate(v.ReleaseTime)
		lines = append(lines,
			"  "+stylePrimary.Render("▌ ")+styleBold.Render(v.ID)+styleDim.Render("  "+v.Type+" · released "+date)+"  "+state,
			"  "+styleDim.Render("enter create instance · i install only · o open version folder"),
		)
	}
	return lines
}

func (m *Model) filterBar(w int) string {
	chips := []string{"all", "releases", "snapshots"}
	var parts []string
	for i, chip := range chips {
		if i == m.versionScope {
			parts = append(parts, lipgloss.NewStyle().
				Background(colPanelAlt).
				Foreground(colPrimary).
				Bold(true).
				Render(" "+chip+" "))
		} else {
			parts = append(parts, styleDim.Render(" "+chip+" "))
		}
	}
	bar := "  " + strings.Join(parts, styleBorder.Render(" "))
	if m.filterActive {
		m.search.Width = util.Clamp(w-44, 12, 40)
		bar += "  " + m.search.View()
	} else if m.filter != "" {
		bar += "  " + styleYellow.Render("/ "+m.filter) + styleDim.Render("  esc clear")
	} else {
		bar += "  " + styleDim.Render("/ filter")
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
	status := styleDim.Render("not installed")
	if m.installed[v.ID] {
		status = styleGreen.Render("installed")
	} else if v.Latest {
		status = styleYellow.Render("latest release")
	}
	row := "  " + sel + " " + installedMark + " " +
		fitLine(idStyle.Render(util.Truncate(v.ID, idW)), idW) + " " +
		fitLine(versionTypeStyle(v.Type).Render(v.Type), typeW) + " " +
		fitLine(styleDim.Render(releaseDate(v.ReleaseTime)), dateW)
	if showStatus {
		row += "  " + fitLine(status, statusW)
	}
	return fitLine(row, w)
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

func releaseDate(s string) string {
	if s == "" {
		return "unknown"
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
		sectionHeader("Accounts", "offline mode", w),
		"",
	}
	if len(m.cfg.Accounts) == 0 {
		lines = append(lines,
			"  "+styleBold.Render("No accounts yet"),
			"  "+styleMuted.Render("Offline accounts need only a username. Mojang authentication is never contacted."),
			"  "+styleDim.Render("Press a to add one."),
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
	}
	lines = append(lines, "")
	acc := m.currentAccount()
	if acc != nil {
		active := "not active"
		if acc.UUID == m.cfg.ActiveAccountUUID {
			active = "active account"
		}
		lines = append(lines,
			"  "+stylePrimary.Render("▌ ")+styleBold.Render(acc.Username)+styleDim.Render("  "+active),
			"  "+styleMuted.Render("UUID  "+acc.UUID),
			"  "+styleDim.Render("created "+util.FormatUnix(acc.CreatedAt)+" · offline player · deterministic Java UUID"),
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
	badge := styleDim.Render("offline")
	if acc.UUID == m.cfg.ActiveAccountUUID {
		badge = styleGreen.Render("active")
	}
	nameW := 20
	uuidW := w - 2 - 2 - 1 - nameW - 10 - 3
	if uuidW < 12 {
		uuidW = 12
	}
	row := "  " + marker + active + " " +
		fitLine(nameStyle.Render(util.Truncate(acc.Username, nameW)), nameW) + " " +
		fitLine(styleDim.Render(acc.UUID), uuidW) + " " +
		fitLine(badge, 8)
	return fitLine(row, w)
}

func (m *Model) viewSettings(w, h int) []string {
	lines := []string{
		sectionHeader("Settings", "saved automatically", w),
		"",
	}
	rows := m.settingsRows()
	for i, row := range rows {
		marker := "  "
		labelStyle := styleMuted
		valueStyle := styleText
		if i == m.settingsCursor {
			marker = stylePrimary.Render("▸ ")
			labelStyle = stylePrimaryB
			valueStyle = styleBold
		}
		lines = append(lines, fitLine("  "+marker+fitLine(labelStyle.Render(row.Label), 20)+"  "+valueStyle.Render(row.Value), w))
	}
	descriptions := []string{
		"Memory limit used by instances that do not set their own value.",
		"Auto-detect selects the runtime requested by the Minecraft version.",
		"Extra space-separated flags passed to the JVM.",
		"Initial Minecraft window size.",
		"Parallel downloads used while installing game files.",
		"Versions, assets, libraries and instance game directories.",
		"mclauncher is built with Go, Bubble Tea and Lip Gloss.",
	}
	if m.settingsCursor >= 0 && m.settingsCursor < len(descriptions) {
		lines = append(lines, "", "  "+styleDim.Render(descriptions[m.settingsCursor]))
	}
	lines = append(lines, "")
	javaLine := "  " + styleDim.Render("JAVA")
	if m.javaScanning {
		javaLine += "  " + stylePrimary.Render(spinnerFrame(m.spinner)) + " " + styleMuted.Render("scanning…")
	} else if len(m.javaRuntimes) > 0 {
		javaLine += "  " + styleMuted.Render(fmt.Sprintf("%s detected · press s to rescan", plural(len(m.javaRuntimes), "runtime", "runtimes")))
	} else {
		javaLine += "  " + styleMuted.Render("no runtime detected · press s to scan")
	}
	lines = append(lines, fitLine(javaLine, w))
	lines = append(lines, "")
	lines = append(lines, "  "+styleDim.Render("Instances can override memory. JVM flags and window size apply to every launch."))
	return lines
}

func (m *Model) viewConsole(w, h int) []string {
	right := "idle"
	if m.game != nil {
		right = styleGreen.Render("● running") + styleDim.Render(" · "+m.game.VersionID+" · "+elapsedSince(m.game.Started))
	}
	lines := []string{
		sectionHeader("Console", right, w),
		"",
	}
	tail := m.consoleLines()
	if len(tail) == 0 {
		lines = append(lines,
			"  "+styleMuted.Render("No output yet."),
			"  "+styleDim.Render("Install or launch an instance and its output will appear here."),
			"  "+styleDim.Render("This console also collects launcher, downloader and Java messages."),
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
	state := "following tail"
	if !m.logFollow {
		state = fmt.Sprintf("scrolled %d lines · G to follow", m.logScroll)
	}
	left := "  " + styleDim.Render(fmt.Sprintf("%d lines", len(m.logs))) + " · " + styleMuted.Render(state)
	if m.game != nil {
		left += " · " + stylePrimary.Render("x stop game")
	}
	lines = append(lines, fitLine(left, w))
	return lines
}

func (m *Model) consoleRow(entry LogLine, w int) string {
	source := entry.Source
	color := colCyan
	label := "game"
	switch source {
	case "launcher":
		color = colPrimary
		label = "launch"
	case "stderr":
		color = colRed
		label = "stderr"
	case "error":
		color = colRed
		label = "error"
	case "system":
		color = colPurple
		label = "system"
	}
	tag := lipgloss.NewStyle().Foreground(color).Render(fitLine(label, 7))
	textW := w - 2 - 8 - 2 - 7 - 2
	if textW < 8 {
		textW = 8
	}
	text := util.Truncate(entry.Text, textW)
	return fitLine("  "+styleDim.Render(entry.At.Format("15:04:05"))+"  "+tag+"  "+styleText.Render(text), w)
}
