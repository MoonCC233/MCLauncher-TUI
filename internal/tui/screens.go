package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MoonCC233/MCLauncher-TUI/internal/config"
	"github.com/MoonCC233/MCLauncher-TUI/internal/i18n"
	"github.com/MoonCC233/MCLauncher-TUI/internal/install"
	"github.com/MoonCC233/MCLauncher-TUI/internal/util"
)

func (m *Model) homeListSize() int {
	n := len(m.recentInstances())
	if n > 6 {
		n = 6
	}
	return n
}

func (m *Model) clampHomeCursor() {
	size := m.homeListSize()
	if m.homeCursor >= size {
		m.homeCursor = size - 1
	}
	if m.homeCursor < 0 {
		m.homeCursor = 0
	}
}

func (m *Model) homeInstance() *config.Instance {
	recent := m.recentInstances()
	if len(recent) == 0 {
		return nil
	}
	m.clampHomeCursor()
	if m.homeCursor >= len(recent) {
		return nil
	}
	return &recent[m.homeCursor]
}

func (m *Model) updateHome(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.homeCursor--
		m.clampHomeCursor()
	case "down", "j":
		m.homeCursor++
		m.clampHomeCursor()
	case "enter":
		inst := m.homeInstance()
		if inst == nil {
			m.switchScreen(ScreenVersions)
			return m, m.loadManifestCmd(false)
		}
		return m, m.startLaunch(*inst)
	case "n":
		m.switchScreen(ScreenVersions)
		return m, m.loadManifestCmd(false)
	case "a":
		m.openAccountForm()
	case "r":
		m.installed = install.InstalledSet(m.layout)
		m.setToast(m.t("Refreshed"), toastInfo)
		return m, m.loadManifestCmd(true)
	case "x":
		if m.game != nil {
			m.stopGame()
			m.screen = ScreenConsole
		}
	case "o":
		_ = util.OpenPath(m.layout.Root)
	}
	return m, nil
}

func (m *Model) updateInstances(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.instanceCursor--
		m.clampInstanceCursor()
		m.ensureInstanceVisible()
	case "down", "j":
		m.instanceCursor++
		m.clampInstanceCursor()
		m.ensureInstanceVisible()
	case "enter":
		inst := m.currentInstance()
		if inst == nil {
			m.switchScreen(ScreenVersions)
			return m, m.loadManifestCmd(false)
		}
		return m, m.startLaunch(*inst)
	case "n":
		m.switchScreen(ScreenVersions)
		return m, m.loadManifestCmd(false)
	case "e":
		inst := m.currentInstance()
		if inst != nil {
			m.openEditInstanceForm(inst)
		}
	case "d":
		inst := m.currentInstance()
		if inst == nil {
			return m, nil
		}
		if m.game != nil && m.game.InstanceID == inst.ID {
			m.alert = AlertState{Title: m.t("Instance is running"), Body: m.t("Stop the game from the Console screen before deleting this instance.")}
			m.modal = modalAlert
			return m, nil
		}
		m.confirm = ConfirmState{
			Kind:   confirmDeleteInstance,
			Title:  m.t("Delete instance?"),
			Body:   m.tf("“%s” will be removed from the launcher. Its game files in instances/%s are kept on disk.", inst.Name, inst.ID),
			Target: inst.ID,
		}
		m.modal = modalConfirm
	case "r":
		m.installed = install.InstalledSet(m.layout)
		m.clampInstanceCursor()
		m.setToast(m.t("Refreshed installed versions"), toastInfo)
	case "x":
		if m.game != nil {
			m.stopGame()
			m.screen = ScreenConsole
		}
	case "o":
		if inst := m.currentInstance(); inst != nil {
			_ = util.OpenPath(m.layout.InstanceDir(inst.ID))
		}
	}
	return m, nil
}

func (m *Model) updateVersions(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.versionCursor--
		m.clampVersionCursor()
	case "down", "j":
		m.versionCursor++
		m.clampVersionCursor()
	case "pgup":
		m.versionCursor -= m.listHeight()
		m.clampVersionCursor()
	case "pgdown":
		m.versionCursor += m.listHeight()
		m.clampVersionCursor()
	case "home":
		m.versionCursor = 0
		m.clampVersionCursor()
	case "end":
		m.versionCursor = len(m.visible) - 1
		m.clampVersionCursor()
	case "enter":
		v := m.currentVersion()
		if v == nil {
			return m, nil
		}
		m.openInstanceForm(v.ID)
	case "i":
		v := m.currentVersion()
		if v == nil {
			return m, nil
		}
		if m.installed[v.ID] {
			m.setToast(m.tf("%s is already installed", v.ID), toastInfo)
			return m, nil
		}
		return m, m.startInstall(v.ID, "", 0)
	case "t":
		m.switchScope()
	case "/":
		return m, m.startFilter()
	case "r":
		m.rebuildVersions()
		return m, m.loadManifestCmd(true)
	case "o":
		v := m.currentVersion()
		if v == nil {
			return m, nil
		}
		if !m.installed[v.ID] {
			m.setToast(m.t("Install this version first"), toastInfo)
			return m, nil
		}
		_ = util.OpenPath(m.layout.VersionDir(v.ID))
	case "x":
		if m.game != nil {
			m.stopGame()
			m.screen = ScreenConsole
		}
	}
	return m, nil
}

