package tui

import (
	"github.com/MoonCC233/MCLauncher-TUI/internal/mirror"
)

// downloadMirrorLabel is what the settings row shows for the download source.
func (m *Model) downloadMirrorLabel() string {
	if mirror.Current() == mirror.BMCLAPI {
		return "BMCLAPI"
	}
	return m.t("Official")
}

// toggleDownloadMirror switches between the official servers and the BMCLAPI
// mirror, and takes effect for every download from now on.
func (m *Model) toggleDownloadMirror() {
	if m.cfg.Settings.DownloadMirror == string(mirror.BMCLAPI) {
		m.cfg.Settings.DownloadMirror = ""
	} else {
		m.cfg.Settings.DownloadMirror = string(mirror.BMCLAPI)
	}
	mirror.Set(mirror.Kind(m.cfg.Settings.DownloadMirror))
	if err := m.saveConfig(); err != nil {
		return
	}
	m.setToast(m.tf("Download source: %s", m.downloadMirrorLabel()), toastSuccess)
}
