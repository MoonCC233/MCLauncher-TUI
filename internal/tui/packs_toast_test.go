package tui

import (
	"strings"
	"testing"

	"github.com/MoonCC233/MCLauncher-TUI/internal/packs"
)

// Downloading a mod used to report "resource pack installed", because the pack
// task only knew that some content had finished.
func TestPackTaskToastNamesTheContent(t *testing.T) {
	m, inst := packModel(t)
	m.openPacksBrowser(inst)
	m.setPackKind(packs.Mod)
	m.packs.instanceID = inst.ID

	if _, cmd := m.startPackTask("Sodium 0.6.13", packs.Record{Filename: "sodium.jar"}, "https://example.test/sodium.jar"); cmd == nil {
		t.Fatal("starting a pack task should return a command")
	}
	if m.task == nil {
		t.Fatal("no task was created")
	}
	if !strings.Contains(m.task.Stage, "Sodium 0.6.13") {
		t.Errorf("stage = %q, want it to name the mod", m.task.Stage)
	}
	if strings.Contains(m.task.Stage, "resource pack") {
		t.Errorf("stage = %q, still mentions resource packs", m.task.Stage)
	}

	m.toast = ToastState{}
	if _, _ = m.handleTaskDone(taskDoneMsg{kind: taskPack}); m.toast.Kind != toastSuccess {
		t.Fatalf("toast kind = %v, want success", m.toast.Kind)
	}
	if !strings.Contains(m.toast.Text, "Sodium 0.6.13") {
		t.Errorf("toast = %q, want it to name the mod", m.toast.Text)
	}
	if strings.Contains(strings.ToLower(m.toast.Text), "resource pack") {
		t.Errorf("toast = %q, still reports a resource pack", m.toast.Text)
	}
}
