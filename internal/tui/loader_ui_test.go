package tui

import (
	"testing"

	"github.com/MoonCC233/MCLauncher-TUI/internal/loader"
)

func TestChooseLoaderVersionFocusesLastField(t *testing.T) {
	m := newTestModel(t)
	m.openInstanceForm("1.21.4")
	m.form.Loader = loader.Fabric
	m.form.LoaderVersions = demoLoaderVersions()
	m.form.LoaderVersion = "0.16.10"
	m.syncFormChoices()
	m.openLoaderVersionPickerState()
	m.loaderVerPick = 1

	m.chooseLoaderVersion()

	if m.modal != modalForm {
		t.Fatalf("modal = %v, want modalForm", m.modal)
	}
	if want := len(m.form.Fields) - 1; m.form.Focus != want {
		t.Fatalf("form focus = %d, want last field %d", m.form.Focus, want)
	}
}
