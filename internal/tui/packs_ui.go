package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/MoonCC233/MCLauncher-TUI/internal/catalog"
	"github.com/MoonCC233/MCLauncher-TUI/internal/config"
	"github.com/MoonCC233/MCLauncher-TUI/internal/install"
	"github.com/MoonCC233/MCLauncher-TUI/internal/loader"
	"github.com/MoonCC233/MCLauncher-TUI/internal/mods"
	"github.com/MoonCC233/MCLauncher-TUI/internal/packs"
	"github.com/MoonCC233/MCLauncher-TUI/internal/util"
)

// packsState is the content browser of one instance: resource packs, shaders and
// mods, either installed locally or downloaded from Modrinth, CurseForge or any
// https link.
type packsState struct {
	instanceID   string
	instanceName string
	kind         packs.Kind
	source       catalog.Source

	list    []packs.Record
	cursor  int
	enabled map[string]bool

	query        textinput.Model
	results      []catalog.Project
	searchCursor int
	loading      bool
	searchedFor  string
	// sort, page and the filter overrides drive the download center. Empty
	// filters follow the instance's game version and loader.
	sort          catalog.Sort
	page          int
	total         int
	versionFilter string
	loaderFilter  string

	project       catalog.Project
	versions      []catalog.Version
	versionCursor int
}

// packPageSize is how many search hits one page holds.
const packPageSize = 20

// packInstallPlan describes a catalog install that still has to resolve its
// required dependencies in the background.
type packInstallPlan struct {
	source      catalog.Source
	version     catalog.Version
	gameVersion string
	loader      string
	kind        packs.Kind
}

// packFile is one file the task has to download.
type packFile struct {
	record packs.Record
	url    string
}

// packSearchMsg carries one page of catalog search results.
type packSearchMsg struct {
	query    string
	source   catalog.Source
	sort     catalog.Sort
	page     int
	total    int
	projects []catalog.Project
	err      error
}

// packVersionsMsg carries the installable versions of the chosen project.
type packVersionsMsg struct {
	project  catalog.Project
	source   catalog.Source
	versions []catalog.Version
	err      error
}

// openPacksBrowser opens the content browser for an instance.
func (m *Model) openPacksBrowser(inst config.Instance) tea.Cmd {
	m.packs.instanceID = inst.ID
	m.packs.instanceName = inst.Name
	m.packs.kind = packs.ResourcePack
	m.packs.source = catalog.SourceModrinth
	m.packs.sort = catalog.SortRelevance
	m.packs.page = 0
	m.packs.total = 0
	m.packs.versionFilter = ""
	m.packs.loaderFilter = ""
	m.packs.cursor = 0
	m.packs.query = newTextInput("", "", 48, 64)
	m.packs.query.Focus()
	m.packs.searchedFor = ""
	m.packs.results = nil
	m.packs.searchCursor = 0
	m.packs.versions = nil
	m.packs.versionCursor = 0
	m.reloadPacks()
	m.modal = modalPacks
	return m.searchPacksCmd("")
}

// reloadPacks refreshes the installed list and its enabled state. For mods the
// jar metadata supplies a readable name and version.
func (m *Model) reloadPacks() {
	dir := m.layout.InstanceDir(m.packs.instanceID)
	m.packs.list = packs.List(dir, m.packs.kind)
	if m.packs.kind == packs.Mod {
		m.decorateMods()
	}
	m.packs.enabled = make(map[string]bool, len(m.packs.list))
	for _, record := range m.packs.list {
		m.packs.enabled[record.Filename] = packs.Enabled(dir, m.packs.kind, record.Filename)
	}
	m.packs.cursor = clampInt(m.packs.cursor, 0, max(0, len(m.packs.list)-1))
}

// decorateMods replaces file names with the name and version inside the jar.
func (m *Model) decorateMods() {
	items, err := mods.Scan(m.layout.ModsDir(m.packs.instanceID))
	if err != nil || len(items) == 0 {
		return
	}
	byName := make(map[string]mods.Mod, len(items))
	for _, item := range items {
		byName[strings.ToLower(item.FileName)] = item
	}
	for i := range m.packs.list {
		item, ok := byName[strings.ToLower(m.packs.list[i].Filename)]
		if !ok || item.Name == "" {
			continue
		}
		title := item.Name
		if item.Version != "" {
			title += " " + item.Version
		}
		m.packs.list[i].Title = title
	}
}

// packKindName is the label of a content kind.
func (m *Model) packKindName(kind packs.Kind) string {
	switch kind {
	case packs.ShaderPack:
		return m.t("Shaders")
	case packs.Mod:
		return m.t("Mods")
	default:
		return m.t("Resource packs")
	}
}

