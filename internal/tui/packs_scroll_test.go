package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MoonCC233/MCLauncher-TUI/internal/catalog"
	"github.com/MoonCC233/MCLauncher-TUI/internal/packs"
)

func TestSearchResultsScrollWithTheCursor(t *testing.T) {
	m, inst := packModel(t)
	m.width, m.height = 100, 24
	m.openPacksBrowser(inst)

	projects := make([]catalog.Project, 0, 20)
	for i := 0; i < 20; i++ {
		projects = append(projects, catalog.Project{ID: fmt.Sprintf("p%02d", i), Title: fmt.Sprintf("Project %02d", i)})
	}
	m.handlePackSearch(packSearchMsg{query: "", projects: projects})

	body := m.packSearchBody(80)
	if len(body) > m.height-6 {
		t.Fatalf("body has %d lines, more than the modal can show", len(body))
	}
	first := strings.Join(body, "\n")
	if !strings.Contains(first, "Project 00") || strings.Contains(first, "Project 19") {
		t.Fatalf("first page = \n%s", first)
	}

	// Moving the cursor down scrolls the window with it.
	m.packs.searchCursor = 19
	body = m.packSearchBody(80)
	last := strings.Join(body, "\n")
	if !strings.Contains(last, "Project 19") {
		t.Fatalf("scrolled page is missing the cursor row:\n%s", last)
	}
	if strings.Contains(last, "Project 00") {
		t.Fatalf("scrolled page still starts at the top:\n%s", last)
	}
	if len(body) > m.height-6 {
		t.Fatalf("body has %d lines after scrolling", len(body))
	}

	// Paging and home/end move the cursor as well.
	m.updatePackSearch(tea.KeyMsg{Type: tea.KeyHome})
	if m.packs.searchCursor != 0 {
		t.Errorf("home -> cursor %d", m.packs.searchCursor)
	}
	m.updatePackSearch(tea.KeyMsg{Type: tea.KeyEnd})
	if m.packs.searchCursor != 19 {
		t.Errorf("end -> cursor %d", m.packs.searchCursor)
	}
	m.updatePackSearch(tea.KeyMsg{Type: tea.KeyPgUp})
	if m.packs.searchCursor >= 19 {
		t.Errorf("pgup -> cursor %d, want it to move up", m.packs.searchCursor)
	}
}

func TestInstalledAndVersionListsScroll(t *testing.T) {
	m, inst := packModel(t)
	m.width, m.height = 100, 20
	packsDir := packs.DirFor(m.layout.InstanceDir(inst.ID), packs.ResourcePack)
	if err := os.MkdirAll(packsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 15; i++ {
		name := filepath.Join(packsDir, fmt.Sprintf("Pack%02d.zip", i))
		if err := os.WriteFile(name, []byte("zip"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m.openPacksBrowser(inst)
	if len(m.packs.list) != 15 {
		t.Fatalf("packs = %d", len(m.packs.list))
	}
	if body := m.packListBody(80); len(body) > m.height-6 {
		t.Fatalf("installed body has %d lines", len(body))
	}
	m.packs.cursor = len(m.packs.list) - 1
	body := strings.Join(m.packListBody(80), "\n")
	if !strings.Contains(body, "Pack14") {
		t.Fatalf("last pack is not visible after scrolling:\n%s", body)
	}

	versions := make([]catalog.Version, 0, 15)
	for i := 0; i < 15; i++ {
		versions = append(versions, catalog.Version{
			ID:            fmt.Sprintf("v%02d", i),
			VersionNumber: fmt.Sprintf("1.0.%d", i),
			Files:         []catalog.File{{Filename: "x.zip", Size: 10, Primary: true}},
		})
	}
	m.handlePackVersions(packVersionsMsg{project: catalog.Project{Title: "Fresh"}, versions: versions})
	if m.modal != modalPackVersions {
		t.Fatalf("modal = %v", m.modal)
	}
	if body := m.packVersionsBody(80); len(body) > m.height-6 {
		t.Fatalf("versions body has %d lines", len(body))
	}
	m.packs.versionCursor = len(versions) - 1
	body = strings.Join(m.packVersionsBody(80), "\n")
	if !strings.Contains(body, "1.0.14") {
		t.Fatalf("last version is not visible after scrolling:\n%s", body)
	}
}

func TestWheelScrollsThePackModals(t *testing.T) {
	m, inst := packModel(t)
	m.openPacksBrowser(inst)
	m.handlePackSearch(packSearchMsg{query: "", projects: []catalog.Project{
		{ID: "a", Title: "A"}, {ID: "b", Title: "B"}, {ID: "c", Title: "C"},
	}})

	m.modal = modalPackSearch
	m.scrollBy(1)
	if m.packs.searchCursor != 1 {
		t.Errorf("wheel down -> cursor %d, want 1", m.packs.searchCursor)
	}
	m.scrollBy(-1)
	if m.packs.searchCursor != 0 {
		t.Errorf("wheel up -> cursor %d, want 0", m.packs.searchCursor)
	}

	m.modal = modalPackVersions
	m.packs.versions = []catalog.Version{{ID: "v1"}, {ID: "v2"}}
	m.scrollBy(1)
	if m.packs.versionCursor != 1 {
		t.Errorf("versions cursor = %d, want 1", m.packs.versionCursor)
	}
}
