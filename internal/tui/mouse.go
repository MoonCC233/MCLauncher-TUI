package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type hitKind int

const (
	hitKey hitKind = iota
	hitNav
	hitRow
	hitChip
	hitFormField
	hitJavaRow
	hitLanguageRow
)

// hitRegion is a clickable rectangle recorded while the frame is rendered.
// Bubble Tea sends mouse coordinates in screen cells, so the same layout code
// that draws a row also registers where it can be clicked.
type hitRegion struct {
	X, Y, W, H int
	Kind       hitKind
	Key        tea.KeyMsg
	Screen     Screen
	Index      int
	Scope      int
}

func (m *Model) addHit(x, y, w, h int, region hitRegion) {
	if w <= 0 || h <= 0 {
		return
	}
	region.X, region.Y, region.W, region.H = x, y, w, h
	m.hits = append(m.hits, region)
}

func (m *Model) hitAt(x, y int) (hitRegion, bool) {
	for i := len(m.hits) - 1; i >= 0; i-- {
		h := m.hits[i]
		if x >= h.X && x < h.X+h.W && y >= h.Y && y < h.Y+h.H {
			return h, true
		}
	}
	return hitRegion{}, false
}

// keyMsg builds a synthetic key message so mouse clicks can reuse the
// keyboard handlers without duplicating action logic.
func keyMsg(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "shift+tab":
		return tea.KeyMsg{Type: tea.KeyShiftTab}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "pgup":
		return tea.KeyMsg{Type: tea.KeyPgUp}
	case "pgdown":
		return tea.KeyMsg{Type: tea.KeyPgDown}
	case "home":
		return tea.KeyMsg{Type: tea.KeyHome}
	case "end":
		return tea.KeyMsg{Type: tea.KeyEnd}
	case " ":
		return tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func (m *Model) updateMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if tea.MouseEvent(msg).IsWheel() {
		dir := 1
		if msg.Button == tea.MouseButtonWheelUp {
			dir = -1
		}
		return m.scrollBy(dir)
	}
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return m, nil
	}
	if m.modal != modalNone {
		if hit, ok := m.hitAt(msg.X, msg.Y); ok {
			return m.activateHit(hit)
		}
		if !m.modalContains(msg.X, msg.Y) && m.modal != modalProgress {
			m.modal = modalNone
		}
		return m, nil
	}
	if hit, ok := m.hitAt(msg.X, msg.Y); ok {
		return m.activateHit(hit)
	}
	return m, nil
}

func (m *Model) modalContains(x, y int) bool {
	if m.modalW <= 0 || m.modalH <= 0 {
		return true
	}
	return x >= m.modalX && x < m.modalX+m.modalW && y >= m.modalY && y < m.modalY+m.modalH
}

func (m *Model) activateHit(hit hitRegion) (tea.Model, tea.Cmd) {
	switch hit.Kind {
	case hitKey:
		return m.Update(hit.Key)
	case hitNav:
		m.switchScreen(hit.Screen)
		return m, nil
	case hitChip:
		m.versionScope = hit.Scope
		m.versionCursor = 0
		m.versionOffset = 0
		m.rebuildVersions()
		return m, nil
	case hitRow:
		double := m.registerClick(hit)
		m.selectRow(hit)
		if double {
			return m.Update(keyMsg("enter"))
		}
		return m, nil
	case hitFormField:
		if len(m.form.Fields) == 0 {
			return m, nil
		}
		idx := hit.Index
		if idx < 0 || idx >= len(m.form.Fields) {
			return m, nil
		}
		m.form.Fields[m.form.Focus].Blur()
		m.form.Focus = idx
		m.form.Err = ""
		return m, m.form.Fields[idx].Focus()
	case hitJavaRow:
		double := m.registerClick(hit)
		m.javaPicker = hit.Index
		if double {
			return m.chooseJava()
		}
		return m, nil
	case hitLanguageRow:
		double := m.registerClick(hit)
		m.languagePick = hit.Index
		if double {
			return m.chooseLanguage()
		}
		return m, nil
	}
	return m, nil
}

