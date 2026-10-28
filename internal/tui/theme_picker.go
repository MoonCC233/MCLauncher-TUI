package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m *Model) currentThemePreset() int {
	for i, preset := range themePresets {
		if preset.ID == m.cfg.Settings.Theme {
			return i
		}
	}
	return 0
}

func (m *Model) themePickerCount() int {
	return len(themePresets) + 2
}

func (m *Model) openThemePicker() {
	m.themePick = m.currentThemePreset()
	m.modal = modalTheme
}

func (m *Model) updateThemePicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.modal = modalNone
		return m, nil
	case "up", "k":
		m.themePick--
	case "down", "j":
		m.themePick++
	case "enter":
		return m.chooseTheme()
	}
	m.themePick = clampInt(m.themePick, 0, m.themePickerCount()-1)
	return m, nil
}

func (m *Model) chooseTheme() (tea.Model, tea.Cmd) {
	switch {
	case m.themePick < len(themePresets):
		preset := themePresets[m.themePick]
		m.setTheme(preset.ID, "")
		m.modal = modalNone
		m.setToast(m.tf("Theme: %s", preset.Name), toastSuccess)
		return m, nil
	case m.themePick == len(themePresets):
		m.openPrompt(promptThemeAccent, m.t("Accent color"), m.cfg.Settings.ThemeAccent, m.t("Hex color such as #fab283"), 7)
		return m, nil
	default:
		m.setTheme(m.cfg.Settings.Theme, "")
		m.modal = modalNone
		m.setToast(m.t("Accent reset to theme default"), toastSuccess)
		return m, nil
	}
}

func (m *Model) themePickerBody(contentW int) ([]string, []hitRegion) {
	var body []string
	var hits []hitRegion
	active := m.currentThemePreset()
	for i := 0; i < m.themePickerCount(); i++ {
		selected := i == m.themePick
		marker := "  "
		if selected {
			marker = stylePrimary.Render("▸ ")
		}
		var line string
		switch {
		case i < len(themePresets):
			preset := themePresets[i]
			line = javaPickerRow(marker, preset.Name, themeSwatches(preset.Primary, preset.Green, preset.Cyan, preset.Purple), i == active, selected, contentW)
		case i == len(themePresets):
			right := m.t("theme default")
			if m.cfg.Settings.ThemeAccent != "" {
				right = m.cfg.Settings.ThemeAccent
			}
			line = javaPickerRow(marker, m.t("Custom accent…"), right, false, selected, contentW)
		default:
			right := m.t("not set")
			if m.cfg.Settings.ThemeAccent != "" {
				right = m.t("restore theme color")
			}
			line = javaPickerRow(marker, m.t("Reset accent"), right, false, selected, contentW)
		}
		body = append(body, line)
		hits = append(hits, hitRegion{Kind: hitThemeRow, Index: i, Y: 3 + len(body) - 1, W: contentW, H: 1})
	}
	return body, hits
}

func themeSwatches(colors ...string) string {
	swatch := "● "
	line := ""
	for _, color := range colors {
		line += lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(swatch)
	}
	return line
}
