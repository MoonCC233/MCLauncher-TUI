package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/MoonCC233/MCLauncher-TUI/internal/config"
)

// titleAnimLabel is the value shown on the Settings row.
func (m *Model) titleAnimLabel() string {
	return m.t(titleAnimName(m.titleAnim()))
}

// titleAnimName maps an animation id to its translation key.
func titleAnimName(id string) string {
	switch id {
	case "off":
		return "Off"
	case "wave":
		return "Wave"
	case "breathe":
		return "Breathe"
	default:
		return "Sheen"
	}
}

func (m *Model) currentTitleAnim() int {
	anim := m.titleAnim()
	for i, id := range config.TitleAnimIDs {
		if id == anim {
			return i
		}
	}
	return 0
}

func (m *Model) openTitleAnimPicker() {
	m.titleAnimPick = m.currentTitleAnim()
	m.modal = modalTitleAnim
}

func (m *Model) updateTitleAnimPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.modal = modalNone
		return m, nil
	case "up", "k":
		m.titleAnimPick--
	case "down", "j":
		m.titleAnimPick++
	case "enter":
		return m.chooseTitleAnim()
	}
	m.titleAnimPick = clampInt(m.titleAnimPick, 0, len(config.TitleAnimIDs)-1)
	return m, nil
}

func (m *Model) chooseTitleAnim() (tea.Model, tea.Cmd) {
	id := config.TitleAnimIDs[clampInt(m.titleAnimPick, 0, len(config.TitleAnimIDs)-1)]
	m.setTitleAnim(id)
	m.modal = modalNone
	m.setToast(m.tf("Title animation: %s", m.t(titleAnimName(id))), toastSuccess)
	return m, nil
}

// cycleTitleAnim walks the animation list, which is what the left/right keys on
// the Settings row do.
func (m *Model) cycleTitleAnim(delta int) {
	ids := config.TitleAnimIDs
	next := (m.currentTitleAnim() + delta) % len(ids)
	if next < 0 {
		next += len(ids)
	}
	id := ids[next]
	m.setTitleAnim(id)
	m.setToast(m.tf("Title animation: %s", m.t(titleAnimName(id))), toastSuccess)
}

func (m *Model) setTitleAnim(id string) {
	if !config.ValidTitleAnim(id) {
		id = config.DefaultTitleAnim
	}
	m.cfg.Settings.TitleAnim = id
	m.saveConfig()
}

func (m *Model) titleAnimPickerBody(contentW int) ([]string, []hitRegion) {
	var body []string
	var hits []hitRegion
	active := m.currentTitleAnim()
	for i, id := range config.TitleAnimIDs {
		selected := i == m.titleAnimPick
		marker := "  "
		if selected {
			marker = stylePrimary.Render("▸ ")
		}
		line := javaPickerRow(marker, m.t(titleAnimName(id)), titleAnimSwatch(id), i == active, selected, contentW)
		body = append(body, line)
		hits = append(hits, hitRegion{Kind: hitTitleAnimRow, Index: i, Y: 3 + len(body) - 1, W: contentW, H: 1})
	}
	return body, hits
}

// titleAnimSwatch previews the motion with a short ladder of shades.
func titleAnimSwatch(id string) string {
	sample := "▁▂▃▄▅▆▇"
	if id == "off" {
		return styleDim.Render("──────")
	}
	return renderTitle(sample, 12, id, 40)
}

// titleFontName maps a font id to its translation key.
func titleFontName(id string) string {
	if id == "line" {
		return "Line"
	}
	return "Block"
}

// titleFontLabel is the value shown on the Settings row.
func (m *Model) titleFontLabel() string {
	return m.t(titleFontName(m.titleFont()))
}

func (m *Model) currentTitleFont() int {
	font := m.titleFont()
	for i, id := range config.TitleFontIDs {
		if id == font {
			return i
		}
	}
	return 0
}

func (m *Model) openTitleFontPicker() {
	m.titleFontPick = m.currentTitleFont()
	m.modal = modalTitleFont
}

func (m *Model) updateTitleFontPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.modal = modalNone
		return m, nil
	case "up", "k":
		m.titleFontPick--
	case "down", "j":
		m.titleFontPick++
	case "enter":
		return m.chooseTitleFont()
	}
	m.titleFontPick = clampInt(m.titleFontPick, 0, len(config.TitleFontIDs)-1)
	return m, nil
}

func (m *Model) chooseTitleFont() (tea.Model, tea.Cmd) {
	id := config.TitleFontIDs[clampInt(m.titleFontPick, 0, len(config.TitleFontIDs)-1)]
	m.setTitleFont(id)
	m.modal = modalNone
	m.setToast(m.tf("Title style: %s", m.t(titleFontName(id))), toastSuccess)
	return m, nil
}

// cycleTitleFont walks the font list, which is what the left/right keys on the
// Settings row do.
func (m *Model) cycleTitleFont(delta int) {
	ids := config.TitleFontIDs
	next := (m.currentTitleFont() + delta) % len(ids)
	if next < 0 {
		next += len(ids)
	}
	id := ids[next]
	m.setTitleFont(id)
	m.setToast(m.tf("Title style: %s", m.t(titleFontName(id))), toastSuccess)
}

func (m *Model) setTitleFont(id string) {
	if !config.ValidTitleFont(id) {
		id = config.DefaultTitleFont
	}
	m.cfg.Settings.TitleFont = id
	m.saveConfig()
}

// titleFontPickerBody lists the fonts and previews the highlighted one.
func (m *Model) titleFontPickerBody(contentW int) ([]string, []hitRegion) {
	var body []string
	var hits []hitRegion
	active := m.currentTitleFont()
	for i, id := range config.TitleFontIDs {
		selected := i == m.titleFontPick
		marker := "  "
		if selected {
			marker = stylePrimary.Render("▸ ")
		}
		line := javaPickerRow(marker, m.t(titleFontName(id)), titleFontSwatch(id), i == active, selected, contentW)
		body = append(body, line)
		hits = append(hits, hitRegion{Kind: hitTitleFontRow, Index: i, Y: 3 + len(body) - 1, W: contentW, H: 1})
	}
	body = append(body, "")
	preview := titleArt(homeTitle, config.TitleFontIDs[clampInt(m.titleFontPick, 0, len(config.TitleFontIDs)-1)])
	for _, row := range preview {
		body = append(body, "  "+styleMuted.Render(row))
	}
	return body, hits
}

func titleFontSwatch(id string) string {
	if id == "line" {
		return styleMuted.Render("/|\\_")
	}
	return stylePrimary.Render("████")
}
