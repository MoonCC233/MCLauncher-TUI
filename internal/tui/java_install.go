package tui

import (
	"context"
	"errors"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MoonCC233/MCLauncher-TUI/internal/install"
	"github.com/MoonCC233/MCLauncher-TUI/internal/java"
)

func (m *Model) startJavaDownload(major int) (tea.Model, tea.Cmd) {
	if m.task != nil {
		m.alert = AlertState{Title: m.t("Another task is running"), Body: m.t("Wait for the current installation or launch to finish.")}
		m.modal = modalAlert
		return m, nil
	}
	if m.game != nil {
		m.alert = AlertState{Title: m.t("Java runtime"), Body: m.t("Stop the game before installing Java.")}
		m.modal = modalAlert
		return m, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	task := &TaskState{
		Kind:      taskJava,
		Title:     m.tf("Installing Java %d", major),
		VersionID: fmt.Sprintf("Java %d", major),
		Stage:     "Resolving Java runtime",
		Detail:    "Adoptium Temurin JDK",
		Started:   time.Now(),
		Cancel:    cancel,
	}
	m.task = task
	m.modal = modalProgress
	client := m.client
	root := m.layout.Java
	send := m.send

	return m, func() tea.Msg {
		go func() {
			emitter := func(msg tea.Msg) {
				if send != nil {
					send(msg)
				}
			}
			progress := func(p java.Progress) {
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
			info, err := java.InstallLatest(ctx, client, root, major, progress)
			path := ""
			if info != nil {
				path = info.Path
			}
			emitter(taskDoneMsg{
				kind:      taskJava,
				versionID: path,
				err:       err,
				cancelled: errors.Is(err, context.Canceled) || ctx.Err() != nil,
			})
		}()
		return nil
	}
}
