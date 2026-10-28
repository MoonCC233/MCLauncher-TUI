package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MoonCC233/MCLauncher-TUI/internal/loader"
)

type loaderVersionsMsg struct {
	kind        loader.Kind
	gameVersion string
	versions    []loader.Version
	err         error
}

func (m *Model) latestLoaderVersion() string {
	for _, version := range m.form.LoaderVersions {
		if version.Stable {
			return version.Version
		}
	}
	if len(m.form.LoaderVersions) > 0 {
		return m.form.LoaderVersions[0].Version
	}
	return ""
}

func (m *Model) syncFormChoices() {
	if m.form.Kind != formInstance || m.form.EditID != "" {
		m.form.Choices = nil
		return
	}
	loaderValue := m.t("None")
	if m.form.Loader != "" {
		loaderValue = loader.Label(m.form.Loader)
	}
	versionValue := m.t("Choose a mod loader first")
	versionHint := ""
	if m.form.Loader != "" {
		switch {
		case m.form.LoaderVersion != "":
			versionValue = m.form.LoaderVersion
			versionHint = m.tf("%s %s", loader.Label(m.form.Loader), m.form.LoaderVersion)
		case m.form.LoaderLoading:
			versionValue = m.t("Latest stable · loading…")
		case m.form.LoaderErr != "":
			versionValue = m.t("Latest stable · unavailable")
			versionHint = m.form.LoaderErr
		default:
			versionValue = m.t("Latest stable")
			if latest := m.latestLoaderVersion(); latest != "" {
				versionValue = m.tf("Latest stable (%s)", latest)
			}
		}
		if versionHint == "" {
			versionHint = m.t("Press Enter to choose a loader version")
		}
	}
	m.form.Choices = []FormChoice{
		{ID: "loader", Label: m.t("Mod loader"), Value: loaderValue, Hint: m.t("Install Fabric or NeoForge instead of vanilla Minecraft.")},
		{ID: "loader-version", Label: m.t("Loader version"), Value: versionValue, Hint: versionHint},
	}
}

func (m *Model) chooseFormChoice() (tea.Model, tea.Cmd) {
	if m.form.Focus < len(m.form.Fields) || m.form.Focus >= m.formRowCount() {
		return m, nil
	}
	choice := m.form.Choices[m.form.Focus-len(m.form.Fields)]
	switch choice.ID {
	case "loader":
		m.openLoaderPicker()
		return m, nil
	case "loader-version":
		if m.form.Loader == "" {
			m.form.Err = m.t("Choose a mod loader first")
			return m, nil
		}
		return m.openLoaderVersionPicker()
	}
	return m, nil
}

func (m *Model) cycleFormChoice(forward bool) (tea.Model, tea.Cmd) {
	if m.form.Focus < len(m.form.Fields) || m.form.Focus >= m.formRowCount() {
		return m, nil
	}
	choice := m.form.Choices[m.form.Focus-len(m.form.Fields)]
	if choice.ID != "loader" {
		return m.chooseFormChoice()
	}
	kinds := []loader.Kind{"", loader.Fabric, loader.NeoForge}
	current := 0
	for i, kind := range kinds {
		if kind == m.form.Loader {
			current = i
			break
		}
	}
	if forward {
		current = (current + 1) % len(kinds)
	} else {
		current = (current + len(kinds) - 1) % len(kinds)
	}
	m.setFormLoader(kinds[current])
	return m, nil
}

func (m *Model) setFormLoader(kind loader.Kind) tea.Cmd {
	if kind != m.form.Loader {
		m.form.LoaderVersion = ""
		m.form.LoaderVersions = nil
		m.form.LoaderErr = ""
		m.form.LoaderLoading = false
	}
	m.form.Loader = kind
	m.syncFormChoices()
	if kind == "" {
		return nil
	}
	return m.loadFormLoaderVersions()
}

func (m *Model) loadFormLoaderVersions() tea.Cmd {
	if m.form.Loader == "" || m.form.LoaderLoading || len(m.form.LoaderVersions) > 0 {
		return nil
	}
	m.form.LoaderLoading = true
	m.form.LoaderErr = ""
	m.syncFormChoices()
	client := m.client
	kind := m.form.Loader
	game := m.form.VersionID
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		versions, err := loader.Versions(ctx, client, kind, game)
		return loaderVersionsMsg{kind: kind, gameVersion: game, versions: versions, err: err}
	}
}

