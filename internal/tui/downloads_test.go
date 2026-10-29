package tui

import (
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MoonCC233/MCLauncher-TUI/internal/i18n"
	"github.com/MoonCC233/MCLauncher-TUI/internal/util"
)

// TestTaskStatusesTranslated guards the Download history labels: the statuses
// are stored as plain strings and translated at render time, so a missing
// catalog entry would silently fall back to English.
func TestTaskStatusesTranslated(t *testing.T) {
	for _, info := range i18n.Supported {
		if info.Code == i18n.EN {
			continue
		}
		for _, status := range []string{taskStatusCompleted, taskStatusFailed, taskStatusCancelled} {
			if !i18n.Has(info.Code, status) {
				t.Errorf("%s: missing translation for task status %q", info.Code, status)
			}
		}
	}
}

func historyModel(t *testing.T, count int) *Model {
	t.Helper()
	m := testModel(t)
	m.width, m.height = 90, 20
	for i := 0; i < count; i++ {
		m.recordTask(&TaskState{
			Kind:      taskInstall,
			Title:     "task " + strconv.Itoa(i),
			VersionID: "1.21.4",
			Started:   time.Now(),
			Done:      int64(i),
		}, taskStatusCompleted, nil)
	}
	m.task = nil
	return m
}

func TestDownloadsHistoryScrolls(t *testing.T) {
	m := historyModel(t, 20)
	m.switchScreen(ScreenDownloads)

	view := m.viewDownloads(m.width, m.height)
	if len(view) > m.height {
		t.Fatalf("view has %d lines, want at most %d", len(view), m.height)
	}
	top := strings.Join(view, "\n")
	if !strings.Contains(top, "task 19") {
		t.Errorf("first page should show the newest task:\n%s", top)
	}
	if strings.Contains(top, "task 0") {
		t.Errorf("oldest task should be off screen before scrolling:\n%s", top)
	}

	// End jumps to the oldest retained entry.
	m.updateDownloads(tea.KeyMsg{Type: tea.KeyEnd})
	bottom := strings.Join(m.viewDownloads(m.width, m.height), "\n")
	if !strings.Contains(bottom, "task 0") {
		t.Errorf("end should reveal the oldest task:\n%s", bottom)
	}
	if m.downloadsScroll <= 0 {
		t.Errorf("downloadsScroll = %d, want > 0 after scrolling", m.downloadsScroll)
	}

	// Home returns to the newest entry.
	m.updateDownloads(tea.KeyMsg{Type: tea.KeyHome})
	if m.downloadsScroll != 0 {
		t.Errorf("downloadsScroll = %d, want 0", m.downloadsScroll)
	}

	// A fresh record resets the scroll position so new history is visible.
	m.updateDownloads(tea.KeyMsg{Type: tea.KeyEnd})
	m.recordTask(&TaskState{Kind: taskInstall, Title: "task 20", Started: time.Now()}, taskStatusCompleted, nil)
	if m.downloadsScroll != 0 {
		t.Errorf("downloadsScroll = %d, want 0 after recording a task", m.downloadsScroll)
	}
}

func TestTaskHistoryPersists(t *testing.T) {
	m := historyModel(t, 25)
	if len(m.taskHistory) != taskHistoryLimit {
		t.Fatalf("history length = %d, want %d", len(m.taskHistory), taskHistoryLimit)
	}

	var payload taskHistoryFile
	if err := util.ReadJSON(m.layout.DownloadsHistory(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Version != 1 || len(payload.Tasks) != taskHistoryLimit {
		t.Errorf("history file = version %d, %d tasks", payload.Version, len(payload.Tasks))
	}

	reloaded := New(m.cfg, m.layout, nil, "test", "")
	if len(reloaded.taskHistory) != taskHistoryLimit {
		t.Fatalf("reloaded history length = %d, want %d", len(reloaded.taskHistory), taskHistoryLimit)
	}
	first := reloaded.taskHistory[0]
	if first.Title != "task 24" || first.Done != 24 {
		t.Errorf("newest record = %+v, want task 24", first)
	}
	if first.Started.IsZero() || first.Finished.IsZero() {
		t.Errorf("timestamps were not persisted: %+v", first)
	}
	if last := reloaded.taskHistory[len(reloaded.taskHistory)-1]; last.Title != "task 5" {
		t.Errorf("oldest retained record = %+v, want task 5", last)
	}

	// A corrupt history file must not prevent the launcher from starting.
	if err := os.WriteFile(m.layout.DownloadsHistory(), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	corrupt := New(m.cfg, m.layout, nil, "test", "")
	if len(corrupt.taskHistory) != 0 {
		t.Errorf("corrupt history loaded %d records", len(corrupt.taskHistory))
	}
}

func TestDownloadsShowsTranslatedStatus(t *testing.T) {
	m := historyModel(t, 3)
	m.setLanguage(i18n.ZHHans)
	view := strings.Join(m.viewDownloads(m.width, m.height), "\n")
	if !strings.Contains(view, "已完成") {
		t.Errorf("history status should be translated:\n%s", view)
	}
	if strings.Contains(view, taskStatusCompleted) {
		t.Errorf("history status fell back to English:\n%s", view)
	}
}
