package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MoonCC233/MCLauncher-TUI/internal/i18n"

	"github.com/MoonCC233/MCLauncher-TUI/internal/util"
)

func (m *Model) updateModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.modal {
	case modalHelp:
		return m.updateHelp(msg)
	case modalForm:
		return m.updateForm(msg)
	case modalConfirm:
		return m.updateConfirm(msg)
	case modalPrompt:
		return m.updatePrompt(msg)
	case modalJava:
		return m.updateJavaPicker(msg)
	case modalLanguage:
		return m.updateLanguagePicker(msg)
	case modalAccountAdd:
		return m.updateAccountType(msg)
	case modalMicrosoft:
		return m.updateMicrosoft(msg)
	case modalResolution:
		return m.updateResolutionPicker(msg)
	case modalTheme:
		return m.updateThemePicker(msg)
	case modalTitleAnim:
		return m.updateTitleAnimPicker(msg)
	case modalTitleFont:
		return m.updateTitleFontPicker(msg)
	case modalLoader:
		return m.updateLoaderPicker(msg)

	case modalPacks:
		return m.updatePacks(msg)
	case modalPackSearch:
		return m.updatePackSearch(msg)
	case modalPackVersions:
		return m.updatePackVersions(msg)
	case modalProgress:
		switch msg.String() {
		case "esc":
			m.cancelTask()
		case "b":
			m.backgroundTask(false)
		case "d":
			m.backgroundTask(true)
		}
		return m, nil
	case modalAlert:
		switch msg.String() {
		case "enter", "esc", " ", "q":
			m.modal = modalNone
		}
		return m, nil
	}
	m.modal = modalNone
	return m, nil
}

func (m *Model) updateHelp(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "enter", "?":
		m.modal = modalNone
	case "up", "k":
		m.helpScroll--
	case "down", "j":
		m.helpScroll++
	case "pgup":
		m.helpScroll -= 10
	case "pgdown":
		m.helpScroll += 10
	}
	m.clampHelpScroll()
	return m, nil
}

func (m *Model) updateForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.modal = modalNone
		return m, nil
	case "tab", "down":
		return m, m.focusFormRow(1)
	case "shift+tab", "up":
		return m, m.focusFormRow(-1)
	case "enter":
		// The loader row is an entry point into the loader picker, not a value to
		// cycle in place.
		if m.formLoaderFocus() {
			return m.openLoaderForForm()
		}
		return m.submitForm()
	}
	if len(m.form.Fields) == 0 || m.formLoaderFocus() {
		return m, nil
	}
	var cmd tea.Cmd
	m.form.Fields[m.form.Focus], cmd = m.form.Fields[m.form.Focus].Update(msg)
	m.form.Err = ""
	return m, cmd
}

func (m *Model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "y", "Y":
		switch m.confirm.Kind {
		case confirmDeleteInstance:
			if inst := m.cfg.FindInstance(m.confirm.Target); inst != nil {
				name := inst.Name
				if err := m.cfg.RemoveInstance(inst); err != nil {
					m.alert = AlertState{Title: m.t("Could not delete instance"), Body: err.Error()}
					m.modal = modalAlert
					return m, nil
				}
				m.clampInstanceCursor()
				m.clampHomeCursor()
				m.ensureInstanceVisible()
				m.setToast(m.tf("Deleted %s", name), toastSuccess)
				m.addLog("launcher", "deleted instance "+name)
			}
		case confirmRemoveAccount:
			target := m.confirm.Target
			if err := m.cfg.RemoveAccount(target); err != nil {
				m.alert = AlertState{Title: m.t("Could not remove account"), Body: err.Error()}
				m.modal = modalAlert
				return m, nil
			}
			m.clampAccountCursor()
			m.setToast(m.t("Account removed"), toastSuccess)
			m.addLog("launcher", "removed account")
		case confirmUninstallVersion:
			target := m.confirm.Target
			if err := m.uninstallVersion(target); err != nil {
				m.alert = AlertState{Title: m.t("Could not uninstall version"), Body: err.Error()}
				m.modal = modalAlert
				return m, nil
			}
			m.refreshInstalled()
			m.setToast(m.tf("Uninstalled %s", target), toastSuccess)

		case confirmInstallUpdate:
			m.modal = modalNone
			return m.startUpdate()
		case confirmRemovePack:
			target := m.confirm.Target
			if err := m.removePack(target); err != nil {
				m.alert = AlertState{Title: m.t("Resource packs"), Body: err.Error()}
				m.modal = modalAlert
				return m, nil
			}
			m.modal = modalPacks
			return m, nil
		}
		m.modal = modalNone
	case "esc", "n", "N":
		m.modal = modalNone
	}
	return m, nil
}

