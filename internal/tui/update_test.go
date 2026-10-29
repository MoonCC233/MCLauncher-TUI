package tui

import (
	"testing"

	"github.com/MoonCC233/MCLauncher-TUI/internal/update"
)

func TestSettingsVersionChecksForUpdates(t *testing.T) {
	m := testModel(t)
	m.settingsCursor = len(m.settingsRows()) - 1
	m.editSetting()
	if !m.updateChecking {
		t.Error("the launcher version row should start an update check")
	}
}

func TestUpdateCheckShowsConfirmation(t *testing.T) {
	m := testModel(t)
	release := &update.Release{TagName: "v9.9.9"}
	m.handleUpdateCheck(updateCheckMsg{release: release})
	if m.modal != modalConfirm {
		t.Fatalf("modal = %v, want confirm", m.modal)
	}
	if m.confirm.Kind != confirmInstallUpdate {
		t.Errorf("confirm kind = %v", m.confirm.Kind)
	}
	if m.updateRelease != release {
		t.Error("release was not retained")
	}
}

func TestUpdateCheckLatestVersion(t *testing.T) {
	m := testModel(t)
	m.handleUpdateCheck(updateCheckMsg{})
	if m.modal != modalNone {
		t.Errorf("modal = %v, want none", m.modal)
	}
	if m.toast.Text == "" {
		t.Error("expected an up-to-date toast")
	}
}

func TestUpdateCheckDevelopmentBuild(t *testing.T) {
	m := testModel(t)
	m.handleUpdateCheck(updateCheckMsg{err: update.ErrDevelopment})
	if m.modal != modalAlert {
		t.Errorf("modal = %v, want alert", m.modal)
	}
	if m.alert.Title != m.t("Check for updates") {
		t.Errorf("alert title = %q", m.alert.Title)
	}
}
