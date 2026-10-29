package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MoonCC233/MCLauncher-TUI/internal/paths"
	"github.com/MoonCC233/MCLauncher-TUI/internal/util"
)

// Task statuses recorded in the Downloads history. They double as translation
// keys, so TestTaskStatusesTranslated keeps them in every catalog.
const (
	taskStatusCompleted = "completed"
	taskStatusFailed    = "failed"
	taskStatusCancelled = "cancelled"
)

const taskHistoryLimit = 20

type taskHistoryFile struct {
	Version int          `json:"version"`
	Tasks   []TaskRecord `json:"tasks"`
}

func loadTaskHistory(layout paths.Layout) []TaskRecord {
	var payload taskHistoryFile
	if err := util.ReadJSON(layout.DownloadsHistory(), &payload); err != nil {
		return nil
	}
	tasks := payload.Tasks
	if len(tasks) > taskHistoryLimit {
		tasks = tasks[:taskHistoryLimit]
	}
	return tasks
}

func (m *Model) saveTaskHistory() {
	if m.layout.Root == "" {
		return
	}
	payload := taskHistoryFile{Version: 1, Tasks: m.taskHistory}
	if err := util.WriteJSONAtomic(m.layout.DownloadsHistory(), &payload); err != nil {
		m.addLog("error", "save download history: "+err.Error())
	}
}

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
	case "up", "k":
		m.downloadsScroll--
		m.clampDownloadsScroll()
	case "down", "j":
		m.downloadsScroll++
		m.clampDownloadsScroll()
	case "pgup":
		m.downloadsScroll -= m.downloadsPage()
		m.clampDownloadsScroll()
	case "pgdown":
		m.downloadsScroll += m.downloadsPage()
		m.clampDownloadsScroll()
	case "home":
		m.downloadsScroll = 0
	case "end", "G":
		m.downloadsScroll = len(m.downloadsContent(max(20, m.width)))
		m.clampDownloadsScroll()
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
	if len(m.taskHistory) > taskHistoryLimit {
		m.taskHistory = m.taskHistory[:taskHistoryLimit]
	}
	m.saveTaskHistory()
	m.downloadsScroll = 0
}

// downloadsPage is the number of content rows the Downloads body shows at once.
func (m *Model) downloadsPage() int {
	visible := m.height - 10
	if visible < 4 {
		visible = 4
	}
	return visible
}

func (m *Model) clampDownloadsScroll() {
	content := len(m.downloadsContent(max(20, m.width)))
	m.downloadsScroll = clampInt(m.downloadsScroll, 0, max(0, content-m.downloadsPage()))
}

func (m *Model) viewDownloads(w, h int) []string {
	right := m.t("idle")
	if m.task != nil {
		right = m.trKnown(m.task.Stage)
	}
	header := []string{
		m.sectionHeader(m.t("Downloads"), right, w),
		"",
	}
	content := m.downloadsContent(w)
	visible := h - len(header)
	if visible < 4 {
		visible = 4
	}
	offset := clampInt(m.downloadsScroll, 0, max(0, len(content)-visible))
	m.downloadsScroll = offset
	end := min(len(content), offset+visible)
	return append(header, content[offset:end]...)
}

// downloadsContent renders the active task and the retained history. The view
// windows this list, so a long history stays reachable with the arrow keys.
func (m *Model) downloadsContent(w int) []string {
	var lines []string
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
		return append(lines, styleDim.Render(m.t("Completed tasks will appear here.")))
	}
	for _, record := range m.taskHistory {
		label := record.Title
		status := m.t(record.Status)
		color := styleGreen
		marker := "✓"
		switch record.Status {
		case taskStatusFailed:
			color = styleRed
			marker = "✗"
		case taskStatusCancelled:
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