// setPackKind selects one content kind and refreshes its list.
func (m *Model) setPackKind(kind packs.Kind) tea.Cmd {
	if kind == m.packs.kind {
		return nil
	}
	m.packs.kind = kind
	m.packs.cursor = 0
	m.packs.results = nil
	m.packs.searchedFor = ""
	m.packs.page = 0
	m.packs.total = 0
	m.packs.loaderFilter = ""
	m.packs.searchCursor = 0
	m.reloadPacks()
	return m.searchPacksCmd("")
}

// cyclePackKind switches between resource packs, shaders and mods.
func (m *Model) cyclePackKind(delta int) tea.Cmd {
	current := 0
	for i, kind := range packs.Kinds {
		if kind == m.packs.kind {
			current = i
		}
	}
	next := (current + delta + len(packs.Kinds)) % len(packs.Kinds)
	return m.setPackKind(packs.Kinds[next])
}

// cyclePackKindFromSearch switches the content kind without leaving the
// download center or throwing away the query.
func (m *Model) cyclePackKindFromSearch(delta int) tea.Cmd {
	current := 0
	for i, kind := range packs.Kinds {
		if kind == m.packs.kind {
			current = i
		}
	}
	next := (current + delta + len(packs.Kinds)) % len(packs.Kinds)
	m.packs.kind = packs.Kinds[next]
	m.packs.page = 0
	m.packs.total = 0
	m.packs.loaderFilter = ""
	m.packs.searchCursor = 0
	m.reloadPacks()
	return m.searchPacksCmd(m.currentPackQuery())
}

// packSourceName is the catalog shown in the browser.
func (m *Model) packSourceName() string {
	return catalog.NormalizeSource(m.packs.source).Label()
}

// cyclePackSource switches between Modrinth and CurseForge and repeats the
// current search against the new catalog.
func (m *Model) cyclePackSource(delta int) tea.Cmd {
	current := 0
	for i, source := range catalog.Sources {
		if source == m.packs.source {
			current = i
		}
	}
	next := (current + delta + len(catalog.Sources)) % len(catalog.Sources)
	m.packs.source = catalog.Sources[next]
	m.packs.results = nil
	m.packs.searchedFor = ""
	m.packs.versions = nil
	m.packs.page = 0
	m.packs.total = 0
	m.packs.searchCursor = 0
	m.packs.versionCursor = 0
	m.setToast(m.tf("Source: %s", m.packSourceName()), toastInfo)
	query := strings.TrimSpace(m.packs.query.Value())
	return m.searchPacksCmd(query)
}

// packVersionFilterValue is the game version sent to the catalog.
func (m *Model) packVersionFilterValue() string {
	if m.packs.versionFilter == "all" {
		return ""
	}
	return m.packGameVersion()
}

// packVersionFilterLabel is the game version shown in the filter bar.
func (m *Model) packVersionFilterLabel() string {
	if m.packs.versionFilter == "all" || m.packGameVersion() == "" {
		return m.t("All versions")
	}
	return m.packGameVersion()
}

// packLoaderFilterValue is the loader sent to the catalog. Non-mod content has
// no loader filter.
func (m *Model) packLoaderFilterValue() string {
	if m.packs.kind != packs.Mod {
		return ""
	}
	switch m.packs.loaderFilter {
	case "all":
		return ""
	case "":
		return m.packLoader()
	default:
		return m.packs.loaderFilter
	}
}

// packLoaderFilterLabel is the loader shown in the filter bar.
func (m *Model) packLoaderFilterLabel() string {
	if m.packs.kind != packs.Mod {
		return ""
	}
	value := m.packLoaderFilterValue()
	if value == "" {
		return m.t("All loaders")
	}
	return loaderKindName(loader.Kind(value))
}

// packSortName is the localized ordering label.
func (m *Model) packSortName() string {
	switch catalog.NormalizeSort(m.packs.sort) {
	case catalog.SortDownloads:
		return m.t("Downloads")
	case catalog.SortNewest:
		return m.t("Newest")
	case catalog.SortUpdated:
		return m.t("Updated")
	default:
		return m.t("Relevance")
	}
}

// packPageCount is the number of result pages for the current query.
func (m *Model) packPageCount() int {
	if m.packs.total <= 0 {
		return 1
	}
	pages := (m.packs.total + packPageSize - 1) / packPageSize
	if pages < 1 {
		pages = 1
	}
	return pages
}

func (m *Model) currentPackQuery() string { return strings.TrimSpace(m.packs.query.Value()) }

// cyclePackSort changes the ordering and reloads the first page.
func (m *Model) cyclePackSort() tea.Cmd {
	current := 0
	for i, sort := range catalog.Sorts {
		if sort == catalog.NormalizeSort(m.packs.sort) {
			current = i
		}
	}
	m.packs.sort = catalog.Sorts[(current+1)%len(catalog.Sorts)]
	m.packs.page = 0
	m.setToast(m.tf("Sort: %s", m.packSortName()), toastInfo)
	return m.searchPacksCmd(m.currentPackQuery())
}