func (m *Model) clampVersionCursor() {
	if m.versionCursor >= len(m.visible) {
		m.versionCursor = len(m.visible) - 1
	}
	if m.versionCursor < 0 {
		m.versionCursor = 0
	}
	m.ensureVersionVisible()
}

func (m *Model) updateAccounts(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.accountCursor--
		m.clampAccountCursor()
		m.ensureAccountVisible()
	case "down", "j":
		m.accountCursor++
		m.clampAccountCursor()
		m.ensureAccountVisible()
	case "enter":
		account := m.currentAccount()
		if account == nil {
			return m, nil
		}
		m.cfg.SetActiveAccount(account.UUID)
		m.saveConfig()
		m.setToast(m.tf("Active account: %s", account.Username), toastSuccess)
	case "a", "n":
		m.openAccountForm()
	case "d":
		account := m.currentAccount()
		if account == nil {
			return m, nil
		}
		if len(m.cfg.Accounts) == 1 {
			m.alert = AlertState{Title: m.t("Keep one account"), Body: m.t("Add another account before removing the last one.")}
			m.modal = modalAlert
			return m, nil
		}
		m.confirm = ConfirmState{
			Kind:   confirmRemoveAccount,
			Title:  m.t("Remove account?"),
			Body:   m.tf("“%s” will be removed from this launcher. Existing worlds are not affected.", account.Username),
			Target: account.UUID,
		}
		m.modal = modalConfirm
	}
	return m, nil
}

func (m *Model) updateSettings(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	rows := len(m.settingsRows())
	switch msg.String() {
	case "up", "k":
		m.settingsCursor--
	case "down", "j":
		m.settingsCursor++
	case "left", "right", "-", "+":
		m.adjustSetting(msg.String())
		return m, nil
	case "enter":
		return m, m.editSetting()
	case "s", "r":
		if !m.javaScanning {
			m.javaScanning = true
			m.javaScanned = false
			m.setToast(m.t("Scanning for Java runtimes…"), toastInfo)
			return m, scanJavaCmd()
		}
	case "o":
		_ = util.OpenPath(m.layout.Root)
	}
	if m.settingsCursor < 0 {
		m.settingsCursor = 0
	}
	if m.settingsCursor >= rows {
		m.settingsCursor = rows - 1
	}
	return m, nil
}

func (m *Model) adjustSetting(key string) {
	delta := 1
	if key == "left" || key == "-" {
		delta = -1
	}
	switch m.currentSettingID() {
	case "memory":
		next := util.Clamp(m.cfg.Settings.MemoryMB+delta*512, 512, 65536)
		if next != m.cfg.Settings.MemoryMB {
			m.cfg.Settings.MemoryMB = next
			m.saveConfig()
			m.setToast(m.tf("Memory: %d MB", next), toastSuccess)
		}
	case "threads":
		next := util.Clamp(m.cfg.Settings.DownloadThreads+delta, 1, 32)
		if next != m.cfg.Settings.DownloadThreads {
			m.cfg.Settings.DownloadThreads = next
			m.saveConfig()
			m.setToast(m.tf("Download threads: %d", next), toastSuccess)
		}
	}
}

func (m *Model) editSetting() tea.Cmd {
	switch m.currentSettingID() {
	case "memory":
		m.openPrompt(promptMemory, m.t("Memory"), fmt.Sprintf("%d", m.cfg.Settings.MemoryMB), m.t("512–65536 MB · applied to the next launch"), 12)
	case "java":
		return m.openJavaPicker()
	case "jvm":
		m.openPrompt(promptJVMArgs, m.t("JVM arguments"), m.cfg.Settings.JVMArgs, m.t("Extra flags passed to Java, e.g. -XX:+UseG1GC"), 64)
	case "resolution":
		m.openPrompt(promptResolution, m.t("Window size"), fmt.Sprintf("%dx%d", m.cfg.Settings.Width, m.cfg.Settings.Height), m.t("width x height · 854x480"), 16)
	case "threads":
		m.openPrompt(promptThreads, m.t("Download threads"), fmt.Sprintf("%d", m.cfg.Settings.DownloadThreads), m.t("1–32 parallel downloads"), 4)
	case "language":
		m.openLanguagePicker()
	case "data":
		_ = util.OpenPath(m.layout.Root)
	}
	return nil
}