func (m *Model) handleLoaderVersions(msg loaderVersionsMsg) {
	if msg.kind != m.form.Loader || msg.gameVersion != m.form.VersionID {
		return
	}
	m.form.LoaderLoading = false
	m.form.LoaderErr = ""
	if msg.err != nil {
		m.form.LoaderErr = msg.err.Error()
		m.form.LoaderVersions = nil
		m.syncFormChoices()
		return
	}
	m.form.LoaderVersions = msg.versions
	m.syncFormChoices()
	if m.modal == modalLoaderVersions {
		m.loaderVerPick = 0
		if m.form.LoaderVersion != "" {
			for i, version := range m.form.LoaderVersions {
				if version.Version == m.form.LoaderVersion {
					m.loaderVerPick = i + 1
					break
				}
			}
		}
	}
}

func (m *Model) openLoaderPicker() {
	m.loaderPick = 0
	switch m.form.Loader {
	case loader.Fabric:
		m.loaderPick = 1
	case loader.NeoForge:
		m.loaderPick = 2
	}
	m.modal = modalLoader
}

func (m *Model) updateLoaderPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.modal = modalForm
		return m, nil
	case "up", "k":
		m.loaderPick--
	case "down", "j":
		m.loaderPick++
	case "enter":
		return m.chooseLoaderKind()
	}
	m.loaderPick = clampInt(m.loaderPick, 0, 2)
	return m, nil
}

func (m *Model) chooseLoaderKind() (tea.Model, tea.Cmd) {
	kinds := []loader.Kind{"", loader.Fabric, loader.NeoForge}
	m.loaderPick = clampInt(m.loaderPick, 0, len(kinds)-1)
	cmd := m.setFormLoader(kinds[m.loaderPick])
	m.form.Focus = len(m.form.Fields) + 1
	if m.form.Loader == "" {
		m.modal = modalForm
		return m, nil
	}
	m.openLoaderVersionPickerState()
	return m, cmd
}

func (m *Model) openLoaderVersionPicker() (tea.Model, tea.Cmd) {
	m.openLoaderVersionPickerState()
	return m, m.loadFormLoaderVersions()
}

func (m *Model) openLoaderVersionPickerState() {
	m.loaderVerPick = 0
	if m.form.LoaderVersion != "" {
		for i, version := range m.form.LoaderVersions {
			if version.Version == m.form.LoaderVersion {
				m.loaderVerPick = i + 1
				break
			}
		}
	}
	m.modal = modalLoaderVersions
}

func (m *Model) loaderVersionPickerCount() int {
	return len(m.form.LoaderVersions) + 2
}

func (m *Model) updateLoaderVersionPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.modal = modalForm
		return m, nil
	case "up", "k":
		m.loaderVerPick--
	case "down", "j":
		m.loaderVerPick++
	case "r":
		if m.form.LoaderErr != "" && !m.form.LoaderLoading {
			m.form.LoaderVersions = nil
			cmd := m.loadFormLoaderVersions()
			return m, cmd
		}
	case "enter":
		return m.chooseLoaderVersion()
	}
	m.loaderVerPick = clampInt(m.loaderVerPick, 0, m.loaderVersionPickerCount()-1)
	return m, nil
}

func (m *Model) chooseLoaderVersion() (tea.Model, tea.Cmd) {
	count := len(m.form.LoaderVersions)
	switch {
	case m.loaderVerPick == 0:
		m.form.LoaderVersion = ""
		m.modal = modalForm
		m.syncFormChoices()
		return m, m.focusFormRow(len(m.form.Fields) - 1)
	case m.loaderVerPick <= count:
		m.form.LoaderVersion = m.form.LoaderVersions[m.loaderVerPick-1].Version
		m.modal = modalForm
		m.syncFormChoices()
		return m, m.focusFormRow(len(m.form.Fields) - 1)
	default:
		value := m.form.LoaderVersion
		if value == "" {
			value = m.latestLoaderVersion()
		}
		m.openPrompt(promptLoaderVersion, m.t("Loader version"), value, m.t("Enter a version published by the loader project."), 64)
		return m, nil
	}
}