// cyclePackVersionFilter toggles between the instance version and every version.
func (m *Model) cyclePackVersionFilter() tea.Cmd {
	if m.packs.versionFilter == "all" {
		m.packs.versionFilter = ""
	} else {
		m.packs.versionFilter = "all"
	}
	m.packs.page = 0
	m.setToast(m.tf("Version: %s", m.packVersionFilterLabel()), toastInfo)
	return m.searchPacksCmd(m.currentPackQuery())
}

// cyclePackLoaderFilter walks the loader chips. It is a no-op outside mods.
func (m *Model) cyclePackLoaderFilter() tea.Cmd {
	if m.packs.kind != packs.Mod {
		m.setToast(m.t("Loaders apply to mods only"), toastInfo)
		return nil
	}
	choices := []string{"", "all"}
	for _, kind := range []string{"fabric", "quilt", "forge", "neoforge"} {
		if kind != m.packLoader() {
			choices = append(choices, kind)
		}
	}
	current := 0
	for i, choice := range choices {
		if choice == m.packs.loaderFilter {
			current = i
		}
	}
	m.packs.loaderFilter = choices[(current+1)%len(choices)]
	m.packs.page = 0
	m.setToast(m.tf("Loader: %s", m.packLoaderFilterLabel()), toastInfo)
	return m.searchPacksCmd(m.currentPackQuery())
}

// changePackPage moves the result page and reloads it.
func (m *Model) changePackPage(delta int) tea.Cmd {
	next := m.packs.page + delta
	if next < 0 || next >= m.packPageCount() {
		return nil
	}
	m.packs.page = next
	return m.searchPacksCmd(m.currentPackQuery())
}

// packLoaderMeta is the loader sidecar of the instance, when it uses a loader.
func (m *Model) packLoaderMeta() (loader.Meta, bool) {
	inst := m.cfg.FindInstance(m.packs.instanceID)
	if inst == nil {
		return loader.Meta{}, false
	}
	meta, ok := m.loaderMeta[inst.VersionID]
	return meta, ok
}

// packGameVersion is the Minecraft version the catalog is filtered by. Loader
// instances are versioned like "fabric-loader-0.20.1-1.19.4", which is not a
// Minecraft version at all: asking Modrinth for it returns nothing, so the
// version from the loader sidecar is used instead.
func (m *Model) packGameVersion() string {
	inst := m.cfg.FindInstance(m.packs.instanceID)
	if inst == nil {
		return ""
	}
	if meta, ok := m.packLoaderMeta(); ok && meta.Minecraft != "" {
		return meta.Minecraft
	}
	return inst.VersionID
}

// packLoader is the mod loader of the instance, used to filter mod results.
func (m *Model) packLoader() string {
	if m.packs.kind != packs.Mod {
		return ""
	}
	if meta, ok := m.packLoaderMeta(); ok {
		return string(meta.Kind)
	}
	return ""
}

func (m *Model) packProjectType() string {
	switch m.packs.kind {
	case packs.ShaderPack:
		return catalog.ProjectTypeShader
	case packs.Mod:
		return catalog.ProjectTypeMod
	default:
		return catalog.ProjectTypeResourcePack
	}
}

func (m *Model) searchPacksCmd(query string) tea.Cmd {
	gameVersion := m.packVersionFilterValue()
	projectType := m.packProjectType()
	loader := m.packLoaderFilterValue()
	source := catalog.NormalizeSource(m.packs.source)
	sort := catalog.NormalizeSort(m.packs.sort)
	page := m.packs.page
	client := catalog.NewClient(m.client)
	m.packs.loading = true
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result, err := client.SearchCatalog(ctx, catalog.SearchOptions{
			Source:      source,
			ProjectType: projectType,
			Query:       query,
			GameVersion: gameVersion,
			Loader:      loader,
			Sort:        sort,
			Limit:       packPageSize,
			Offset:      page * packPageSize,
		})
		if err != nil {
			return packSearchMsg{query: query, source: source, sort: sort, page: page, err: err}
		}
		return packSearchMsg{
			query:    query,
			source:   source,
			sort:     sort,
			page:     page,
			total:    result.TotalHits,
			projects: result.Hits,
		}
	}
}

func (m *Model) packVersionsCmd(project catalog.Project) tea.Cmd {
	gameVersion := m.packVersionFilterValue()
	loader := m.packLoaderFilterValue()
	source := catalog.SourceForProject(project)
	if !project.Source.Valid() {
		source = catalog.NormalizeSource(m.packs.source)
	}
	client := catalog.NewClient(m.client)
	m.packs.loading = true
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		versions, err := client.VersionsForSource(ctx, source, project.ID, gameVersion, loader)
		return packVersionsMsg{project: project, source: source, versions: versions, err: err}
	}
}