func (m *Model) openPrompt(kind promptKind, title, value, help string, limit int) {
	input := newTextInput("", value, 40, limit)
	input.Focus()
	m.prompt = PromptState{Kind: kind, Title: title, Help: help, Input: input}
	m.modal = modalPrompt
}

func (m *Model) openJavaPicker() tea.Cmd {
	m.javaPicker = 0
	m.modal = modalJava
	if !m.javaScanned && !m.javaScanning {
		m.javaScanning = true
		return scanJavaCmd()
	}
	return nil
}

func (m *Model) openLanguagePicker() {
	m.languagePick = 0
	if m.cfg.Settings.Language != "" {
		for i, info := range i18n.Supported {
			if string(info.Code) == m.cfg.Settings.Language {
				m.languagePick = i + 1
				break
			}
		}
	}
	m.modal = modalLanguage
}

func (m *Model) submitPrompt() (tea.Model, tea.Cmd) {
	value := strings.TrimSpace(m.prompt.Input.Value())
	switch m.prompt.Kind {
	case promptMemory:
		n, err := parseInt(value)
		if err != nil || n < 512 || n > 65536 {
			m.prompt.Input.SetValue("")
			m.setToast(m.t("Memory must be 512–65536 MB"), toastError)
			return m, nil
		}
		m.cfg.Settings.MemoryMB = n
	case promptJVMArgs:
		m.cfg.Settings.JVMArgs = value
	case promptResolution:
		w, h, ok := util.ParseWindowSize(value)
		if !ok {
			m.setToast(m.t("Use width x height, e.g. 854x480"), toastError)
			return m, nil
		}
		m.cfg.Settings.Width = w
		m.cfg.Settings.Height = h
	case promptThreads:
		n, err := parseInt(value)
		if err != nil || n < 1 || n > 32 {
			m.setToast(m.t("Threads must be 1–32"), toastError)
			return m, nil
		}
		m.cfg.Settings.DownloadThreads = n
	case promptJavaPath:
		if value == "" {
			m.cfg.Settings.JavaPath = ""
			m.saveConfig()
			m.modal = modalNone
			m.setToast(m.t("Java runtime: auto-detect"), toastSuccess)
			return m, nil
		}
		m.modal = modalNone
		return m, detectJavaCmd(value)
	}
	m.saveConfig()
	m.modal = modalNone
	m.setToast(m.t("Settings saved"), toastSuccess)
	return m, nil
}

func (m *Model) updateConsole(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	page := m.height - 8
	if page < 4 {
		page = 4
	}
	switch msg.String() {
	case "up", "k":
		m.logFollow = false
		m.logScroll++
		m.clampLogScroll()
	case "down", "j":
		if m.logScroll > 0 {
			m.logScroll--
		}
		if m.logScroll == 0 {
			m.logFollow = true
		}
	case "pgup":
		m.logFollow = false
		m.logScroll += page
		m.clampLogScroll()
	case "pgdown":
		m.logScroll -= page
		if m.logScroll <= 0 {
			m.logScroll = 0
			m.logFollow = true
		}
	case "home":
		m.logFollow = false
		m.logScroll = len(m.logs)
		m.clampLogScroll()
	case "end", "G":
		m.logFollow = true
		m.logScroll = 0
	case "x":
		m.stopGame()
	case "c":
		m.logs = nil
		m.logScroll = 0
		m.logFollow = true
		m.addLog("launcher", "console cleared")
	case "o":
		_ = util.OpenPath(m.layout.Logs)
	}
	return m, nil
}

func (m *Model) clampLogScroll() {
	if m.logScroll < 0 {
		m.logScroll = 0
	}
	if max := len(m.logs) - 1; m.logScroll > max {
		m.logScroll = max
	}
	if m.logScroll < 0 {
		m.logScroll = 0
	}
}

func (m *Model) consoleTailHeight() int {
	h := m.bodyHeight() - 4
	if h < 4 {
		h = 4
	}
	return h
}

func (m *Model) consoleLines() []LogLine {
	height := m.consoleTailHeight()
	if len(m.logs) == 0 {
		return nil
	}
	end := len(m.logs) - m.logScroll
	if end > len(m.logs) {
		end = len(m.logs)
	}
	if end < 0 {
		end = 0
	}
	start := end - height
	if start < 0 {
		start = 0
	}
	return m.logs[start:end]
}

func (m *Model) logAge(ts time.Time) string {
	return ts.Local().Format("15:04:05")
}
