package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

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
	case modalProgress:
		if msg.String() == "esc" {
			m.cancelTask()
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
	if m.helpScroll < 0 {
		m.helpScroll = 0
	}
	return m, nil
}

func (m *Model) updateForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.modal = modalNone
		return m, nil
	case "tab", "down":
		if len(m.form.Fields) > 0 {
			m.form.Fields[m.form.Focus].Blur()
			m.form.Focus = (m.form.Focus + 1) % len(m.form.Fields)
			m.form.Err = ""
			return m, m.form.Fields[m.form.Focus].Focus()
		}
		return m, nil
	case "shift+tab", "up":
		if len(m.form.Fields) > 0 {
			m.form.Fields[m.form.Focus].Blur()
			m.form.Focus = (m.form.Focus + len(m.form.Fields) - 1) % len(m.form.Fields)
			m.form.Err = ""
			return m, m.form.Fields[m.form.Focus].Focus()
		}
		return m, nil
	case "enter":
		return m.submitForm()
	}
	if len(m.form.Fields) == 0 {
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
					m.alert = AlertState{Title: "Could not delete instance", Body: err.Error()}
					m.modal = modalAlert
					return m, nil
				}
				m.clampInstanceCursor()
				m.clampHomeCursor()
				m.ensureInstanceVisible()
				m.setToast("Deleted "+name, toastSuccess)
				m.addLog("launcher", "deleted instance "+name)
			}
		case confirmRemoveAccount:
			target := m.confirm.Target
			if err := m.cfg.RemoveAccount(target); err != nil {
				m.alert = AlertState{Title: "Could not remove account", Body: err.Error()}
				m.modal = modalAlert
				return m, nil
			}
			m.clampAccountCursor()
			m.setToast("Account removed", toastSuccess)
			m.addLog("launcher", "removed account")
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
		m.modal = modalNone
		return m, nil
	case "enter":
		return m.submitPrompt()
	}
	var cmd tea.Cmd
	m.prompt.Input, cmd = m.prompt.Input.Update(msg)
	return m, cmd
}

func (m *Model) javaPickerCount() int {
	return len(m.javaRuntimes) + 3
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
			return m, scanJavaCmd()
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
		m.setToast("Java runtime: auto-detect", toastSuccess)
		return m, nil
	}
	n := len(m.javaRuntimes)
	if idx >= 1 && idx <= n {
		rt := m.javaRuntimes[idx-1]
		m.cfg.Settings.JavaPath = rt.Path
		m.saveConfig()
		m.modal = modalNone
		m.setToast("Java runtime: "+rt.Short(), toastSuccess)
		m.addLog("launcher", "java runtime: "+rt.Path)
		return m, nil
	}
	if idx == n+1 {
		if !m.javaScanning {
			m.javaScanning = true
			return m, scanJavaCmd()
		}
		return m, nil
	}
	m.openPrompt(promptJavaPath, "Java executable", m.cfg.Settings.JavaPath, "Full path to java.exe or a JDK folder", 260)
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
		{"Sections", []entry{
			{"tab / shift+tab", "next / previous section"},
			{"1 … 6", "jump directly to a section"},
			{"esc", "back to Home"},
			{"ctrl+c", "quit launcher"},
		}},
		{"Navigation", []entry{
			{"↑ ↓ / j k", "move the cursor"},
			{"pgup / pgdown", "jump by a page"},
			{"home / end", "first / last item"},
			{"enter", "open, play or edit"},
		}},
		{"Instances", []entry{
			{"n", "create from the Versions screen"},
			{"e", "rename / change memory"},
			{"d", "delete instance (files are kept)"},
			{"o", "open the instance folder"},
			{"x", "stop the running game"},
		}},
		{"Versions", []entry{
			{"/", "filter by version id"},
			{"t", "all → releases → snapshots"},
			{"enter", "install if needed, then create instance"},
			{"i", "install without creating an instance"},
			{"r", "refresh the Mojang manifest"},
		}},
		{"Accounts & settings", []entry{
			{"a", "add an offline account"},
			{"enter", "set the active account"},
			{"s", "rescan Java runtimes"},
			{"← → / - +", "adjust memory or download threads"},
		}},
		{"Console", []entry{
			{"↑ ↓ / pgup pgdown", "scroll through game output"},
			{"G / end", "follow the tail again"},
			{"x", "stop the running game"},
			{"c", "clear the console"},
			{"o", "open the logs folder"},
		}},
	}
	out := []string{styleBold.Render("Keyboard shortcuts"), styleBorder.Render(strings.Repeat("─", width)), ""}
	for _, section := range sections {
		out = append(out, stylePrimaryB.Render(section.title))
		for _, e := range section.entries {
			key := fitLine(styleYellow.Render(e.key), 18)
			out = append(out, "  "+key+styleMuted.Render(e.desc))
		}
		out = append(out, "")
	}
	out = append(out, styleDim.Render("Offline mode: launcher exits without stopping a running game."))
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
		return []string{"Nothing is running."}
	}
	stage := task.Stage
	if stage == "" {
		stage = "Working"
	}
	lines := []string{
		stylePrimaryB.Render(stage),
		styleMuted.Render(truncateRunes(task.Detail, width)),
		"",
	}
	if task.Total > 0 {
		lines = append(lines,
			progressBar(task.Done, task.Total, max(10, width-12))+"  "+
				styleText.Render(fmt.Sprintf("%3d%%", m.progressPercent())),
			styleDim.Render(fmt.Sprintf("%s / %s · %s",
				util.HumanBytes(task.Done), util.HumanBytes(task.Total), util.HumanDuration(time.Since(task.Started)))),
		)
	} else {
		lines = append(lines, spinnerFrame(m.spinner)+" "+styleDim.Render("working · "+util.HumanDuration(time.Since(task.Started))))
	}
	if task.FilesTotal > 0 {
		lines = append(lines, styleDim.Render(fmt.Sprintf("%d / %d files", task.FilesDone, task.FilesTotal)))
	}
	lines = append(lines, "")
	if task.Cancelled {
		lines = append(lines, styleYellow.Render("Cancelling…"))
	} else {
		lines = append(lines, styleDim.Render("esc cancel"))
	}
	return lines
}