func (m *Model) updatePrompt(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		switch m.prompt.Kind {
		case promptOptiFineJar:
			m.modal = modalLoader
		default:
			m.modal = modalNone
		}
		return m, nil
	case "enter":
		return m.submitPrompt()
	}
	var cmd tea.Cmd
	m.prompt.Input, cmd = m.prompt.Input.Update(msg)
	m.prompt.Err = ""
	return m, cmd
}

func (m *Model) javaPickerCount() int {
	return len(m.javaRuntimes) + 6
}

func (m *Model) updateJavaPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.modal = modalNone
		return m, nil
	case "up", "k":
		m.javaPicker--
	case "down", "j":
		m.javaPicker++
	case "r":
		if !m.javaScanning {
			m.javaScanning = true
			return m, scanJavaCmd(m.layout.Java)
		}
		return m, nil
	case "enter":
		return m.chooseJava()
	}
	if m.javaPicker < 0 {
		m.javaPicker = 0
	}
	if m.javaPicker >= m.javaPickerCount() {
		m.javaPicker = m.javaPickerCount() - 1
	}
	return m, nil
}

func (m *Model) chooseJava() (tea.Model, tea.Cmd) {
	idx := m.javaPicker
	if idx < 0 {
		idx = 0
	}
	if idx == 0 {
		m.cfg.Settings.JavaPath = ""
		m.saveConfig()
		m.modal = modalNone
		m.setToast(m.t("Java runtime: auto-detect"), toastSuccess)
		return m, nil
	}
	n := len(m.javaRuntimes)
	if idx >= 1 && idx <= n {
		rt := m.javaRuntimes[idx-1]
		m.cfg.Settings.JavaPath = rt.Path
		m.saveConfig()
		m.modal = modalNone
		m.setToast(m.tf("Java runtime: %s", rt.Short()), toastSuccess)
		m.addLog("launcher", "java runtime: "+rt.Path)
		return m, nil
	}
	if idx == n+1 {
		if !m.javaScanning {
			m.javaScanning = true
			return m, scanJavaCmd(m.layout.Java)
		}
		return m, nil
	}
	if idx == n+2 {
		m.openPrompt(promptJavaPath, m.t("Java executable"), m.cfg.Settings.JavaPath, m.t("Full path to java.exe or a JDK folder"), 260)
		return m, nil
	}
	if idx >= n+3 && idx <= n+5 {
		major := []int{21, 17, 8}[idx-(n+3)]
		m.modal = modalNone
		return m.startJavaDownload(major)
	}
	return m, nil
}

func (m *Model) updateLanguagePicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.modal = modalNone
		return m, nil
	case "up", "k":
		m.languagePick--
	case "down", "j":
		m.languagePick++
	case "enter":
		return m.chooseLanguage()
	}
	m.languagePick = clampInt(m.languagePick, 0, m.languagePickerCount()-1)
	return m, nil
}

func (m *Model) chooseLanguage() (tea.Model, tea.Cmd) {
	idx := m.languagePick
	if idx <= 0 {
		m.useSystemLanguage()
	} else if idx <= len(i18n.Supported) {
		m.setLanguage(i18n.Supported[idx-1].Code)
	}
	m.modal = modalNone
	return m, nil
}

func (m *Model) alertBodyLines(width int) []string {
	lines := wrapPlain(m.alert.Body, width)
	if len(lines) == 0 {
		lines = []string{""}
	}
	return lines
}

