package tui

// onOffLabel renders a boolean setting.
func (m *Model) onOffLabel(on bool) string {
	if on {
		return m.t("On")
	}
	return m.t("Off")
}

// stopsGameOnQuit reports whether quitting the launcher should also stop the
// game. Players can opt out from the Settings screen.
func (m *Model) stopsGameOnQuit() bool { return !m.cfg.Settings.KeepGameRunning }

// toggleKeepGameRunning flips whether quitting the launcher also stops Minecraft.
func (m *Model) toggleKeepGameRunning() {
	m.cfg.Settings.KeepGameRunning = !m.cfg.Settings.KeepGameRunning
	if err := m.saveConfig(); err != nil {
		return
	}
	if m.cfg.Settings.KeepGameRunning {
		m.setToast(m.t("Minecraft keeps running after the launcher quits"), toastInfo)
	} else {
		m.setToast(m.t("Quitting the launcher also stops Minecraft"), toastInfo)
	}
}