func (m *Model) handlePackSearch(msg packSearchMsg) (tea.Model, tea.Cmd) {
	if msg.source != "" {
		// A slower response from a catalog, filter or page the player already
		// moved away from must not replace the current results.
		if msg.source != m.packs.source {
			return m, nil
		}
		if msg.sort != "" && msg.sort != catalog.NormalizeSort(m.packs.sort) {
			return m, nil
		}
		if msg.page != m.packs.page {
			return m, nil
		}
	}
	m.packs.loading = false
	m.packs.searchedFor = msg.query
	if msg.total == 0 && len(msg.projects) > 0 {
		// Hand-built messages (tests, previews) may omit the catalog total.
		msg.total = len(msg.projects)
	}
	m.packs.total = msg.total
	m.packs.page = msg.page
	if msg.err != nil {
		m.packs.results = nil
		m.setToast(msg.err.Error(), toastError)
		return m, nil
	}
	m.packs.results = msg.projects
	m.packs.searchCursor = 0
	return m, nil
}

func (m *Model) handlePackVersions(msg packVersionsMsg) (tea.Model, tea.Cmd) {
	if msg.source != "" && msg.source != m.packs.source {
		return m, nil
	}
	m.packs.loading = false
	if msg.err != nil {
		m.setToast(msg.err.Error(), toastError)
		return m, nil
	}
	if len(msg.versions) == 0 {
		m.setToast(m.t("No results."), toastInfo)
		return m, nil
	}
	m.packs.project = msg.project
	m.packs.versions = msg.versions
	m.packs.versionCursor = 0
	m.modal = modalPackVersions
	return m, nil
}

// updatePacks handles the installed list.
func (m *Model) updatePacks(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.modal = modalNone
		return m, nil
	case "up", "k":
		m.packs.cursor--
	case "down", "j":
		m.packs.cursor++
	case "pgup":
		m.packs.cursor -= m.modalListRows(6)
	case "pgdown":
		m.packs.cursor += m.modalListRows(6)
	case "home":
		m.packs.cursor = 0
	case "end":
		m.packs.cursor = len(m.packs.list) - 1
	case "enter":
		return m.toggleSelectedPack()
	case "d":
		return m.confirmRemovePack()
	case "t", "tab", "right", "l":
		return m, m.cyclePackKind(1)
	case "shift+tab", "left", "h":
		return m, m.cyclePackKind(-1)
	case "1", "2", "3":
		index := int(msg.String()[0] - '1')
		if index >= 0 && index < len(packs.Kinds) {
			return m, m.setPackKind(packs.Kinds[index])
		}
		return m, nil
	case "s":
		return m, m.cyclePackSource(1)
	case "i":
		m.openPrompt(promptPackPath, m.t("Import a file"), "", m.t("Path to a .zip or .jar file you downloaded"), 260)
		return m, nil
	case "u":
		m.openPrompt(promptPackURL, m.t("Add from a URL"), "", m.t("https link to a .zip or .jar file"), 400)
		return m, nil
	case "/":
		m.modal = modalPackSearch
		m.packs.query.SetValue("")
		m.packs.query.Focus()
		return m, m.searchPacksCmd("")
	}
	m.packs.cursor = clampInt(m.packs.cursor, 0, max(0, len(m.packs.list)-1))
	return m, nil
}

func (m *Model) toggleSelectedPack() (tea.Model, tea.Cmd) {
	if m.packs.cursor < 0 || m.packs.cursor >= len(m.packs.list) {
		return m, nil
	}
	record := m.packs.list[m.packs.cursor]
	dir := m.layout.InstanceDir(m.packs.instanceID)
	if m.packs.kind == packs.ShaderPack {
		m.setToast(m.t("Shaders are selected in game"), toastInfo)
		return m, nil
	}
	want := !m.packs.enabled[record.Filename]
	if _, err := packs.SetEnabled(dir, m.packs.kind, record.Filename, want); err != nil {
		m.alert = AlertState{Title: m.packKindName(m.packs.kind), Body: err.Error()}
		m.modal = modalAlert
		return m, nil
	}
	m.reloadPacks()
	if want {
		m.setToast(m.tf("Enabled %s", record.Title), toastSuccess)
	} else {
		m.setToast(m.tf("Disabled %s", record.Title), toastInfo)
	}
	return m, nil
}

func (m *Model) confirmRemovePack() (tea.Model, tea.Cmd) {
	if m.packs.cursor < 0 || m.packs.cursor >= len(m.packs.list) {
		return m, nil
	}
	record := m.packs.list[m.packs.cursor]
	m.confirm = ConfirmState{
		Kind:   confirmRemovePack,
		Title:  m.t("Remove resource pack?"),
		Body:   record.Title,
		Target: record.Filename,
	}
	m.modal = modalConfirm
	return m, nil
}

// removePack deletes the item after the confirmation dialog.
func (m *Model) removePack(filename string) error {
	dir := m.layout.InstanceDir(m.packs.instanceID)
	if err := packs.Remove(dir, m.packs.kind, filename); err != nil {
		return err
	}
	m.reloadPacks()
	m.setToast(m.tf("Removed %s", filename), toastSuccess)
	return nil
}

