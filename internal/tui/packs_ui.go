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
// mods, either installed locally or downloaded from Modrinth or any https link.
type packsState struct {
	instanceID   string
	instanceName string
	kind         packs.Kind

	list    []packs.Record
	cursor  int
	enabled map[string]bool

	query        textinput.Model
	results      []catalog.Project
	searchCursor int
	loading      bool
	searchedFor  string

	project       catalog.Project
	versions      []catalog.Version
	versionCursor int
}

// packSearchMsg carries one page of Modrinth search results.
type packSearchMsg struct {
	query    string
	projects []catalog.Project
	err      error
}

// packVersionsMsg carries the installable versions of the chosen project.
type packVersionsMsg struct {
	project  catalog.Project
	versions []catalog.Version
	err      error
}

// openPacksBrowser opens the content browser for an instance.
func (m *Model) openPacksBrowser(inst config.Instance) tea.Cmd {
	m.packs.instanceID = inst.ID
	m.packs.instanceName = inst.Name
	m.packs.kind = packs.ResourcePack
	m.packs.cursor = 0
	m.packs.query = newTextInput("", "", 36, 48)
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
	gameVersion := m.packGameVersion()
	projectType := m.packProjectType()
	loader := m.packLoader()
	client := catalog.NewClient(m.client)
	m.packs.loading = true
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result, err := client.Search(ctx, projectType, query, gameVersion, loader, 20, 0)
		if err != nil {
			return packSearchMsg{query: query, err: err}
		}
		return packSearchMsg{query: query, projects: result.Hits}
	}
}

func (m *Model) packVersionsCmd(project catalog.Project) tea.Cmd {
	gameVersion := m.packGameVersion()
	loader := m.packLoader()
	client := catalog.NewClient(m.client)
	m.packs.loading = true
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		versions, err := client.VersionsFor(ctx, project.ID, gameVersion, loader)
		return packVersionsMsg{project: project, versions: versions, err: err}
	}
}

func (m *Model) handlePackSearch(msg packSearchMsg) (tea.Model, tea.Cmd) {
	m.packs.loading = false
	m.packs.searchedFor = msg.query
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
	case "up":
		m.packs.searchCursor = clampInt(m.packs.searchCursor-1, 0, max(0, len(m.packs.results)-1))
		return m, nil
	case "down":
		m.packs.searchCursor = clampInt(m.packs.searchCursor+1, 0, max(0, len(m.packs.results)-1))
		return m, nil
	case "pgup":
		m.packs.searchCursor = clampInt(m.packs.searchCursor-m.modalListRows(4), 0, max(0, len(m.packs.results)-1))
		return m, nil
	case "pgdown":
		m.packs.searchCursor = clampInt(m.packs.searchCursor+m.modalListRows(4), 0, max(0, len(m.packs.results)-1))
		return m, nil
	case "home":
		m.packs.searchCursor = 0
		return m, nil
	case "end":
		m.packs.searchCursor = max(0, len(m.packs.results)-1)
		return m, nil
	case "enter":
		if m.packs.loading {
			return m, nil
		}
		query := strings.TrimSpace(m.packs.query.Value())
		if query != m.packs.searchedFor || len(m.packs.results) == 0 {
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
		m.packs.versionCursor -= m.modalListRows(4)
	case "pgdown":
		m.packs.versionCursor += m.modalListRows(4)
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
	record := packs.Record{
		ProjectID: m.packs.project.ID,
		VersionID: version.ID,
		Title:     title,
		Filename:  file.Filename,
		SHA512:    file.Hashes["sha512"],
		Size:      file.Size,
	}
	return m.startPackTask(title, record, file.URL)
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
	return m.startPackTask(record.Title, record, rawURL)
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

// startPackTask runs one download in the background task slot.
func (m *Model) startPackTask(title string, record packs.Record, rawURL string) (tea.Model, tea.Cmd) {
	ctx, cancel := context.WithCancel(context.Background())
	stage := m.tf("Installing %s", title)
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
			onProgress := func(done int64) {
				emitter(installEventMsg{ev: install.Event{
					Type:   install.EventProgress,
					Stage:  stage,
					Detail: record.Filename,
					Done:   done,
					Total:  record.Size,
				}})
			}
			dir := layout.InstanceDir(instanceID)
			_, err := packs.InstallURL(ctx, client, dir, kind, record, rawURL, onProgress)
			emitter(taskDoneMsg{kind: taskPack, err: err, cancelled: ctx.Err() != nil})
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
		body = append(body, javaPickerRow(marker, record.Title, right, m.packs.enabled[record.Filename], selected, contentW))
	}
	return body
}

// packSearchBody renders the query line and the search results.
func (m *Model) packSearchBody(contentW int) []string {
	body := []string{"  " + m.packs.query.View(), ""}
	if m.packs.loading {
		body = append(body, "  "+stylePrimary.Render(spinnerFrame(m.spinner))+" "+styleMuted.Render(m.t("Searching…")))
		return body
	}
	if len(m.packs.results) == 0 {
		body = append(body, styleMuted.Render("  "+m.t("No results.")))
		return body
	}
	start, end := windowIndex(m.packs.searchCursor, len(m.packs.results), m.modalListRows(len(body)))
	for i := start; i < end; i++ {
		project := m.packs.results[i]
		selected := i == m.packs.searchCursor
		marker := "  "
		if selected {
			marker = stylePrimary.Render("▸ ")
		}
		label := project.Title
		if project.Description != "" {
			label += styleDim.Render("  " + truncateRunes(project.Description, max(8, contentW-42)))
		}
		right := m.tf("%d downloads", project.Downloads)
		body = append(body, javaPickerRow(marker, label, right, false, selected, contentW))
	}
	return body
}

// packVersionsBody renders the version list of the chosen project.
func (m *Model) packVersionsBody(contentW int) []string {
	body := []string{styleDim.Render("  " + m.packs.project.Title), ""}
	start, end := windowIndex(m.packs.versionCursor, len(m.packs.versions), m.modalListRows(len(body)))
	for i := start; i < end; i++ {
		version := m.packs.versions[i]
		selected := i == m.packs.versionCursor
		marker := "  "
		if selected {
			marker = stylePrimary.Render("▸ ")
		}
		file, _ := version.PrimaryFile()
		label := version.VersionNumber
		if len(version.GameVersions) > 0 {
			label += styleDim.Render("  " + strings.Join(version.GameVersions, ", "))
		}
		body = append(body, javaPickerRow(marker, label, util.HumanBytes(file.Size), false, selected, contentW))
	}
	return body
}
