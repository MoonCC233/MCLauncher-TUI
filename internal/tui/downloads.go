package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MoonCC233/MCLauncher-TUI/internal/util"
)

func (m *Model) backgroundTask(openDownloads bool) {
	if m.task == nil {
		return
	}
	m.task.Backgrounded = true
	m.modal = modalNone
	if openDownloads {
		m.switchScreen(ScreenDownloads)
	}
	m.setToast(m.t("Task continues in Downloads"), toastInfo)
}

func (m *Model) updateDownloads(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if m.task != nil {
			m.task.Backgrounded = false
			m.modal = modalProgress
		}
		return m, nil
	case "x":
		if m.task != nil {
			m.cancelTask()
			m.setToast(m.t("Cancelling…"), toastInfo)
		}
		return m, nil
	}
	return m, nil
}

func (m *Model) recordTask(task *TaskState, status string, err error) {
	if task == nil {
		return
	}
	record := TaskRecord{
		Kind:       task.Kind,
		Title:      task.Title,
		VersionID:  task.VersionID,
		Stage:      task.Stage,
		Detail:     task.Detail,
		Status:     status,
		Started:    task.Started,
		Finished:   time.Now(),
		Done:       task.Done,
		Total:      task.Total,
		FilesDone:  task.FilesDone,
		FilesTotal: task.FilesTotal,
	}
	if err != nil {
		record.Err = err.Error()
	}
	m.taskHistory = append([]TaskRecord{record}, m.taskHistory...)
	if len(m.taskHistory) > 20 {
		m.taskHistory = m.taskHistory[:20]
	}
}

func (m *Model) viewDownloads(w, h int) []string {
	right := m.t("idle")
	if m.task != nil {
		right = m.trKnown(m.task.Stage)
	}
	lines := []string{
		m.sectionHeader(m.t("Downloads"), right, w),
		"",
	}
	if m.task != nil {
		task := m.task
		lines = append(lines,
			stylePrimaryB.Render(task.Title),
			styleMuted.Render(truncateRunes(task.Detail, max(10, w-4))),
			"",
			m.progressBar(task.Done, task.Total, max(10, w-14))+"  "+styleText.Render(fmt.Sprintf("%3d%%", m.progressPercent())),
		)
		detail := util.HumanDuration(time.Since(task.Started))
		if task.Total > 0 {
			detail = fmt.Sprintf("%s / %s · %s", util.HumanBytes(task.Done), util.HumanBytes(task.Total), detail)
		}
		lines = append(lines, styleDim.Render(detail))
		if task.FilesTotal > 0 {
			lines = append(lines, styleDim.Render(m.tf("%d / %d files", task.FilesDone, task.FilesTotal)))
		}
		if task.Cancelled {
			lines = append(lines, styleYellow.Render(m.t("Cancelling…")))
		}
		lines = append(lines, "", styleDim.Render(m.t("Press Enter to reopen progress, x to cancel.")))
	} else {
		lines = append(lines,
			styleMuted.Render(m.t("No active task.")),
			styleDim.Render(m.t("Installs and game launches run here when sent to the background.")),
		)
	}
	lines = append(lines, "", styleDim.Render(m.t("HISTORY")))
	if len(m.taskHistory) == 0 {
		lines = append(lines, styleDim.Render(m.t("Completed tasks will appear here.")))
		return lines
	}
	for _, record := range m.taskHistory {
		label := record.Title
		status := m.t(record.Status)
		color := styleGreen
		marker := "✓"
		switch record.Status {
		case "failed":
			color = styleRed
			marker = "✗"
		case "cancelled":
			color = styleYellow
			marker = "○"
		}
		meta := status + " · " + m.relTime(record.Finished.Unix())
		if record.Total > 0 {
			meta += " · " + util.HumanBytes(record.Total)
		}
		lines = append(lines,
			fitLine("  "+color.Render(marker+" ")+styleText.Render(label)+"  "+styleDim.Render(meta), w),
		)
		if record.Err != "" {
			lines = append(lines, "    "+styleRed.Render(truncateRunes(record.Err, max(10, w-6))))
		}
	}
	return lines
}