// updatePackSearch handles typing and result navigation.
func (m *Model) updatePackSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.modal = modalPacks
		return m, nil
	case "tab":
		return m, m.cyclePackKindFromSearch(1)
	case "shift+tab":
		return m, m.cyclePackKindFromSearch(-1)
	case "up":
		m.packs.searchCursor = clampInt(m.packs.searchCursor-1, 0, max(0, len(m.packs.results)-1))
		return m, nil
	case "down":
		m.packs.searchCursor = clampInt(m.packs.searchCursor+1, 0, max(0, len(m.packs.results)-1))
		return m, nil
	case "pgup":
		m.packs.searchCursor = clampInt(m.packs.searchCursor-max(1, m.modalListRows(4)/3), 0, max(0, len(m.packs.results)-1))
		return m, nil
	case "pgdown":
		m.packs.searchCursor = clampInt(m.packs.searchCursor+max(1, m.modalListRows(4)/3), 0, max(0, len(m.packs.results)-1))
		return m, nil
	case "home":
		m.packs.searchCursor = 0
		return m, nil
	case "end":
		m.packs.searchCursor = max(0, len(m.packs.results)-1)
		return m, nil
	case "ctrl+t", "ctrl+s":
		return m, m.cyclePackSource(1)
	case "ctrl+o":
		return m, m.cyclePackSort()
	case "ctrl+f":
		return m, m.cyclePackVersionFilter()
	case "ctrl+k":
		return m, m.cyclePackLoaderFilter()
	case "ctrl+p":
		return m, m.changePackPage(-1)
	case "ctrl+n":
		return m, m.changePackPage(1)
	case "ctrl+r":
		return m, m.searchPacksCmd(m.currentPackQuery())
	case "enter":
		if m.packs.loading {
			return m, nil
		}
		query := strings.TrimSpace(m.packs.query.Value())
		if query != m.packs.searchedFor || len(m.packs.results) == 0 {
			m.packs.page = 0
			return m, m.searchPacksCmd(query)
		}
		project := m.packs.results[clampInt(m.packs.searchCursor, 0, len(m.packs.results)-1)]
		return m, m.packVersionsCmd(project)
	}
	var cmd tea.Cmd
	m.packs.query, cmd = m.packs.query.Update(msg)
	return m, cmd
}

// updatePackVersions installs the highlighted version.
func (m *Model) updatePackVersions(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.modal = modalPackSearch
		return m, nil
	case "up", "k":
		m.packs.versionCursor--
	case "down", "j":
		m.packs.versionCursor++
	case "pgup":
		m.packs.versionCursor -= max(1, m.modalListRows(6)/2)
	case "pgdown":
		m.packs.versionCursor += max(1, m.modalListRows(6)/2)
	case "home":
		m.packs.versionCursor = 0
	case "end":
		m.packs.versionCursor = len(m.packs.versions) - 1
	case "enter":
		return m.startPackInstall()
	}
	m.packs.versionCursor = clampInt(m.packs.versionCursor, 0, max(0, len(m.packs.versions)-1))
	return m, nil
}

// startPackInstall downloads the selected catalog file as a background task.
func (m *Model) startPackInstall() (tea.Model, tea.Cmd) {
	if m.task != nil {
		m.alert = AlertState{Title: m.t("Another task is running"), Body: m.t("Wait for the current installation to finish.")}
		m.modal = modalAlert
		return m, nil
	}
	if len(m.packs.versions) == 0 {
		return m, nil
	}
	version := m.packs.versions[clampInt(m.packs.versionCursor, 0, max(0, len(m.packs.versions)-1))]
	file, ok := version.PrimaryFile()
	if !ok {
		return m, nil
	}
	title := m.packs.project.Title
	if title == "" {
		title = version.Name
	}
	source := catalog.SourceForProject(m.packs.project)
	if !m.packs.project.Source.Valid() {
		source = catalog.NormalizeSource(m.packs.source)
	}
	dependencyVersion := m.packGameVersion()
	if len(version.GameVersions) > 0 {
		dependencyVersion = version.GameVersions[0]
	}
	dependencyLoader := m.packLoader()
	if len(version.Loaders) > 0 {
		dependencyLoader = version.Loaders[0]
	}
	record := packs.Record{
		ProjectID: m.packs.project.ID,
		VersionID: version.ID,
		Title:     title,
		Filename:  file.Filename,
		SHA512:    file.Hashes["sha512"],
		SHA1:      file.Hashes["sha1"],
		Size:      file.Size,
		Source:    string(source),
	}
	plan := &packInstallPlan{
		source:      source,
		version:     version,
		gameVersion: dependencyVersion,
		loader:      dependencyLoader,
		kind:        m.packs.kind,
	}
	return m.startPackTask(title, record, file.URL, plan)
}

