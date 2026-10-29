package tui

import (
	"context"
	"errors"
	"net/http"
	"runtime"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MoonCC233/MCLauncher-TUI/internal/install"
	"github.com/MoonCC233/MCLauncher-TUI/internal/update"
)

type updateCheckMsg struct {
	release *update.Release
	err     error
}

func checkUpdateCmd(client *http.Client, current string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		release, err := update.Check(ctx, client, current)
		return updateCheckMsg{release: release, err: err}
	}
}

func (m *Model) checkForUpdates() (tea.Model, tea.Cmd) {
	if m.updateChecking {
		return m, nil
	}
	if m.updateInstalled != "" {
		m.alert = AlertState{
			Title: m.t("Update installed"),
			Body:  m.tf("Restart the launcher to finish updating to %s.", m.updateInstalled),
		}
		m.modal = modalAlert
		return m, nil
	}
	m.updateChecking = true
	m.updateRelease = nil
	m.setToast(m.t("Checking for updates…"), toastInfo)
	return m, checkUpdateCmd(m.client, m.AppVersion)
}

func (m *Model) handleUpdateCheck(msg updateCheckMsg) (tea.Model, tea.Cmd) {
	m.updateChecking = false
	if msg.err != nil {
		title := m.t("Update check failed")
		body := msg.err.Error()
		if errors.Is(msg.err, update.ErrDevelopment) {
			title = m.t("Check for updates")
			body = m.t("Update checks are unavailable for development builds.")
		}
		m.alert = AlertState{Title: title, Body: body}
		m.modal = modalAlert
		return m, nil
	}
	if msg.release == nil {
		m.setToast(m.tf("You are using the latest version: %s", m.AppVersion), toastSuccess)
		return m, nil
	}
	m.updateRelease = msg.release
	m.confirm = ConfirmState{
		Kind:  confirmInstallUpdate,
		Title: m.t("Update available"),
		Body:  m.tf("Version %s is available. Install it now?", msg.release.TagName),
	}
	m.modal = modalConfirm
	return m, nil
}

func (m *Model) startUpdate() (tea.Model, tea.Cmd) {
	if m.updateRelease == nil {
		return m, nil
	}
	if m.task != nil {
		m.alert = AlertState{Title: m.t("Another task is running"), Body: m.t("Wait for the current installation or launch to finish.")}
		m.modal = modalAlert
		return m, nil
	}
	release := *m.updateRelease
	asset, err := update.FindAsset(&release, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		m.alert = AlertState{Title: m.t("Update failed"), Body: err.Error()}
		m.modal = modalAlert
		return m, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	task := &TaskState{
		Kind:      taskUpdate,
		Title:     m.tf("Updating launcher to %s", release.TagName),
		VersionID: release.TagName,
		Stage:     "Downloading update",
		Detail:    asset.Name,
		Started:   time.Now(),
		Cancel:    cancel,
	}
	m.task = task
	m.modal = modalProgress
	client := m.client
	send := m.send

	return m, func() tea.Msg {
		go func() {
			emitter := func(msg tea.Msg) {
				if send != nil {
					send(msg)
				}
			}
			progress := func(p update.Progress) {
				eventType := install.EventStage
				if p.Done > 0 || p.Total > 0 {
					eventType = install.EventProgress
				}
				emitter(installEventMsg{ev: install.Event{
					Type:   eventType,
					Stage:  p.Stage,
					Detail: p.Detail,
					Done:   p.Done,
					Total:  p.Total,
				}})
			}
			err := update.DownloadAndInstall(ctx, client, &release, asset, progress)
			emitter(taskDoneMsg{
				kind:      taskUpdate,
				versionID: release.TagName,
				err:       err,
				cancelled: errors.Is(err, context.Canceled) || ctx.Err() != nil,
			})
		}()
		return nil
	}
}
