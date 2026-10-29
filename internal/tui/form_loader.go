package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// pendingCreate remembers an instance that is created once its loader finished
// installing, because the instance has to reference the loader version.
type pendingCreate struct {
	name   string
	memory int
	base   string
}

// formRowCount is the number of focusable rows: the text fields plus the loader
// row when it is shown.
func (m *Model) formRowCount() int {
	rows := len(m.form.Fields)
	if m.form.LoaderRow {
		rows++
	}
	return rows
}

// formLoaderFocus reports whether the loader row currently has focus.
func (m *Model) formLoaderFocus() bool {
	return m.form.LoaderRow && m.form.Focus == len(m.form.Fields)
}

// focusFormRow moves the form focus by delta across the fields and the loader
// row, blurring or focusing the text input as needed.
func (m *Model) focusFormRow(delta int) tea.Cmd {
	rows := m.formRowCount()
	if rows == 0 {
		return nil
	}
	if m.form.Focus < len(m.form.Fields) {
		m.form.Fields[m.form.Focus].Blur()
	}
	m.form.Focus = (m.form.Focus + delta%rows + rows) % rows
	m.form.Err = ""
	if m.form.Focus < len(m.form.Fields) {
		return m.form.Fields[m.form.Focus].Focus()
	}
	return nil
}

// openLoaderForForm leaves the form for the loader picker: the player then picks
// the loader, its version and the Java runtime, exactly like from the Versions
// screen. The instance is created once the loader is installed.
func (m *Model) openLoaderForForm() (tea.Model, tea.Cmd) {
	name, memory, ok := m.instanceFormValues()
	if !ok {
		return m, nil
	}
	base := m.form.VersionID
	m.pendingCreate = &pendingCreate{name: name, memory: memory, base: base}
	m.openLoaderPicker(base)
	return m, nil
}

// instanceFormValues validates the name and memory of the new-instance form.
func (m *Model) instanceFormValues() (string, int, bool) {
	name := ""
	if len(m.form.Fields) > 0 {
		name = strings.TrimSpace(m.form.Fields[0].Value())
	}
	if name == "" {
		m.form.Err = m.t("instance name cannot be empty")
		return "", 0, false
	}
	if len([]rune(name)) > 48 {
		m.form.Err = m.t("instance name is too long (max 48 characters)")
		return "", 0, false
	}
	memory := 0
	if len(m.form.Fields) > 1 {
		raw := strings.TrimSpace(m.form.Fields[1].Value())
		if raw != "" {
			n, err := parseInt(raw)
			if err != nil || n < 512 || n > 65536 {
				m.form.Err = m.t("memory must be between 512 and 65536 MB")
				return "", 0, false
			}
			memory = n
		}
	}
	return name, memory, true
}

// finishPendingCreate creates the instance once the loader version is ready.
func (m *Model) finishPendingCreate(loaderVersionID string) {
	pending := m.pendingCreate
	if pending == nil {
		return
	}
	m.pendingCreate = nil
	versionID := loaderVersionID
	if versionID == "" {
		versionID = pending.base
	}
	inst, err := m.cfg.AddInstance(pending.name, versionID, pending.memory)
	if err != nil {
		m.alert = AlertState{Title: m.t("Could not create instance"), Body: err.Error()}
		m.modal = modalAlert
		return
	}
	m.instanceCursor = len(m.cfg.Instances) - 1
	m.homeCursor = 0
	m.screen = ScreenInstances
	m.setToast(m.tf("Created %s", inst.Name), toastSuccess)
	m.addLog("launcher", "created instance "+inst.Name)
}

// formLoaderValue is what the loader row shows on the right.
func (m *Model) formLoaderValue() string {
	if m.form.Loader == "" {
		return m.t("None")
	}
	return loaderKindName(m.form.Loader)
}