// startPackURL installs a file from any https link the player pasted.
func (m *Model) startPackURL(rawURL string) (tea.Model, tea.Cmd) {
	if m.task != nil {
		m.alert = AlertState{Title: m.t("Another task is running"), Body: m.t("Wait for the current installation to finish.")}
		m.modal = modalAlert
		return m, nil
	}
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return m, nil
	}
	name := filepath.Base(strings.SplitN(rawURL, "?", 2)[0])
	record := packs.Record{Filename: name, Title: strings.TrimSuffix(name, filepath.Ext(name))}
	return m.startPackTask(record.Title, record, rawURL, nil)
}

// startPackImport copies a file the player downloaded from another site.
func (m *Model) startPackImport(path string) (tea.Model, tea.Cmd) {
	path = strings.TrimSpace(path)
	if path == "" {
		return m, nil
	}
	dir := m.layout.InstanceDir(m.packs.instanceID)
	record, err := packs.ImportFile(dir, m.packs.kind, path)
	if err != nil {
		m.alert = AlertState{Title: m.packKindName(m.packs.kind), Body: err.Error()}
		m.modal = modalAlert
		return m, nil
	}
	m.reloadPacks()
	m.modal = modalPacks
	m.setToast(m.tf("Imported %s", record.Title), toastSuccess)
	return m, nil
}

// startPackTask runs one download in the background task slot. Catalog
// installs resolve their required dependencies first and then download every
// file in order.
func (m *Model) startPackTask(title string, record packs.Record, rawURL string, plan *packInstallPlan) (tea.Model, tea.Cmd) {
	ctx, cancel := context.WithCancel(context.Background())
	stage := m.tf("Installing %s", title)
	resolveStage := m.t("Resolving dependencies")
	m.task = &TaskState{
		Kind:         taskPack,
		Title:        stage,
		VersionID:    record.VersionID,
		Stage:        stage,
		Detail:       record.Filename,
		ContentTitle: title,
		Total:        record.Size,
		FilesTotal:   1,
		Started:      time.Now(),
		Cancel:       cancel,
	}
	m.modal = modalProgress
	m.addLog("launcher", fmt.Sprintf("content %s (%s)", title, m.packs.kind))

	layout := m.layout
	instanceID := m.packs.instanceID
	kind := m.packs.kind
	client := m.client
	send := m.send
	return m, func() tea.Msg {
		go func() {
			emitter := func(msg tea.Msg) {
				if send != nil {
					send(msg)
				}
			}
			files := []packFile{{record: record, url: rawURL}}
			if plan != nil && plan.kind == packs.Mod {
				emitter(installEventMsg{ev: install.Event{
					Type:       install.EventStage,
					Stage:      resolveStage,
					Detail:     record.Filename,
					FilesTotal: 1,
				}})
				dependencies, _ := catalog.NewClient(client).ResolveRequired(ctx, plan.source, plan.version, plan.gameVersion, plan.loader)
				for _, dependency := range dependencies {
					file, ok := dependency.PrimaryFile()
					if !ok {
						continue
					}
					dependencyTitle := dependency.Name
					if dependencyTitle == "" {
						name := packs.SanitizeFilename(file.Filename)
						dependencyTitle = strings.TrimSuffix(name, filepath.Ext(name))
					}
					files = append(files, packFile{
						record: packs.Record{
							ProjectID: dependency.ProjectID,
							VersionID: dependency.ID,
							Title:     dependencyTitle,
							Filename:  file.Filename,
							SHA512:    file.Hashes["sha512"],
							SHA1:      file.Hashes["sha1"],
							Size:      file.Size,
							Source:    string(plan.source),
						},
						url: file.URL,
					})
				}
			}

			emitter(installEventMsg{ev: install.Event{
				Type:       install.EventStage,
				Stage:      stage,
				Detail:     files[0].record.Filename,
				Total:      files[0].record.Size,
				FilesTotal: len(files),
			}})
			dir := layout.InstanceDir(instanceID)
			for i, file := range files {
				if err := ctx.Err(); err != nil {
					emitter(taskDoneMsg{kind: taskPack, err: err, cancelled: true})
					return
				}
				onProgress := func(done int64) {
					emitter(installEventMsg{ev: install.Event{
						Type:       install.EventProgress,
						Stage:      stage,
						Detail:     file.record.Filename,
						Done:       done,
						Total:      file.record.Size,
						FilesDone:  i,
						FilesTotal: len(files),
					}})
				}
				var err error
				if plan != nil {
					_, err = packs.Install(ctx, client, dir, kind, file.record, file.url, onProgress)
				} else {
					_, err = packs.InstallURL(ctx, client, dir, kind, file.record, file.url, onProgress)
				}
				if err != nil {
					emitter(taskDoneMsg{kind: taskPack, err: err, cancelled: ctx.Err() != nil})
					return
				}
				emitter(installEventMsg{ev: install.Event{
					Type:       install.EventProgress,
					Stage:      stage,
					Detail:     file.record.Filename,
					Done:       file.record.Size,
					Total:      file.record.Size,
					FilesDone:  i + 1,
					FilesTotal: len(files),
				}})
			}
			emitter(taskDoneMsg{kind: taskPack, cancelled: ctx.Err() != nil})
		}()
		return nil
	}
}