func (m *Model) helpContent(width int) []string {
	type entry struct{ key, desc string }
	sections := []struct {
		title   string
		entries []entry
	}{
		{m.t("Sections"), []entry{
			{"tab / shift+tab", m.t("next / previous section")},
			{"1 … 7", m.t("jump directly to a section")},
			{"esc", m.t("back to Home")},
			{"ctrl+c", m.t("quit launcher")},
		}},
		{m.t("Navigation"), []entry{
			{"↑ ↓ / j k", m.t("move the cursor")},
			{"pgup / pgdown", m.t("jump by a page")},
			{"home / end", m.t("first / last item")},
			{"enter", m.t("open, play or edit")},
		}},
		{m.t("Instances"), []entry{
			{"n", m.t("create from the Versions screen")},
			{"e", m.t("rename / change memory")},
			{"p", m.t("open resources & mods")},
			{"d", m.t("delete instance (files are kept)")},
			{"o", m.t("open the instance folder")},
			{"m", m.t("manage mods for the selected instance")},
			{"x", m.t("stop the running game")},
		}},
		{m.t("Versions"), []entry{
			{"/", m.t("filter by version id")},
			{"t", m.t("cycle version filters")},
			{"enter", m.t("install if needed, then create instance")},
			{"i", m.t("install without creating an instance")},

			{"d", m.t("uninstall installed version")},
			{"r", m.t("refresh the Mojang manifest")},
		}},

		{m.t("Accounts & settings"), []entry{
			{"a", m.t("add account")},
			{"enter", m.t("set the active account")},
			{"s", m.t("rescan Java runtimes")},
			{"← → / - +", m.t("adjust memory or download threads")},
		}},
		{m.t("Console"), []entry{
			{"↑ ↓ / pgup pgdown", m.t("scroll through game output")},
			{"G / end", m.t("follow the tail again")},
			{"x", m.t("stop the running game")},
			{"c", m.t("clear the console")},
			{"o", m.t("open the logs folder")},
		}},
		{m.t("Mouse"), []entry{
			{m.t("click"), m.t("select ; click again to open")},
			{m.t("wheel"), m.t("scroll lists, help and console")},
			{"?", m.t("open this help")},
		}},
	}
	out := []string{styleBold.Render(m.t("Keyboard shortcuts")), styleBorder.Render(strings.Repeat("─", width)), ""}
	for _, section := range sections {
		out = append(out, stylePrimaryB.Render(section.title))
		for _, e := range section.entries {
			key := fitLine(styleYellow.Render(e.key), 18)
			out = append(out, "  "+key+styleMuted.Render(e.desc))
		}
		out = append(out, "")
	}
	out = append(out, styleDim.Render(m.t("Offline mode: launcher exits without stopping a running game.")))
	return out
}

func (m *Model) progressPercent() int {
	if m.task == nil || m.task.Total <= 0 {
		return 0
	}
	p := int(float64(m.task.Done) / float64(m.task.Total) * 100)
	if p < 0 {
		p = 0
	}
	if p > 100 {
		p = 100
	}
	return p
}

func (m *Model) progressBody(width int) []string {
	task := m.task
	if task == nil {
		return []string{m.t("Nothing is running.")}
	}
	stage := task.Stage
	if stage == "" {
		stage = m.t("Working")
	}
	lines := []string{
		stylePrimaryB.Render(m.trKnown(stage)),
		styleMuted.Render(truncateRunes(task.Detail, width)),
		"",
	}
	if task.Total > 0 {
		lines = append(lines,
			m.progressBar(task.Done, task.Total, max(10, width-12))+"  "+
				styleText.Render(fmt.Sprintf("%3d%%", m.progressPercent())),
			styleDim.Render(fmt.Sprintf("%s / %s · %s",
				util.HumanBytes(task.Done), util.HumanBytes(task.Total), util.HumanDuration(time.Since(task.Started)))),
		)
	} else {
		lines = append(lines, spinnerFrame(m.spinner)+" "+styleDim.Render(m.tf("working · %s", util.HumanDuration(time.Since(task.Started)))))
	}
	if task.FilesTotal > 0 {
		lines = append(lines, styleDim.Render(m.tf("%d / %d files", task.FilesDone, task.FilesTotal)))
	}
	lines = append(lines, "")
	if task.Cancelled {
		lines = append(lines, styleYellow.Render(m.t("Cancelling…")))
	}
	return lines
}
