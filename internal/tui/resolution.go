package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

type resolutionPreset struct {
	Width  int
	Height int
	Label  string
}

var resolutionPresets = []resolutionPreset{
	{854, 480, "SD"},
	{1280, 720, "HD"},
	{1366, 768, "WXGA"},
	{1600, 900, "HD+"},
	{1920, 1080, "Full HD"},
	{2560, 1440, "QHD"},
	{3840, 2160, "4K UHD"},
}

func (m *Model) resolutionPickerCount() int {
	return len(resolutionPresets) + 1
}

func (m *Model) currentResolutionPreset() int {
	for i, preset := range resolutionPresets {
		if preset.Width == m.cfg.Settings.Width && preset.Height == m.cfg.Settings.Height {
			return i
		}
	}
	return len(resolutionPresets)
}

func (m *Model) openResolutionPicker() {
	m.resolutionPick = m.currentResolutionPreset()
	m.modal = modalResolution
}

func (m *Model) updateResolutionPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.modal = modalNone
		return m, nil
	case "up", "k":
		m.resolutionPick--
	case "down", "j":
		m.resolutionPick++
	case "enter":
		return m.chooseResolution()
	}
	m.resolutionPick = clampInt(m.resolutionPick, 0, m.resolutionPickerCount()-1)
	return m, nil
}

func (m *Model) chooseResolution() (tea.Model, tea.Cmd) {
	if m.resolutionPick < len(resolutionPresets) {
		preset := resolutionPresets[m.resolutionPick]
		m.cfg.Settings.Width = preset.Width
		m.cfg.Settings.Height = preset.Height
		m.saveConfig()
		m.modal = modalNone
		m.setToast(m.tf("Resolution: %d × %d", preset.Width, preset.Height), toastSuccess)
		return m, nil
	}
	m.openPrompt(promptResolution, m.t("Window size"), fmt.Sprintf("%dx%d", m.cfg.Settings.Width, m.cfg.Settings.Height), m.t("width x height · 854x480"), 16)
	return m, nil
}

func (m *Model) resolutionPickerBody(contentW int) ([]string, []hitRegion) {
	var body []string
	var hits []hitRegion
	active := m.currentResolutionPreset()
	for i := 0; i < m.resolutionPickerCount(); i++ {
		selected := i == m.resolutionPick
		marker := "  "
		if selected {
			marker = stylePrimary.Render("▸ ")
		}
		var line string
		if i < len(resolutionPresets) {
			preset := resolutionPresets[i]
			label := fmt.Sprintf("%d × %d", preset.Width, preset.Height)
			line = javaPickerRow(marker, label, preset.Label, i == active, selected, contentW)
		} else {
			line = javaPickerRow(marker, m.t("Custom…"), "", false, selected, contentW)
		}
		body = append(body, line)
		hits = append(hits, hitRegion{Kind: hitResolutionRow, Index: i, Y: 3 + len(body) - 1, W: contentW, H: 1})
	}
	return body, hits
}