// packTabs renders the visible type selector: the active type is highlighted so
// the three content kinds are not hidden behind a single toggle key.
func (m *Model) packTabs(contentW int) string {
	parts := make([]string, 0, len(packs.Kinds))
	for _, kind := range packs.Kinds {
		label := m.packKindName(kind)
		if kind == m.packs.kind {
			parts = append(parts, pill(label, colPrimary))
			continue
		}
		parts = append(parts, styleMuted.Render(" "+label+" "))
	}
	return "  " + strings.Join(parts, " ")
}

// packFolderLabel is the folder the current kind is read from.
func (m *Model) packFolderLabel() string {
	return m.packs.kind.Folder() + "/"
}

// packEmptyText is the empty state of the current kind.
func (m *Model) packEmptyText() string {
	switch m.packs.kind {
	case packs.ShaderPack:
		return m.t("No shaders installed.")
	case packs.Mod:
		return m.t("No mods installed.")
	default:
		return m.t("No resource packs installed.")
	}
}

// modalListRows is how many list rows fit under n header lines inside a modal.
// The modal body is clipped to height-6 lines, so the list has to be windowed
// instead of appended whole.
func (m *Model) modalListRows(header int) int {
	rows := m.height - 6 - header
	if rows < 3 {
		rows = 3
	}
	return rows
}

// windowIndex returns the slice of a list that keeps the cursor visible.
func windowIndex(cursor, total, visible int) (int, int) {
	if total <= 0 {
		return 0, 0
	}
	if visible >= total {
		return 0, total
	}
	cursor = clampInt(cursor, 0, total-1)
	start := cursor - visible/2
	if start < 0 {
		start = 0
	}
	if start > total-visible {
		start = total - visible
	}
	return start, start + visible
}

// packListBody renders the installed items.
func (m *Model) packListBody(contentW int) []string {
	body := []string{
		m.packTabs(contentW),
		"",
		styleDim.Render("  " + m.packs.instanceName + " · " + m.packFolderLabel() + " · " + m.tf("%d installed", len(m.packs.list))),
		styleDim.Render("  " + m.tf("Source: %s", m.packSourceName())),
		"",
	}
	if m.packs.kind == packs.Mod && m.packLoader() == "" {
		body = append(body, styleYellow.Render("  "+m.t("Mods need a mod loader")), "")
	}
	if len(m.packs.list) == 0 {
		body = append(body,
			styleMuted.Render("  "+m.packEmptyText()),
			styleDim.Render("  "+m.t("Press / to search, i to import a file, u for a link.")),
		)
		return body
	}
	start, end := windowIndex(m.packs.cursor, len(m.packs.list), m.modalListRows(len(body)))
	for i := start; i < end; i++ {
		record := m.packs.list[i]
		selected := i == m.packs.cursor
		marker := "  "
		if selected {
			marker = stylePrimary.Render("▸ ")
		}
		state := m.t("Off")
		if m.packs.enabled[record.Filename] {
			state = m.t("On")
		}
		right := state + " · " + util.HumanBytes(record.Size)
		switch record.Source {
		case "modrinth":
			right += " · MR"
		case "curseforge":
			right += " · CF"
		}
		body = append(body, javaPickerRow(marker, record.Title, right, m.packs.enabled[record.Filename], selected, contentW))
	}
	return body
}

// packFilterChips renders the source, version, loader and sort filters used by
// the download center, in the compact two-row form.
func (m *Model) packFilterChips(contentW int, includePage bool) []string {
	chip := func(label, value string) string {
		return styleMuted.Render(label+": ") + stylePrimary.Render(value)
	}
	first := []string{
		chip(m.t("source"), m.packSourceName()),
		chip(m.t("version"), m.packVersionFilterLabel()),
	}
	if m.packs.kind == packs.Mod {
		first = append(first, chip(m.t("loader"), m.packLoaderFilterLabel()))
	}
	second := []string{chip(m.t("sort"), m.packSortName())}
	if includePage {
		second = append(second,
			styleMuted.Render(m.tf("Page %d / %d", m.packs.page+1, m.packPageCount())),
			styleMuted.Render(m.tf("%d results", m.packs.total)),
		)
	}
	return []string{
		fitLine("  "+strings.Join(first, "   "), contentW),
		fitLine("  "+strings.Join(second, "   "), contentW),
	}
}

