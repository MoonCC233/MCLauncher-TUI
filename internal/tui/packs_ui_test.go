package tui

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MoonCC233/MCLauncher-TUI/internal/catalog"
	"github.com/MoonCC233/MCLauncher-TUI/internal/config"
	"github.com/MoonCC233/MCLauncher-TUI/internal/loader"
	"github.com/MoonCC233/MCLauncher-TUI/internal/packs"
)

func packModel(t *testing.T) (*Model, config.Instance) {
	t.Helper()
	m := testModel(t)
	inst := config.Instance{ID: "i1", Name: "Test", VersionID: "1.21.4"}
	m.cfg.Instances = append(m.cfg.Instances, inst)
	return m, inst
}

// seedPack drops a pack file into the instance folder, the way a user would.
func seedPack(t *testing.T, instanceDir, name string) {
	t.Helper()
	if err := os.MkdirAll(packs.Dir(instanceDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packs.Dir(instanceDir), name), []byte("zip"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPacksBrowserOpensAndLists(t *testing.T) {
	m, inst := packModel(t)
	cmd := m.openPacksBrowser(inst)
	if m.modal != modalPacks {
		t.Fatalf("modal = %v, want the pack list", m.modal)
	}
	if cmd == nil {
		t.Error("opening the browser should start a search")
	}
	body := strings.Join(m.packListBody(60), "\n")
	if !strings.Contains(body, m.t("No resource packs installed.")) {
		t.Errorf("empty state missing:\n%s", body)
	}
	if !strings.Contains(body, "Test") {
		t.Errorf("instance name missing:\n%s", body)
	}
}

func TestPacksSearchFlow(t *testing.T) {
	m, inst := packModel(t)
	m.openPacksBrowser(inst)

	project := catalog.Project{ID: "p1", Title: "Fresh Textures", Description: "clean 16x16", Downloads: 4200}
	m.handlePackSearch(packSearchMsg{query: "", projects: []catalog.Project{project}})
	if len(m.packs.results) != 1 || m.packs.loading {
		t.Fatalf("results = %+v loading = %v", m.packs.results, m.packs.loading)
	}
	body := strings.Join(m.packSearchBody(90), "\n")
	for _, want := range []string{"Fresh Textures", "4.2K"} {
		if !strings.Contains(body, want) {
			t.Errorf("search body missing %q:\n%s", want, body)
		}
	}

	// A changed query searches again, the same query opens the version list.
	m.modal = modalPackSearch
	m.packs.query.SetValue("fresh")
	if _, cmd := m.updatePackSearch(tea.KeyMsg{Type: tea.KeyEnter}); cmd == nil {
		t.Error("enter with a changed query should search")
	}
	m.packs.searchedFor = "fresh"
	m.packs.loading = false
	if _, cmd := m.updatePackSearch(tea.KeyMsg{Type: tea.KeyEnter}); cmd == nil {
		t.Error("enter on a result should fetch its versions")
	}

	m.handlePackVersions(packVersionsMsg{project: project, versions: []catalog.Version{{
		ID: "v1", VersionNumber: "1.0", GameVersions: []string{"1.21.4"},
		Files: []catalog.File{{Filename: "Fresh.zip", URL: "https://cdn.modrinth.com/x/Fresh.zip", Size: 1024, Primary: true}},
	}}})
	if m.modal != modalPackVersions {
		t.Fatalf("modal = %v, want the version list", m.modal)
	}
	if body := strings.Join(m.packVersionsBody(80), "\n"); !strings.Contains(body, "1.0") {
		t.Errorf("version list missing the entry:\n%s", body)
	}

	// Esc walks back one level at a time.
	m.updatePackVersions(tea.KeyMsg{Type: tea.KeyEsc})
	if m.modal != modalPackSearch {
		t.Errorf("modal = %v, want the search view", m.modal)
	}
	m.updatePackSearch(tea.KeyMsg{Type: tea.KeyEsc})
	if m.modal != modalPacks {
		t.Errorf("modal = %v, want the pack list", m.modal)
	}
	m.updatePacks(tea.KeyMsg{Type: tea.KeyEsc})
	if m.modal != modalNone {
		t.Errorf("modal = %v, want none", m.modal)
	}
}

func TestPacksToggleEnablesAndDisables(t *testing.T) {
	m, inst := packModel(t)
	dir := m.layout.InstanceDir(inst.ID)
	seedPack(t, dir, "Fresh.zip")

	m.openPacksBrowser(inst)
	if len(m.packs.list) != 1 {
		t.Fatalf("installed packs = %d, want 1", len(m.packs.list))
	}
	if m.packs.list[0].Title != "Fresh" {
		t.Errorf("title = %q, want it derived from the filename", m.packs.list[0].Title)
	}
	if m.packs.enabled["Fresh.zip"] {
		t.Fatal("a pack starts disabled")
	}

	m.toggleSelectedPack()
	if !m.packs.enabled["Fresh.zip"] {
		t.Error("toggling should enable the pack")
	}
	if !packs.Enabled(dir, packs.ResourcePack, "Fresh.zip") {
		t.Error("options.txt was not updated")
	}
	if body := strings.Join(m.packListBody(80), "\n"); !strings.Contains(body, m.t("On")) {
		t.Errorf("list should show the enabled state:\n%s", body)
	}

	m.toggleSelectedPack()
	if m.packs.enabled["Fresh.zip"] {
		t.Error("toggling again should disable the pack")
	}
	if packs.Enabled(dir, packs.ResourcePack, "Fresh.zip") {
		t.Error("options.txt still enables the pack")
	}
}

func TestPacksRemoveDeletesFiles(t *testing.T) {
	m, inst := packModel(t)
	dir := m.layout.InstanceDir(inst.ID)
	seedPack(t, dir, "Fresh.zip")

	m.openPacksBrowser(inst)
	if _, _ = m.updatePacks(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")}); m.modal != modalConfirm {
		t.Fatalf("modal = %v, want the confirmation dialog", m.modal)
	}
	if err := m.removePack("Fresh.zip"); err != nil {
		t.Fatal(err)
	}
	if len(m.packs.list) != 0 {
		t.Error("pack list should be empty after removal")
	}
	if _, err := os.Stat(filepath.Join(packs.Dir(dir), "Fresh.zip")); !os.IsNotExist(err) {
		t.Error("pack file should be deleted")
	}
}

func TestPacksShortcutFromHomeAndInstances(t *testing.T) {
	m, inst := packModel(t)
	seedPack(t, m.layout.InstanceDir(inst.ID), "Fresh.zip")

	// Instances screen: p opens the browser for the highlighted instance.
	m.switchScreen(ScreenInstances)

	for i := range m.cfg.Instances {
		if m.cfg.Instances[i].ID == inst.ID {
			m.instanceCursor = i
		}
	}
	if _, _ = m.updateInstances(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")}); m.modal != modalPacks {
		t.Fatalf("instances: modal = %v, want the pack list", m.modal)
	}
	if m.packs.instanceID != inst.ID {
		t.Errorf("instances: browsing %q, want %q", m.packs.instanceID, inst.ID)
	}
	m.modal = modalNone

	// Home screen: same shortcut for the highlighted recent instance.
	m.switchScreen(ScreenHome)
	m.homeCursor = 0
	if _, _ = m.updateHome(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")}); m.modal != modalPacks {
		t.Fatalf("home: modal = %v, want the pack list", m.modal)
	}
	if m.packs.instanceID == "" {
		t.Error("home: no instance selected")
	}
}

func TestPackKindSwitching(t *testing.T) {
	m, inst := packModel(t)
	dir := m.layout.InstanceDir(inst.ID)
	seedPack(t, dir, "Fresh.zip")
	if err := os.MkdirAll(packs.DirFor(dir, packs.ShaderPack), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packs.DirFor(dir, packs.ShaderPack), "Bright.zip"), []byte("zip"), 0o644); err != nil {
		t.Fatal(err)
	}

	m.openPacksBrowser(inst)
	if m.packs.kind != packs.ResourcePack {
		t.Fatalf("kind = %q, want resource packs", m.packs.kind)
	}
	if len(m.packs.list) != 1 || m.packs.list[0].Title != "Fresh" {
		t.Fatalf("resource packs = %+v", m.packs.list)
	}

	m.cyclePackKind(1)
	if m.packs.kind != packs.ShaderPack {
		t.Fatalf("kind = %q, want shaders", m.packs.kind)
	}
	if len(m.packs.list) != 1 || m.packs.list[0].Title != "Bright" {
		t.Fatalf("shaders = %+v", m.packs.list)
	}

	m.cyclePackKind(1)
	if m.packs.kind != packs.Mod {
		t.Fatalf("kind = %q, want mods", m.packs.kind)
	}
	if body := strings.Join(m.packListBody(80), "\n"); !strings.Contains(body, m.t("Mods need a mod loader")) {
		t.Errorf("mods without a loader should warn:\n%s", body)
	}

	m.cyclePackKind(1)
	if m.packs.kind != packs.ResourcePack {
		t.Fatalf("kind = %q, want it to wrap to resource packs", m.packs.kind)
	}
}

func TestPackImportFromFile(t *testing.T) {
	m, inst := packModel(t)
	m.openPacksBrowser(inst)

	source := filepath.Join(t.TempDir(), "Imported Pack.zip")
	if err := os.WriteFile(source, []byte("PK\x03\x04"), 0o644); err != nil {
		t.Fatal(err)
	}

	m.updatePacks(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})
	if m.modal != modalPrompt || m.prompt.Kind != promptPackPath {
		t.Fatalf("modal = %v kind = %v, want the import prompt", m.modal, m.prompt.Kind)
	}
	m.prompt.Input.SetValue(source)
	m.submitPrompt()

	if len(m.packs.list) != 1 || m.packs.list[0].Title != "Imported Pack" {
		t.Fatalf("list = %+v, want the imported pack", m.packs.list)
	}
	if _, err := os.Stat(filepath.Join(packs.DirFor(m.layout.InstanceDir(inst.ID), packs.ResourcePack), "Imported Pack.zip")); err != nil {
		t.Fatalf("imported file missing: %v", err)
	}
}

func TestPackAddFromURLStartsTask(t *testing.T) {
	m, inst := packModel(t)
	m.openPacksBrowser(inst)

	m.updatePacks(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")})
	if m.modal != modalPrompt || m.prompt.Kind != promptPackURL {
		t.Fatalf("modal = %v kind = %v, want the url prompt", m.modal, m.prompt.Kind)
	}
	m.prompt.Input.SetValue("https://example.com/packs/Fancy.zip")
	if _, cmd := m.submitPrompt(); cmd == nil {
		t.Error("submitting a url should start the download task")
	}
	if m.task == nil || m.task.Kind != taskPack {
		t.Fatalf("task = %+v, want a pack task", m.task)
	}
	if m.task.Detail != "Fancy.zip" {
		t.Errorf("detail = %q, want the file name from the url", m.task.Detail)
	}
	if m.modal != modalProgress {
		t.Errorf("modal = %v, want the progress dialog", m.modal)
	}
	// Do not let the background download outlive the test.
	if m.task.Cancel != nil {
		m.task.Cancel()
	}
	_ = inst
}

func TestPackTabsShowAllKinds(t *testing.T) {
	m, inst := packModel(t)
	m.openPacksBrowser(inst)

	tabs := m.packTabs(80)
	for _, kind := range packs.Kinds {
		label := m.packKindName(kind)
		if !strings.Contains(tabs, label) {
			t.Errorf("tab bar is missing %q: %s", label, tabs)
		}
	}
	if !strings.Contains(tabs, m.t("Resource packs")) {
		t.Errorf("tabs = %q, want the resource pack tab", tabs)
	}

	// The tab bar and the folder are part of the body, so the current kind is
	// always visible instead of hidden behind a toggle.
	body := strings.Join(m.packListBody(80), "\n")
	if !strings.Contains(body, "resourcepacks/") {
		t.Errorf("body is missing the folder:\n%s", body)
	}
	if !strings.Contains(body, m.t("No resource packs installed.")) {
		t.Errorf("body is missing the empty state:\n%s", body)
	}
	if !strings.Contains(body, m.t("Press / to search, i to import a file, u for a link.")) {
		t.Errorf("body is missing the hint:\n%s", body)
	}

	// Switching to shaders updates both the tab bar and the empty state.
	m.setPackKind(packs.ShaderPack)
	body = strings.Join(m.packListBody(80), "\n")
	if !strings.Contains(body, "shaderpacks/") || !strings.Contains(body, m.t("No shaders installed.")) {
		t.Errorf("shader tab body:\n%s", body)
	}

	m.setPackKind(packs.Mod)
	body = strings.Join(m.packListBody(80), "\n")
	if !strings.Contains(body, "mods/") || !strings.Contains(body, m.t("No mods installed.")) {
		t.Errorf("mod tab body:\n%s", body)
	}
}

func TestPackNumberKeysSwitchKind(t *testing.T) {
	m, inst := packModel(t)
	m.openPacksBrowser(inst)

	m.updatePacks(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3")})
	if m.packs.kind != packs.Mod {
		t.Errorf("kind = %q, want mods after pressing 3", m.packs.kind)
	}
	m.updatePacks(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1")})
	if m.packs.kind != packs.ResourcePack {
		t.Errorf("kind = %q, want resource packs after pressing 1", m.packs.kind)
	}
	m.updatePacks(tea.KeyMsg{Type: tea.KeyTab})
	if m.packs.kind != packs.ShaderPack {
		t.Errorf("kind = %q, want shaders after tab", m.packs.kind)
	}
	m.updatePacks(tea.KeyMsg{Type: tea.KeyShiftTab})
	if m.packs.kind != packs.ResourcePack {
		t.Errorf("kind = %q, want resource packs after shift+tab", m.packs.kind)
	}
}

func TestInstancesNoLongerHasAModsPanel(t *testing.T) {
	m, inst := packModel(t)
	m.switchScreen(ScreenInstances)
	for i := range m.cfg.Instances {
		if m.cfg.Instances[i].ID == inst.ID {
			m.instanceCursor = i
		}
	}

	// m used to open a separate mods panel; the content browser replaced it.
	if _, _ = m.updateInstances(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("m")}); m.modal != modalNone {
		t.Fatalf("m should no longer open a panel, modal = %v", m.modal)
	}
	if _, _ = m.updateInstances(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")}); m.modal != modalPacks {
		t.Fatalf("p should still open the browser, modal = %v", m.modal)
	}
}

func TestModTitlesComeFromJarMetadata(t *testing.T) {
	m, inst := packModel(t)
	dir := m.layout.ModsDir(inst.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	jar := filepath.Join(dir, "cool-1.0.jar")
	file, err := os.Create(jar)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	entry, err := writer.Create("fabric.mod.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte(`{"id":"cool","name":"Cool Mod","version":"1.0"}`)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	m.openPacksBrowser(inst)
	m.setPackKind(packs.Mod)
	if len(m.packs.list) != 1 {
		t.Fatalf("mods = %+v", m.packs.list)
	}
	if got := m.packs.list[0].Title; got != "Cool Mod 1.0" {
		t.Errorf("title = %q, want the jar metadata name and version", got)
	}
}

func TestPackSearchUsesMinecraftVersionForLoaderInstances(t *testing.T) {
	m, inst := packModel(t)
	loaderID := "fabric-loader-0.20.1-1.19.4"
	m.cfg.Instances[0].VersionID = loaderID
	m.loaderMeta[loaderID] = loader.Meta{Kind: loader.Fabric, Minecraft: "1.19.4", VersionID: loaderID}
	m.packs.instanceID = inst.ID

	// The loader version id is not a Minecraft version: Modrinth would return
	// nothing for it, so the sidecar's Minecraft version has to be used.
	if got := m.packGameVersion(); got != "1.19.4" {
		t.Errorf("game version = %q, want 1.19.4", got)
	}
	m.packs.kind = packs.Mod
	if got := m.packLoader(); got != "fabric" {
		t.Errorf("loader = %q, want fabric", got)
	}
	m.packs.kind = packs.ShaderPack
	if got := m.packLoader(); got != "" {
		t.Errorf("loader = %q, want no loader filter for shaders", got)
	}
}

func TestPackSearchUsesTheInstanceVersionForVanilla(t *testing.T) {
	m, inst := packModel(t)
	m.packs.instanceID = inst.ID
	if got := m.packGameVersion(); got != "1.21.4" {
		t.Errorf("game version = %q, want the instance version", got)
	}
	m.packs.kind = packs.Mod
	if got := m.packLoader(); got != "" {
		t.Errorf("loader = %q, want none for a vanilla instance", got)
	}
}

func TestPackSourceSwitch(t *testing.T) {
	m, inst := packModel(t)
	m.openPacksBrowser(inst)

	if m.packs.source != catalog.SourceModrinth {
		t.Fatalf("source = %q, want Modrinth by default", m.packs.source)
	}
	if _, cmd := m.updatePacks(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")}); cmd == nil {
		t.Error("changing the source should start a new search")
	}
	if m.packs.source != catalog.SourceCurseForge {
		t.Fatalf("source = %q, want CurseForge after s", m.packs.source)
	}
	if body := strings.Join(m.packListBody(90), "\n"); !strings.Contains(body, "CurseForge") {
		t.Errorf("body should show the active source:\n%s", body)
	}

	// The version view keeps the source of the project it came from.
	project := catalog.Project{ID: "42", Title: "JEI", Source: catalog.SourceCurseForge}
	m.handlePackVersions(packVersionsMsg{project: project, versions: []catalog.Version{{
		ID: "7", VersionNumber: "7.0",
		Files: []catalog.File{{Filename: "jei.jar", Size: 10, Primary: true}},
	}}})
	if body := strings.Join(m.packVersionsBody(90), "\n"); !strings.Contains(body, "CurseForge") {
		t.Errorf("version body should show the project source:\n%s", body)
	}

	m.modal = modalPacks
	if _, cmd := m.updatePacks(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")}); cmd == nil {
		t.Error("changing back should start a new search")
	}
	if m.packs.source != catalog.SourceModrinth {
		t.Errorf("source = %q, want Modrinth after switching back", m.packs.source)
	}
}

func TestPackSourceSearchBodiesShowTheSource(t *testing.T) {
	m, inst := packModel(t)
	m.openPacksBrowser(inst)

	if body := strings.Join(m.packSearchBody(90), "\n"); !strings.Contains(body, "Modrinth") {
		t.Errorf("search body should show the source:\n%s", body)
	}
	m.packs.source = catalog.SourceCurseForge
	if body := strings.Join(m.packSearchBody(90), "\n"); !strings.Contains(body, "CurseForge") {
		t.Errorf("search body should follow the source:\n%s", body)
	}
}

func TestPackDownloadFiltersCycle(t *testing.T) {
	m, inst := packModel(t)
	m.openPacksBrowser(inst)
	m.packs.kind = packs.Mod

	if cmd := m.cyclePackSort(); cmd == nil {
		t.Error("changing the sort should reload the search")
	}
	if m.packs.sort != catalog.SortDownloads {
		t.Errorf("sort = %q, want downloads", m.packs.sort)
	}
	if cmd := m.cyclePackSort(); cmd == nil || m.packs.sort != catalog.SortNewest {
		t.Errorf("sort = %q, want newest", m.packs.sort)
	}

	if cmd := m.cyclePackVersionFilter(); cmd == nil {
		t.Error("changing the version filter should reload the search")
	}
	if m.packs.versionFilter != "all" {
		t.Errorf("version filter = %q, want all", m.packs.versionFilter)
	}
	if got := m.packVersionFilterValue(); got != "" {
		t.Errorf("version value = %q, want no restriction", got)
	}

	if cmd := m.cyclePackLoaderFilter(); cmd == nil {
		t.Error("changing the loader should reload the search")
	}
	if m.packs.loaderFilter != "all" {
		t.Errorf("loader filter = %q, want all", m.packs.loaderFilter)
	}
	if got := m.packLoaderFilterValue(); got != "" {
		t.Errorf("loader value = %q, want no restriction", got)
	}
	m.cyclePackLoaderFilter()
	if m.packs.loaderFilter != "fabric" {
		t.Errorf("loader filter = %q, want fabric", m.packs.loaderFilter)
	}
	if got := m.packLoaderFilterValue(); got != "fabric" {
		t.Errorf("loader value = %q", got)
	}
}

func TestPackPagination(t *testing.T) {
	m, inst := packModel(t)
	m.openPacksBrowser(inst)
	m.packs.total = 45

	if got := m.packPageCount(); got != 3 {
		t.Errorf("page count = %d, want 3", got)
	}
	if cmd := m.changePackPage(1); cmd == nil || m.packs.page != 1 {
		t.Fatalf("page = %d, want the next page", m.packs.page)
	}
	m.packs.page = 2
	if cmd := m.changePackPage(1); cmd != nil {
		t.Error("moving past the last page should do nothing")
	}
	if cmd := m.changePackPage(-3); cmd != nil {
		t.Error("moving before the first page should do nothing")
	}
}

func TestPackSearchTabSwitchesKindWithoutLosingTheQuery(t *testing.T) {
	m, inst := packModel(t)
	m.openPacksBrowser(inst)
	m.modal = modalPackSearch
	m.packs.kind = packs.ResourcePack
	m.packs.query.SetValue("sodium")

	if _, cmd := m.updatePackSearch(tea.KeyMsg{Type: tea.KeyTab}); cmd == nil {
		t.Error("tab should switch kind and reload the search")
	}
	if m.packs.kind != packs.ShaderPack {
		t.Errorf("kind = %q, want shaders", m.packs.kind)
	}
	if got := m.currentPackQuery(); got != "sodium" {
		t.Errorf("query = %q, want it preserved", got)
	}
}

func TestPackSearchBodyShowsTheFilterBar(t *testing.T) {
	m, inst := packModel(t)
	m.openPacksBrowser(inst)
	m.packs.total = 45
	m.packs.page = 1
	m.packs.sort = catalog.SortDownloads

	body := strings.Join(m.packSearchBody(100), "\n")
	for _, want := range []string{
		"Modrinth", "1.21.4", "Downloads", "Page 2 / 3", "45 results",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("filter bar missing %q:\n%s", want, body)
		}
	}
}

func TestPackVersionsBodyShowsProjectDetails(t *testing.T) {
	m, inst := packModel(t)
	m.openPacksBrowser(inst)
	m.packs.project = catalog.Project{
		ID: "42", Title: "Sodium", Author: "CaffeineMC",
		Description: "A modern rendering engine.", Downloads: 1_200_000,
		Source: catalog.SourceCurseForge,
	}
	m.packs.versions = []catalog.Version{{
		ID: "v1", VersionNumber: "1.2.3", GameVersions: []string{"1.21.4"},
		Loaders: []string{"fabric"}, Downloads: 4_200,
		Files: []catalog.File{{Filename: "sodium.jar", Size: 1024, Primary: true}},
	}}
	m.handlePackVersions(packVersionsMsg{project: m.packs.project, versions: m.packs.versions})

	body := strings.Join(m.packVersionsBody(100), "\n")
	for _, want := range []string{"Sodium", "CaffeineMC", "A modern rendering engine.", "CurseForge", "1.2.3", "1.21.4", "fabric", "4.2K"} {
		if !strings.Contains(body, want) {
			t.Errorf("version body missing %q:\n%s", want, body)
		}
	}
}
