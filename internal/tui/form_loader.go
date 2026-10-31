package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MoonCC233/MCLauncher-TUI/internal/loader"
)

// pendingCreate remembers an instance that is created once its loader finished
// installing, because the instance has to reference the loader version.
type pendingCreate struct {
	name   string
	memory int
	base   string
}

// formLoaderKinds is the loader menu shown inside the new-instance form.
var formLoaderKinds = []loader.Kind{
	loader.Forge,
	loader.NeoForge,
	loader.Fabric,
	loader.Quilt,
	loader.OptiFine,
}

// formRowCount is the number of focusable rows: the text fields plus the
// loader rows when they are shown.
func (m *Model) formRowCount() int {
	rows := len(m.form.Fields)
	if m.form.LoaderRow {
		rows += len(formLoaderKinds)
	}
	return rows
}

// formLoaderFocus reports whether a loader row currently has focus.
func (m *Model) formLoaderFocus() bool {
	return m.form.LoaderRow && m.form.Focus >= len(m.form.Fields) && m.form.Focus < m.formRowCount()
}

// formLoaderIndex is the selected loader row, or -1 when a text field is
// focused.
func (m *Model) formLoaderIndex() int {
	if !m.formLoaderFocus() {
		return -1
	}
	return m.form.Focus - len(m.form.Fields)
}

// focusFormRow moves the form focus by delta across the fields and the loader
// rows, blurring or focusing the text input as needed.
func (m *Model) focusFormRow(delta int) tea.Cmd {
	rows := m.formRowCount()
	if rows == 0 {
		return nil
	}
	return m.setFormFocus((m.form.Focus + delta%rows + rows) % rows)
}

// setFormFocus moves the form focus to one row, blurring the previous input and
// focusing the new one when it is a text field.
func (m *Model) setFormFocus(index int) tea.Cmd {
	if index < 0 || index >= m.formRowCount() {
		return nil
	}
	if m.form.Focus >= 0 && m.form.Focus < len(m.form.Fields) {
		m.form.Fields[m.form.Focus].Blur()
	}
	m.form.Focus = index
	m.form.Err = ""
	if index < len(m.form.Fields) {
		return m.form.Fields[index].Focus()
	}
	return nil
}

// openLoaderForFormKind opens the loader picker directly on the version step of
// one loader, which is what clicking a loader row in the install sheet does.
func (m *Model) openLoaderForFormKind(kind loader.Kind) (tea.Model, tea.Cmd) {
	name, memory, ok := m.instanceFormValues()
	if !ok {
		return m, nil
	}
	base := m.form.VersionID
	m.pendingCreate = &pendingCreate{name: name, memory: memory, base: base}
	m.openLoaderPicker(base)
	m.loaderPick.FromForm = true
	m.loaderPick.Kind = kind
	m.loaderPick.Err = ""
	m.loaderPick.Cursor = 0

	if kind == loader.OptiFine {
		m.openPrompt(promptOptiFineJar, m.t("OptiFine jar"), "",
			m.t("Full path to an OptiFine_<version>_<edition>.jar file"), 260)
		return m, nil
	}

	m.loaderPick.Step = loaderStepVersion
	m.loaderPick.Loading = true
	m.loaderPick.Versions = nil
	m.modal = modalLoader
	return m, fetchLoaderVersionsCmd(m.client, kind, m.loaderPick.Minecraft)
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