// compactCount shortens large download counts without losing their magnitude.
func compactCount(n int64) string {
	switch {
	case n >= 1_000_000_000:
		return fmt.Sprintf("%.1fB", float64(n)/1_000_000_000)
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// packResultCard renders one search hit as a three-line card: the title and
// download count, the author and source, then the summary.
func (m *Model) packResultCard(project catalog.Project, selected bool, contentW int) []string {
	marker := "  "
	titleStyle := styleText
	if selected {
		marker = stylePrimary.Render("▸ ")
		titleStyle = stylePrimaryB
	}
	title := truncateRunes(project.Title, max(8, contentW-24))
	right := "↓ " + compactCount(project.Downloads)
	first := composeLR("", marker+titleStyle.Render(title), styleDim.Render(right), contentW)

	meta := make([]string, 0, 2)
	if project.Author != "" {
		meta = append(meta, project.Author)
	}
	meta = append(meta, catalog.SourceForProject(project).Label())
	second := fitLine("    "+styleMuted.Render(strings.Join(meta, " · ")), contentW)

	description := truncateRunes(project.Description, max(8, contentW-6))
	third := fitLine("    "+styleDim.Render(description), contentW)
	return []string{first, second, third}
}

// packSearchBody renders the query line, filters and result cards.
func (m *Model) packSearchBody(contentW int) []string {
	body := []string{"  " + m.packs.query.View()}
	body = append(body, m.packFilterChips(contentW, true)...)
	body = append(body, "")
	if m.packs.loading {
		body = append(body, "  "+stylePrimary.Render(spinnerFrame(m.spinner))+" "+styleMuted.Render(m.t("Searching…")))
		return body
	}
	if len(m.packs.results) == 0 {
		body = append(body, styleMuted.Render("  "+m.t("No results.")))
		return body
	}
	header := len(body)
	rows := m.modalListRows(header)
	showPage := rows >= 4
	pageRows := 0
	if showPage {
		pageRows = 1
	}
	visible := max(1, (rows-pageRows)/3)
	start, end := windowIndex(m.packs.searchCursor, len(m.packs.results), visible)
	for i := start; i < end; i++ {
		body = append(body, m.packResultCard(m.packs.results[i], i == m.packs.searchCursor, contentW)...)
	}
	if showPage {
		page := m.tf("Page %d / %d", m.packs.page+1, m.packPageCount())
		body = append(body, fitLine("  "+styleDim.Render(page), contentW))
	}
	return body
}

// packVersionRow renders one installable version with its metadata on a second
// line, so version, game version and loader are visible together.
func (m *Model) packVersionRow(version catalog.Version, selected bool, contentW int) []string {
	marker := "  "
	if selected {
		marker = stylePrimary.Render("▸ ")
	}
	label := version.VersionNumber
	if label == "" {
		label = version.Name
	}
	file, _ := version.PrimaryFile()
	first := javaPickerRow(marker, label, util.HumanBytes(file.Size), false, selected, contentW)

	meta := make([]string, 0, 3)
	if len(version.GameVersions) > 0 {
		meta = append(meta, strings.Join(version.GameVersions, ", "))
	}
	if len(version.Loaders) > 0 {
		meta = append(meta, strings.Join(version.Loaders, ", "))
	}
	if version.Downloads > 0 {
		meta = append(meta, "↓ "+compactCount(version.Downloads))
	}
	return []string{first, fitLine("    "+styleDim.Render(strings.Join(meta, " · ")), contentW)}
}

// packVersionsBody renders the project intro and its version list.
func (m *Model) packVersionsBody(contentW int) []string {
	project := m.packs.project
	source := catalog.SourceForProject(project)
	title := truncateRunes(project.Title, max(8, contentW-24))
	body := []string{
		fitLine("  "+styleBold.Render(title)+"  "+pill(source.Label(), colPrimary), contentW),
	}
	meta := make([]string, 0, 2)
	if project.Author != "" {
		meta = append(meta, project.Author)
	}
	if project.Downloads > 0 {
		meta = append(meta, "↓ "+compactCount(project.Downloads))
	}
	if len(meta) > 0 {
		body = append(body, fitLine("  "+styleMuted.Render(strings.Join(meta, " · ")), contentW))
	}
	if project.Description != "" {
		lines := wrapPlain(project.Description, max(12, contentW-4))
		for i, line := range lines {
			if i >= 2 {
				break
			}
			body = append(body, fitLine("  "+styleDim.Render(line), contentW))
		}
	}
	body = append(body, m.packFilterChips(contentW, false)...)
	body = append(body,
		fitLine("  "+styleDim.Render(m.tf("%d versions", len(m.packs.versions))), contentW),
		"",
	)

	header := len(body)
	visible := max(1, m.modalListRows(header)/2)
	start, end := windowIndex(m.packs.versionCursor, len(m.packs.versions), visible)
	for i := start; i < end; i++ {
		body = append(body, m.packVersionRow(m.packs.versions[i], i == m.packs.versionCursor, contentW)...)
	}
	return body
}
