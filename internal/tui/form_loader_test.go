package tui

import (
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MoonCC233/MCLauncher-TUI/internal/loader"
)

func TestInstanceFormHasLoaderRow(t *testing.T) {
	m := testModel(t)
	m.openInstanceForm("1.21.4")
	if !m.form.LoaderRow {
		t.Fatal("a vanilla version should offer the loader row")
	}
	if got := m.formRowCount(); got != len(m.form.Fields)+1 {
		t.Errorf("form rows = %d, want %d", got, len(m.form.Fields)+1)
	}
	if got := m.formLoaderValue(); got != m.t("None") {
		t.Errorf("default loader = %q, want None", got)
	}

	// The row is reached with tab and wraps back to the first field.
	m.form.Focus = len(m.form.Fields) - 1
	m.focusFormRow(1)
	if !m.formLoaderFocus() {
		t.Fatal("tab should reach the loader row")
	}
	m.focusFormRow(1)
	if m.form.Focus != 0 {
		t.Errorf("focus = %d, want it to wrap to the first field", m.form.Focus)
	}
}

func TestInstanceFormSkipsLoaderForLoaderVersion(t *testing.T) {
	m := testModel(t)
	m.loaderMeta["fabric-1.21.4"] = loader.Meta{Kind: loader.Fabric, Minecraft: "1.21.4"}
	m.openInstanceForm("fabric-1.21.4")
	if m.form.LoaderRow {
		t.Error("a loader version should not offer another loader")
	}
}

func TestLoaderRowOpensThePicker(t *testing.T) {
	m := testModel(t)
	m.openInstanceForm("1.21.4")
	m.form.Fields[0].SetValue("Modded")
	m.form.Focus = len(m.form.Fields) // the loader row

	model, cmd := m.updateForm(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Errorf("opening the picker should not start a background command yet")
	}
	next := model.(*Model)
	if next.modal != modalLoader {
		t.Fatalf("modal = %v, want the loader picker", next.modal)
	}
	if next.loaderPick.Step != loaderStepKind {
		t.Errorf("step = %v, want the loader kind step", next.loaderPick.Step)
	}
	if next.loaderPick.Minecraft != "1.21.4" {
		t.Errorf("minecraft = %q, want 1.21.4", next.loaderPick.Minecraft)
	}
	if next.pendingCreate == nil || next.pendingCreate.name != "Modded" {
		t.Fatalf("pending create = %+v, want the instance to be remembered", next.pendingCreate)
	}

	// Leaving the form without a valid name must not open the picker.
	m2 := testModel(t)
	m2.openInstanceForm("1.21.4")
	m2.form.Fields[0].SetValue("")
	m2.form.Focus = len(m2.form.Fields)
	if _, _ = m2.updateForm(tea.KeyMsg{Type: tea.KeyEnter}); m2.modal != modalForm {
		t.Errorf("modal = %v, want to stay in the form with an error", m2.modal)
	}
	if m2.form.Err == "" {
		t.Error("an empty name should set a form error")
	}
	if m2.pendingCreate != nil {
		t.Error("no instance should be pending")
	}
}

func TestPendingCreateMakesInstance(t *testing.T) {
	m := testModel(t)
	m.cfg.Path = filepath.Join(t.TempDir(), "config.json")
	m.pendingCreate = &pendingCreate{name: "Modded", base: "1.21.4"}

	m.finishPendingCreate("fabric-loader-0.16.9-1.21.4")

	if m.pendingCreate != nil {
		t.Error("pending create should be cleared")
	}
	inst := m.cfg.FindInstance(m.cfg.Instances[len(m.cfg.Instances)-1].ID)
	if inst == nil {
		t.Fatal("no instance was created")
	}
	if inst.Name != "Modded" {
		t.Errorf("name = %q, want Modded", inst.Name)
	}
	if inst.VersionID != "fabric-loader-0.16.9-1.21.4" {
		t.Errorf("version = %q, want the loader version", inst.VersionID)
	}
	if m.screen != ScreenInstances {
		t.Errorf("screen = %v, want the instance list", m.screen)
	}
}

func TestCancellingThePickerReturnsToTheForm(t *testing.T) {
	m := testModel(t)
	m.openInstanceForm("1.21.4")
	m.form.Fields[0].SetValue("Modded")
	m.form.Focus = len(m.form.Fields)
	m.openLoaderForForm()

	if m.modal != modalLoader {
		t.Fatalf("modal = %v, want the loader picker", m.modal)
	}
	m.updateLoaderPicker(tea.KeyMsg{Type: tea.KeyEsc})
	if m.modal != modalForm {
		t.Errorf("modal = %v, want to be back in the form", m.modal)
	}
	if m.pendingCreate != nil {
		t.Error("cancelling should forget the pending instance")
	}
}