func (m *Model) loaderKindPickerBody(contentW int) ([]string, []hitRegion) {
	var body []string
	var hits []hitRegion
	rows := []struct {
		kind  loader.Kind
		label string
		right string
	}{
		{"", m.t("None"), m.t("vanilla")},
		{loader.Fabric, "Fabric", m.t("mods")},
		{loader.NeoForge, "NeoForge", m.t("mods")},
	}
	for i, row := range rows {
		selected := i == m.loaderPick
		marker := "  "
		if selected {
			marker = stylePrimary.Render("▸ ")
		}
		active := row.kind == m.form.Loader
		body = append(body, javaPickerRow(marker, row.label, row.right, active, selected, contentW))
		hits = append(hits, hitRegion{Kind: hitLoaderKindRow, Index: i, Y: 3 + len(body) - 1, W: contentW, H: 1})
	}
	return body, hits
}

func (m *Model) loaderVersionPickerBody(contentW int) ([]string, []hitRegion) {
	var body []string
	var hits []hitRegion
	latest := m.latestLoaderVersion()
	latestRight := latest
	if latestRight == "" {
		latestRight = m.t("loading…")
	}
	if m.form.LoaderErr != "" {
		latestRight = m.t("unavailable")
	}
	selected := m.loaderVerPick == 0
	marker := "  "
	if selected {
		marker = stylePrimary.Render("▸ ")
	}
	label := m.t("Latest stable")
	if latest != "" {
		label = m.tf("Latest stable (%s)", latest)
	}
	active := m.form.LoaderVersion == "" || (latest != "" && m.form.LoaderVersion == latest)
	body = append(body, javaPickerRow(marker, label, latestRight, active, selected, contentW))
	hits = append(hits, hitRegion{Kind: hitLoaderVersionRow, Index: 0, Y: 3 + len(body) - 1, W: contentW, H: 1})

	for i, version := range m.form.LoaderVersions {
		index := i + 1
		selected := m.loaderVerPick == index
		marker := "  "
		if selected {
			marker = stylePrimary.Render("▸ ")
		}
		right := m.t("stable")
		if !version.Stable {
			right = m.t("beta")
		}
		body = append(body, javaPickerRow(marker, version.Version, right, m.form.LoaderVersion == version.Version, selected, contentW))
		hits = append(hits, hitRegion{Kind: hitLoaderVersionRow, Index: index, Y: 3 + len(body) - 1, W: contentW, H: 1})
	}

	customIndex := len(m.form.LoaderVersions) + 1
	selected = m.loaderVerPick == customIndex
	marker = "  "
	if selected {
		marker = stylePrimary.Render("▸ ")
	}
	body = append(body, javaPickerRow(marker, m.t("Custom version…"), "", false, selected, contentW))
	hits = append(hits, hitRegion{Kind: hitLoaderVersionRow, Index: customIndex, Y: 3 + len(body) - 1, W: contentW, H: 1})

	if m.form.LoaderLoading {
		body = append(body, "", "  "+stylePrimary.Render(spinnerFrame(m.spinner))+" "+styleMuted.Render(m.t("Loading loader versions…")))
	} else if m.form.LoaderErr != "" {
		body = append(body, "", "  "+styleRed.Render(m.tf("Could not load loader versions: %s", m.form.LoaderErr)))
	}
	return body, hits
}

func (m *Model) submitLoaderForm(name string, memory int) (tea.Model, tea.Cmd) {
	if m.form.Loader == "" {
		return m, nil
	}
	if m.form.LoaderLoading {
		m.form.Err = m.t("Wait for the loader version list to finish loading.")
		return m, nil
	}
	loaderVersion := m.form.LoaderVersion
	if loaderVersion == "" {
		loaderVersion = m.latestLoaderVersion()
	}
	if loaderVersion == "" {
		m.form.Err = m.t("Choose a loader version or enter one manually.")
		return m, nil
	}
	m.modal = modalNone
	return m, m.startLoaderInstall(m.form.Loader, m.form.VersionID, loaderVersion, name, memory)
}

func sanitizeLoaderVersion(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, " \t\r\n/\\") {
		return "", false
	}
	if strings.Contains(value, "..") {
		return "", false
	}
	return value, true
}

func loaderVersionSummary(kind loader.Kind, version string) string {
	return fmt.Sprintf("%s %s", loader.Label(kind), version)
}