func (m *Model) registerClick(hit hitRegion) bool {
	now := time.Now()
	double := now.Sub(m.lastClick) < 450*time.Millisecond &&
		m.lastClickX == hit.X && m.lastClickY == hit.Y
	if double {
		// Reset so a third click starts a fresh single/double-click pair.
		m.lastClick = time.Time{}
		m.lastClickX, m.lastClickY = 0, 0
		return true
	}
	m.lastClick = now
	m.lastClickX = hit.X
	m.lastClickY = hit.Y
	return false
}

func (m *Model) selectRow(hit hitRegion) {
	switch hit.Screen {
	case ScreenHome:
		m.homeCursor = hit.Index
		m.clampHomeCursor()
	case ScreenInstances:
		m.instanceCursor = hit.Index
		m.clampInstanceCursor()
		m.ensureInstanceVisible()
	case ScreenVersions:
		m.versionCursor = hit.Index
		m.clampVersionCursor()
	case ScreenAccounts:
		m.accountCursor = hit.Index
		m.clampAccountCursor()
		m.ensureAccountVisible()
	case ScreenSettings:
		m.settingsCursor = hit.Index
		m.clampSettingsCursor()
	}
}

func (m *Model) scrollBy(dir int) (tea.Model, tea.Cmd) {
	switch m.modal {
	case modalHelp:
		m.helpScroll += dir * 3
		m.clampHelpScroll()
		return m, nil
	case modalJava:
		if m.javaPickerCount() > 0 {
			m.javaPicker = clampInt(m.javaPicker+dir, 0, m.javaPickerCount()-1)
		}
		return m, nil
	case modalLanguage:
		if n := m.languagePickerCount(); n > 0 {
			m.languagePick = clampInt(m.languagePick+dir, 0, n-1)
		}
		return m, nil
	case modalNone:
		// handled below
	default:
		return m, nil
	}

	switch m.screen {
	case ScreenHome:
		if m.homeListSize() > 0 {
			m.homeCursor = clampInt(m.homeCursor+dir, 0, m.homeListSize()-1)
		}
	case ScreenInstances:
		m.instanceCursor = clampInt(m.instanceCursor+dir, 0, max(0, len(m.cfg.Instances)-1))
		m.ensureInstanceVisible()
	case ScreenVersions:
		if len(m.visible) > 0 {
			m.versionCursor = clampInt(m.versionCursor+dir, 0, len(m.visible)-1)
			m.ensureVersionVisible()
		}
	case ScreenAccounts:
		m.accountCursor = clampInt(m.accountCursor+dir, 0, max(0, len(m.cfg.Accounts)-1))
		m.ensureAccountVisible()
	case ScreenSettings:
		m.settingsCursor = clampInt(m.settingsCursor+dir, 0, len(m.settingsRows())-1)
	case ScreenConsole:
		if dir < 0 {
			m.logFollow = false
			m.logScroll += 3
			m.clampLogScroll()
		} else {
			m.logScroll -= 3
			if m.logScroll <= 0 {
				m.logScroll = 0
				m.logFollow = true
			}
		}
	}
	return m, nil
}

func (m *Model) clampHelpScroll() {
	visible := m.height - 11
	if visible < 6 {
		visible = 6
	}
	all := len(m.helpContent(m.modalContentWidth()))
	if visible > all {
		visible = all
	}
	maxScroll := all - visible
	if maxScroll < 0 {
		maxScroll = 0
	}
	m.helpScroll = clampInt(m.helpScroll, 0, maxScroll)
}

func (m *Model) clampSettingsCursor() {
	rows := len(m.settingsRows())
	if m.settingsCursor >= rows {
		m.settingsCursor = rows - 1
	}
	if m.settingsCursor < 0 {
		m.settingsCursor = 0
	}
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
