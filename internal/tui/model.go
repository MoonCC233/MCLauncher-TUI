package tui

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/MoonCC233/MCLauncher-TUI/internal/config"
	"github.com/MoonCC233/MCLauncher-TUI/internal/i18n"
	"github.com/MoonCC233/MCLauncher-TUI/internal/install"
	"github.com/MoonCC233/MCLauncher-TUI/internal/java"
	"github.com/MoonCC233/MCLauncher-TUI/internal/launch"
	"github.com/MoonCC233/MCLauncher-TUI/internal/loader"
	"github.com/MoonCC233/MCLauncher-TUI/internal/mojang"
	"github.com/MoonCC233/MCLauncher-TUI/internal/paths"
	"github.com/MoonCC233/MCLauncher-TUI/internal/update"
	"github.com/MoonCC233/MCLauncher-TUI/internal/util"
)

type Screen int

const (
	ScreenHome Screen = iota
	ScreenInstances
	ScreenVersions
	ScreenAccounts
	ScreenSettings
	ScreenConsole
	ScreenDownloads
	ScreenAbout
	screenCount
)

var screenNameKeys = [screenCount]string{"Home", "Instances", "Versions", "Accounts", "Settings", "Console", "Downloads", "About"}
var screenIcons = [screenCount]string{"⌂", "▤", "◈", "◉", "⚙", "▸", "↓", "✦"}

type settingsRow struct {
	ID          string
	Label       string
	Value       string
	Description string
}

type modalKind int

const (
	modalNone modalKind = iota
	modalHelp
	modalForm
	modalConfirm
	modalPrompt
	modalJava
	modalProgress
	modalAlert
	modalLanguage
	modalAccountAdd
	modalMicrosoft
	modalResolution
	modalTheme
	modalTitleAnim
	modalTitleFont
	modalLoader
	modalPacks
	modalPackSearch
	modalPackVersions
	modalAnnounce
	modalSkin
)

type formKind int

const (
	formInstance formKind = iota
	formAccount
)

type promptKind int

const (
	promptMemory promptKind = iota
	promptJVMArgs
	promptResolution
	promptThreads
	promptJavaPath
	promptDataDir
	promptMicrosoftClientID
	promptThemeAccent
	promptModPath
	promptOptiFineJar
	promptGameWindowTitle
	promptPackPath
	promptPackURL
	promptSkinPath
)

type confirmKind int

const (
	confirmDeleteInstance confirmKind = iota
	confirmRemoveAccount
	confirmUninstallVersion
	confirmInstallUpdate
	confirmRemovePack
	confirmRemoveSkin
)

type taskKind string

const (
	taskInstall taskKind = "install"
	taskLaunch  taskKind = "launch"
	taskLoader  taskKind = "loader"
	taskUpdate  taskKind = "update"
	taskJava    taskKind = "java"
	taskPack    taskKind = "pack"
)

type toastKind int

const (
	toastInfo toastKind = iota
	toastSuccess
	toastError
)

type LogLine struct {
	At     time.Time
	Source string
	Text   string
}

type TaskState struct {
	Kind            taskKind
	Title           string
	VersionID       string
	InstanceID      string
	CreateName      string
	CreateMemory    int
	LoaderKind      loader.Kind
	LoaderMinecraft string
	LoaderVersion   string
	LoaderJar       string
	Backgrounded    bool
	Stage           string
	Detail          string
	Done            int64
	Total           int64
	FilesDone       int
	FilesTotal      int
	Started         time.Time
	Cancel          context.CancelFunc
	Cancelled       bool
	Err             error
}

type TaskRecord struct {
	Kind       taskKind  `json:"kind"`
	Title      string    `json:"title"`
	VersionID  string    `json:"version_id,omitempty"`
	Stage      string    `json:"stage,omitempty"`
	Detail     string    `json:"detail,omitempty"`
	Status     string    `json:"status"`
	Started    time.Time `json:"started"`
	Finished   time.Time `json:"finished"`
	Done       int64     `json:"done,omitempty"`
	Total      int64     `json:"total,omitempty"`
	FilesDone  int       `json:"files_done,omitempty"`
	FilesTotal int       `json:"files_total,omitempty"`
	Err        string    `json:"error,omitempty"`
}

type FormState struct {
	Kind      formKind
	Title     string
	VersionID string
	EditID    string
	Fields    []textinput.Model
	Labels    []string
	Hints     []string
	Focus     int
	Err       string

	// LoaderRow shows the "Mod loader" choice of the new-instance form.
	LoaderRow bool
	Loader    loader.Kind
}

type PromptState struct {
	Kind  promptKind
	Title string
	Help  string
	Err   string
	Input textinput.Model
}

type ConfirmState struct {
	Kind   confirmKind
	Title  string
	Body   string
	Target string
	Aux    string
}

type loaderPicker struct {
	Step          int
	Minecraft     string
	Kind          loader.Kind
	Cursor        int
	Versions      []loader.RemoteVersion
	LoaderVersion string
	OptiFineJar   string
	Loading       bool
	Err           string
}

type AlertState struct {
	Title  string
	Body   string
	Themed bool
}

type GameState struct {
	Session      *launch.Session
	InstanceID   string
	InstanceName string
	VersionID    string
	Started      time.Time
	Pid          int
	Stopping     bool
}

type ToastState struct {
	Text  string
	Kind  toastKind
	Until time.Time
}

type Model struct {
	cfg         *config.Config
	layout      paths.Layout
	client      *http.Client
	send        func(tea.Msg)
	AppVersion  string
	DataRoot    string
	StartupNote string

	width  int
	height int
	ready  bool

	screen        Screen
	sidebarCursor int

	manifest        *mojang.Manifest
	manifestLoading bool
	manifestErr     string
	manifestAt      time.Time
	installed       map[string]bool
	loaderMeta      map[string]loader.Meta

	// pendingCreate is set while a new instance waits for its mod loader.
	pendingCreate *pendingCreate

	homeCursor     int
	instanceCursor int
	instanceOffset int
	versionCursor  int
	versionOffset  int
	accountCursor  int
	accountOffset  int
	settingsCursor int

	versionScope int // 0 all, 1 releases, 2 snapshots, 3 ancient, 4 April Fools
	filter       string
	filterActive bool
	search       textinput.Model
	visible      []mojang.VersionSummary

	accountTypePick int
	resolutionPick  int
	themePick       int
	titleAnimPick   int
	titleFontPick   int
	packs           packsState
	announce        announceState
	about           aboutState
	ms              MicrosoftLoginState
	skinModal       skinModalState

	javaRuntimes []java.Info
	javaScanned  bool
	javaScanning bool
	javaPicker   int

	lang         i18n.Lang
	languagePick int

	lastClick  time.Time
	lastClickX int
	lastClickY int

	modalX int
	modalY int
	modalW int
	modalH int

	modal           modalKind
	helpScroll      int
	form            FormState
	prompt          PromptState
	confirm         ConfirmState
	alert           AlertState
	task            *TaskState
	taskHistory     []TaskRecord
	loaderPick      loaderPicker
	updateChecking  bool
	updateRelease   *update.Release
	updateInstalled string

	downloadsScroll int

	hits []hitRegion

	game *GameState

	logs      []LogLine
	logScroll int
	logFollow bool

	toast   ToastState
	spinner int
	clock   int
	// animFrame advances a few times per second and drives the wordmark sheen.
	// animRunning tracks whether a tick is already scheduled.
	animFrame   int
	animRunning bool

	quit bool
}

func New(cfg *config.Config, layout paths.Layout, client *http.Client, appVersion, notice string) *Model {
	if client == nil {
		client = util.NewHTTPClient()
	}
	applyTheme(cfg.Settings.Theme, cfg.Settings.ThemeAccent)
	m := &Model{
		cfg:             cfg,
		layout:          layout,
		client:          client,
		AppVersion:      appVersion,
		DataRoot:        layout.Root,
		StartupNote:     notice,
		lang:            i18n.Resolve(cfg.Settings.Language),
		screen:          ScreenHome,
		installed:       install.InstalledSet(layout),
		loaderMeta:      loader.Scan(layout),
		taskHistory:     loadTaskHistory(layout),
		manifestLoading: true,
		versionScope:    0,
		logFollow:       true,
		search:          textinput.New(),
	}
	m.search.Placeholder = m.t("search versions")
	m.search.Prompt = "/ "
	m.search.CharLimit = 48
	m.search.PromptStyle = styleSearchPrompt
	m.search.TextStyle = styleSearchText
	m.search.PlaceholderStyle = styleSearchPlaceholder
	m.search.Cursor.Style = styleSearchCursor
	m.addLog("system", "mclauncher "+appVersion)
	if notice != "" {
		m.addLog("error", notice)
		m.setToast(m.trKnown(notice), toastError)
	}
	return m
}

func (m *Model) SetSender(send func(tea.Msg)) { m.send = send }

func (m *Model) sendMsg(msg tea.Msg) {
	if m.send != nil {
		m.send(msg)
	}
}

func (m *Model) Init() tea.Cmd {
	m.javaScanning = true
	return tea.Batch(m.loadManifestCmd(false), scanJavaCmd(m.layout.Java), tickCmd(), m.loadAnnouncementsCmd(false))
}

func (m *Model) quitApp() (tea.Model, tea.Cmd) {
	m.quit = true
	if m.task != nil && m.task.Cancel != nil {
		m.task.Cancel()
	}
	m.stopGameOnQuit()
	return m, tea.Quit
}

// stopGameOnQuit stops Minecraft unless the player asked to keep it running.
func (m *Model) stopGameOnQuit() {
	if !m.stopsGameOnQuit() {
		return
	}
	m.stopGameForQuit()
}

// Shutdown stops any game process owned by the launcher. It is safe to call
// after the Bubble Tea program has returned.
func (m *Model) Shutdown() {
	m.stopGameOnQuit()
}

// stopGameForQuit makes sure no Minecraft process is left behind when the
// launcher exits. Stop() is graceful; fall back to Kill() so a stuck JVM does
// not survive the launcher.
func (m *Model) stopGameForQuit() {
	if m.game == nil || m.game.Session == nil {
		return
	}
	if !m.game.Stopping {
		m.addLog("launcher", "stopping Minecraft before exit…")
	}
	m.game.Stopping = true
	session := m.game.Session
	if session.Pid() == 0 {
		return
	}
	stopped := make(chan struct{})
	go func() {
		_ = session.Stop()
		close(stopped)
	}()
	select {
	case <-session.Done():
		return
	case <-stopped:
	case <-time.After(2 * time.Second):
	}
	select {
	case <-session.Done():
	case <-time.After(time.Second):
		_ = session.Kill()
		select {
		case <-session.Done():
		case <-time.After(time.Second):
		}
	}
}

// Update runs the message through the model and then makes sure the title
// animation is running only while it is actually on screen.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	model, cmd := m.updateMsg(msg)
	next, ok := model.(*Model)
	if !ok || next == nil {
		return model, cmd
	}
	if anim := next.ensureAnimTick(); anim != nil {
		return next, tea.Batch(cmd, anim)
	}
	return next, cmd
}

func (m *Model) updateMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = m.width > 0 && m.height > 0
		return m, nil
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m.quitApp()
		}
		if m.modal != modalNone {
			return m.updateModal(msg)
		}
		if m.screen == ScreenVersions && m.filterActive {
			return m.updateFilter(msg)
		}
		return m.updateGlobalKey(msg)
	case tea.MouseMsg:
		return m.updateMouse(msg)
	case tickMsg:
		m.clock++
		m.spinner++
		if m.toast.Text != "" && time.Now().After(m.toast.Until) {
			m.toast = ToastState{}
		}
		return m, tickCmd()
	case animTickMsg:
		m.animRunning = false
		if m.animVisible() {
			m.animFrame++
		}
		return m, nil
	case announcementsMsg:
		return m.handleAnnouncements(msg)
	case contributorsMsg:
		return m.handleContributors(msg)
	case releasesMsg:
		return m.handleReleases(msg)
	case issuesMsg:
		return m.handleIssues(msg)
	case manifestMsg:
		m.manifestLoading = false
		if msg.manifest != nil {
			m.manifest = msg.manifest
			m.manifestAt = msg.at
			m.manifestErr = ""
			if msg.manifest.Warning != "" {
				m.manifestErr = msg.manifest.Warning
			}
			m.rebuildVersions()
			m.addLog("launcher", fmt.Sprintf("version manifest: %s · %s", msg.manifest.Source, plural(len(msg.manifest.Versions), "version", "versions")))
		}
		if msg.err != nil {
			m.manifestErr = msg.err.Error()
			m.addLog("error", "manifest: "+msg.err.Error())
			if m.manifest == nil {
				m.setToast(m.t("Could not load the version list"), toastError)
			}
		}
		return m, nil
	case javaScanMsg:
		m.javaScanning = false
		m.javaScanned = true
		m.javaRuntimes = msg.runtimes
		if msg.err != nil {
			m.addLog("error", "java scan: "+msg.err.Error())
		} else if len(msg.runtimes) == 0 {
			m.addLog("launcher", "java scan: no runtimes found")
		} else {
			m.addLog("launcher", fmt.Sprintf("java scan: %s", plural(len(msg.runtimes), "runtime", "runtimes")))
		}
		return m, nil
	case javaDetectMsg:
		if msg.err != nil {
			m.alert = AlertState{Title: m.t("Invalid Java path"), Body: msg.err.Error()}
			m.modal = modalAlert
			return m, nil
		}
		if msg.info != nil {
			m.cfg.Settings.JavaPath = msg.info.Path
			m.saveConfig()
			merged := append([]java.Info(nil), m.javaRuntimes...)
			found := false
			for i := range merged {
				if strings.EqualFold(merged[i].Path, msg.info.Path) {
					found = true
					break
				}
			}
			if !found {
				merged = append(merged, *msg.info)
			}
			m.javaRuntimes = merged
			m.javaScanned = true
			m.setToast(m.tf("Java runtime: %s", msg.info.Short()), toastSuccess)
			m.addLog("launcher", "java runtime: "+msg.info.Path)
		}
		return m, nil
	case installEventMsg:
		m.handleInstallEvent(msg.ev)
		return m, nil
	case taskStageMsg:
		if m.task != nil {
			m.task.Stage = msg.Stage
			m.task.Detail = msg.Detail
		}
		return m, nil
	case taskDoneMsg:
		return m.handleTaskDone(msg)
	case packSearchMsg:
		return m.handlePackSearch(msg)
	case packVersionsMsg:
		return m.handlePackVersions(msg)
	case gameStartedMsg:
		backgrounded := m.task != nil && m.task.Backgrounded
		m.recordTask(m.task, taskStatusCompleted, nil)
		m.game = &GameState{
			Session:      msg.session,
			InstanceID:   msg.instanceID,
			InstanceName: msg.instanceName,
			VersionID:    msg.versionID,
			Started:      time.Now(),
			Pid:          msg.pid,
		}
		m.task = nil
		m.addLog("launcher", fmt.Sprintf("started %s · pid %d", msg.instanceName, msg.pid))
		m.setToast(m.t("Minecraft is running"), toastSuccess)
		if backgrounded {
			// A backgrounded launch keeps whatever the user is looking at.
			return m, nil
		}
		m.modal = modalNone
		m.screen = ScreenConsole
		m.logFollow = true
		m.logScroll = 0
		return m, nil
	case gameLineMsg:
		source := "game"
		if msg.line.Stream == "stderr" {
			source = "stderr"
		}
		m.addLog(source, msg.line.Text)
		return m, nil
	case consoleLineMsg:
		m.addLog(msg.source, msg.text)
		return m, nil
	case gameExitMsg:
		return m.handleGameExit(msg)
	case toastMsg:
		m.setToast(msg.text, msg.kind)
		return m, nil
	case microsoftCodeMsg:
		return m.handleMicrosoftCode(msg)
	case microsoftTickMsg:
		if m.modal != modalMicrosoft || msg.attempt != m.ms.Attempt || m.ms.Stage != "waiting" {
			return m, nil
		}
		return m, pollMicrosoftCmd(m.client, m.ms.ClientID, m.ms.Code)
	case microsoftPollMsg:
		return m.handleMicrosoftPoll(msg)
	case microsoftSessionMsg:
		return m.handleMicrosoftSession(msg)
	case microsoftRefreshedMsg:
		return m.handleMicrosoftRefreshed(msg)
	case loaderVersionsMsg:
		m.handleLoaderVersions(msg)
		return m, nil
	case updateCheckMsg:
		return m.handleUpdateCheck(msg)
	}
	return m, nil
}

func (m *Model) updateGlobalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m.quitApp()
	case "?":
		m.modal = modalHelp
		m.helpScroll = 0
		return m, nil
	case "tab":
		m.switchScreen((m.screen + 1) % screenCount)
		return m, nil
	case "shift+tab":
		m.switchScreen((m.screen + screenCount - 1) % screenCount)
		return m, nil
	case "1", "2", "3", "4", "5", "6", "7":
		idx := int(msg.String()[0] - '1')
		if idx >= 0 && idx < int(screenCount) {
			m.switchScreen(Screen(idx))
		}
		return m, nil
	case "esc":
		if m.screen == ScreenVersions && m.filter != "" {
			m.filter = ""
			m.search.SetValue("")
			m.rebuildVersions()
			return m, nil
		}
		if m.screen != ScreenHome {
			m.switchScreen(ScreenHome)
		}
		return m, nil
	}

	switch m.screen {
	case ScreenHome:
		return m.updateHome(msg)
	case ScreenInstances:
		return m.updateInstances(msg)
	case ScreenVersions:
		return m.updateVersions(msg)
	case ScreenAccounts:
		return m.updateAccounts(msg)
	case ScreenSettings:
		return m.updateSettings(msg)
	case ScreenConsole:
		return m.updateConsole(msg)
	case ScreenDownloads:
		return m.updateDownloads(msg)
	case ScreenAbout:
		return m.updateAbout(msg)
	}
	return m, nil
}

func (m *Model) switchScreen(screen Screen) {
	if screen < 0 || screen >= screenCount {
		return
	}
	m.screen = screen
	m.filterActive = false
	if screen == ScreenVersions {
		m.installed = install.InstalledSet(m.layout)
		m.loaderMeta = loader.Scan(m.layout)
		m.rebuildVersions()
	}
	if screen == ScreenAccounts {
		m.clampAccountCursor()
	}
	if screen == ScreenInstances {
		m.clampInstanceCursor()
	}
	if screen == ScreenDownloads {
		m.downloadsScroll = 0
		m.clampDownloadsScroll()
	}
}

func (m *Model) loadManifestCmd(force bool) tea.Cmd {
	if force {
		m.manifestLoading = true
	}
	client := m.client
	layout := m.layout
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		manifest, err := mojang.FetchManifest(ctx, client, layout, force)
		return manifestMsg{manifest: manifest, err: err, at: time.Now()}
	}
}

func (m *Model) rebuildVersions() {
	if m.manifest == nil {
		filter := strings.ToLower(strings.TrimSpace(m.filter))
		out := make([]mojang.VersionSummary, 0, len(m.loaderMeta))
		for _, v := range m.localVersionSummaries() {
			if filter != "" && !strings.Contains(strings.ToLower(v.ID), filter) {
				continue
			}
			out = append(out, v)
		}
		m.visible = out
		if m.versionCursor >= len(out) {
			m.versionCursor = len(out) - 1
		}
		if m.versionCursor < 0 {
			m.versionCursor = 0
		}
		m.ensureVersionVisible()
		return
	}
	filter := strings.ToLower(strings.TrimSpace(m.filter))
	scope := m.versionScope
	out := make([]mojang.VersionSummary, 0, len(m.manifest.Versions)+len(m.loaderMeta))
	manifestIDs := make(map[string]bool, len(m.manifest.Versions))
	for _, v := range m.manifest.Versions {
		manifestIDs[v.ID] = true
	}
	if scope == 0 {
		locals := m.localVersionSummaries()
		for _, v := range locals {
			if manifestIDs[v.ID] {
				continue
			}
			if filter != "" && !strings.Contains(strings.ToLower(v.ID), filter) {
				continue
			}
			out = append(out, v)
		}
	}
	for _, v := range m.manifest.Versions {
		april := isAprilFoolsVersion(v.ID)
		switch scope {
		case 1:
			if v.Type != "release" || april {
				continue
			}
		case 2:
			if v.Type != "snapshot" || april {
				continue
			}
		case 3:
			if v.Type != "old_beta" && v.Type != "old_alpha" {
				continue
			}
		case 4:
			if !april {
				continue
			}
		}
		if filter != "" && !strings.Contains(strings.ToLower(v.ID), filter) {
			continue
		}
		out = append(out, v)
	}
	m.visible = out
	if m.versionCursor >= len(out) {
		m.versionCursor = len(out) - 1
	}
	if m.versionCursor < 0 {
		m.versionCursor = 0
	}
	m.ensureVersionVisible()
}

// localVersionSummaries turns installed loader sidecars into list entries that
// can be used to create instances just like Mojang versions.
func (m *Model) localVersionSummaries() []mojang.VersionSummary {
	out := make([]mojang.VersionSummary, 0, len(m.loaderMeta))
	for id, meta := range m.loaderMeta {
		if !m.installed[id] {
			continue
		}
		out = append(out, mojang.VersionSummary{
			ID:          id,
			Type:        "loader",
			ReleaseTime: time.Unix(meta.InstalledAt, 0).UTC().Format(time.RFC3339),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		ai, aj := m.loaderMeta[out[i].ID].InstalledAt, m.loaderMeta[out[j].ID].InstalledAt
		if ai != aj {
			return ai > aj
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func (m *Model) refreshInstalled() {
	m.installed = install.InstalledSet(m.layout)
	m.loaderMeta = loader.Scan(m.layout)
}

func (m *Model) ensureVersionVisible() {
	listHeight := m.listHeight()
	if listHeight < 1 {
		return
	}
	if m.versionCursor < m.versionOffset {
		m.versionOffset = m.versionCursor
	}
	if m.versionCursor >= m.versionOffset+listHeight {
		m.versionOffset = m.versionCursor - listHeight + 1
	}
	if m.versionOffset < 0 {
		m.versionOffset = 0
	}
}

func (m *Model) listHeight() int {
	h := m.bodyHeight() - 6 - m.filterBarHeight(m.contentWidth())
	if h < 4 {
		h = 4
	}
	return h
}

func (m *Model) bodyHeight() int {
	h := m.height - 3
	if h < 5 {
		h = 5
	}
	return h
}

func (m *Model) instanceListHeight() int {
	h := m.bodyHeight() - 6
	if h < 3 {
		h = 3
	}
	return h
}

func (m *Model) accountListHeight() int {
	return m.instanceListHeight()
}

func (m *Model) ensureInstanceVisible() {
	listHeight := m.instanceListHeight()
	if m.instanceCursor < m.instanceOffset {
		m.instanceOffset = m.instanceCursor
	}
	if m.instanceCursor >= m.instanceOffset+listHeight {
		m.instanceOffset = m.instanceCursor - listHeight + 1
	}
	if m.instanceOffset < 0 {
		m.instanceOffset = 0
	}
}

func (m *Model) ensureAccountVisible() {
	listHeight := m.accountListHeight()
	if m.accountCursor < m.accountOffset {
		m.accountOffset = m.accountCursor
	}
	if m.accountCursor >= m.accountOffset+listHeight {
		m.accountOffset = m.accountCursor - listHeight + 1
	}
	if m.accountOffset < 0 {
		m.accountOffset = 0
	}
}

func (m *Model) updateFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.filterActive = false
		m.filter = ""
		m.search.SetValue("")
		m.rebuildVersions()
		return m, nil
	case "enter":
		m.filterActive = false
		return m, nil
	case "ctrl+u":
		m.filter = ""
		m.search.SetValue("")
		m.rebuildVersions()
		return m, nil
	}
	var cmd tea.Cmd
	m.search, cmd = m.search.Update(msg)
	if m.search.Value() != m.filter {
		m.filter = m.search.Value()
		m.rebuildVersions()
	}
	return m, cmd
}

func (m *Model) startFilter() tea.Cmd {
	m.filterActive = true
	_, barW, _, _ := m.versionsHeaderGeometry(m.contentWidth())
	m.search.Width = util.Clamp(barW-5, 1, 1<<20)
	m.search.Focus()
	m.search.SetValue(m.filter)
	m.search.CursorEnd()
	return textinput.Blink
}

func (m *Model) switchScope() {
	m.versionScope = (m.versionScope + 1) % 5
	m.versionCursor = 0
	m.versionOffset = 0
	m.rebuildVersions()
}

// aprilFoolsVersions lists the Minecraft Java Edition releases that were
// published as April Fools jokes. They keep their real manifest type (release
// or snapshot), so a whitelist is the only reliable way to group them.
var aprilFoolsVersions = map[string]bool{
	"2.0":                  true,
	"15w14a":               true,
	"1.RV-Pre1":            true,
	"3D Shareware v1.34":   true,
	"20w14infinite":        true,
	"22w13oneblockatatime": true,
	"23w13a_or_b":          true,
	"24w14potato":          true,
	"25w14craftmine":       true,
}

func isAprilFoolsVersion(id string) bool { return aprilFoolsVersions[id] }

// saveConfig persists the config and reports a failure both as a toast and to
// the caller, so a later success message cannot hide it.
func (m *Model) saveConfig() error {
	err := m.cfg.Save()
	if err == nil {
		return nil
	}
	m.setToast(m.tf("Could not save config: %s", err.Error()), toastError)
	m.addLog("error", "save config: "+err.Error())
	return err
}

func (m *Model) setToast(text string, kind toastKind) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	m.toast = ToastState{Text: text, Kind: kind, Until: time.Now().Add(7 * time.Second)}
}

func (m *Model) addLog(source, text string) {
	text = ansi.Strip(strings.ReplaceAll(text, "\r", ""))
	parts := strings.Split(text, "\n")
	now := time.Now()
	added := 0
	for _, part := range parts {
		part = strings.TrimRight(part, " \t")
		if part == "" {
			continue
		}
		m.logs = append(m.logs, LogLine{At: now, Source: source, Text: part})
		added++
	}
	if len(m.logs) > 4000 {
		drop := len(m.logs) - 3000
		m.logs = append([]LogLine(nil), m.logs[drop:]...)
		if m.logScroll > 0 {
			m.logScroll -= drop
			if m.logScroll < 0 {
				m.logScroll = 0
			}
		}
	}
	if !m.logFollow && added > 0 {
		m.logScroll += added
	}
}

func (m *Model) clampInstanceCursor() {
	if m.instanceCursor >= len(m.cfg.Instances) {
		m.instanceCursor = len(m.cfg.Instances) - 1
	}
	if m.instanceCursor < 0 {
		m.instanceCursor = 0
	}
}

func (m *Model) clampAccountCursor() {
	if m.accountCursor >= len(m.cfg.Accounts) {
		m.accountCursor = len(m.cfg.Accounts) - 1
	}
	if m.accountCursor < 0 {
		m.accountCursor = 0
	}
}

func (m *Model) activeAccount() *config.Account {
	return m.cfg.ActiveAccount()
}

func (m *Model) currentInstance() *config.Instance {
	if len(m.cfg.Instances) == 0 {
		return nil
	}
	m.clampInstanceCursor()
	return &m.cfg.Instances[m.instanceCursor]
}

func (m *Model) currentVersion() *mojang.VersionSummary {
	if len(m.visible) == 0 {
		return nil
	}
	if m.versionCursor < 0 {
		m.versionCursor = 0
	}
	if m.versionCursor >= len(m.visible) {
		m.versionCursor = len(m.visible) - 1
	}
	return &m.visible[m.versionCursor]
}

func (m *Model) currentAccount() *config.Account {
	if len(m.cfg.Accounts) == 0 {
		return nil
	}
	m.clampAccountCursor()
	return &m.cfg.Accounts[m.accountCursor]
}

func (m *Model) recentInstances() []config.Instance {
	out := append([]config.Instance(nil), m.cfg.Instances...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].LastPlayedAt != out[j].LastPlayedAt {
			return out[i].LastPlayedAt > out[j].LastPlayedAt
		}
		return out[i].CreatedAt > out[j].CreatedAt
	})
	return out
}

func (m *Model) javaSummary() (string, bool) {
	if m.cfg.Settings.JavaPath != "" {
		for _, rt := range m.javaRuntimes {
			if strings.EqualFold(rt.Path, m.cfg.Settings.JavaPath) {
				return rt.Short(), true
			}
		}
		return util.Truncate(m.cfg.Settings.JavaPath, 28), true
	}
	if len(m.javaRuntimes) > 0 {
		best := m.javaRuntimes[0]
		for _, rt := range m.javaRuntimes[1:] {
			if rt.Major > best.Major {
				best = rt
			}
		}
		return best.Short(), true
	}
	if m.javaScanning {
		return m.t("scanning…"), false
	}
	return m.t("not found"), false
}

func (m *Model) runtimeForDisplay() string {
	label, ok := m.javaSummary()
	if !ok {
		return "Java " + label
	}
	return "Java " + strings.TrimPrefix(label, "Java ")
}

func (m *Model) settingsRows() []settingsRow {
	javaLabel := m.t("Auto-detect")
	if m.cfg.Settings.JavaPath != "" {
		javaLabel = util.ShortenPath(m.cfg.Settings.JavaPath, 52)
	} else if label, ok := m.javaSummary(); ok {
		javaLabel = m.t("Auto") + " · " + label
	}
	jvm := m.cfg.Settings.JVMArgs
	if strings.TrimSpace(jvm) == "" {
		jvm = "—"
	}
	language := m.tf("System (%s)", i18n.NativeName(m.lang))
	if m.cfg.Settings.Language != "" {
		language = i18n.NativeName(m.lang)
	}
	microsoft := m.t("Built-in")
	if m.cfg.Settings.MicrosoftClientID != "" {
		microsoft = util.Truncate(m.cfg.Settings.MicrosoftClientID, 40)
	}
	versionValue := m.AppVersion
	if m.updateChecking {
		versionValue = m.t("Checking for updates…")
	} else if m.updateRelease != nil {
		versionValue = m.AppVersion + " → " + m.updateRelease.TagName
	}
	return []settingsRow{
		{ID: "memory", Label: m.t("Memory"), Value: fmt.Sprintf("%d MB", m.cfg.Settings.MemoryMB),
			Description: m.t("Memory limit used by instances that do not set their own value.")},
		{ID: "java", Label: m.t("Java runtime"), Value: javaLabel,
			Description: m.t("Auto-detect selects the runtime requested by the Minecraft version.")},
		{ID: "jvm", Label: m.t("JVM arguments"), Value: util.Truncate(jvm, 56),
			Description: m.t("Extra space-separated flags passed to the JVM.")},
		{ID: "resolution", Label: m.t("Resolution"), Value: fmt.Sprintf("%d × %d", m.cfg.Settings.Width, m.cfg.Settings.Height),
			Description: m.t("Initial Minecraft window size.")},
		{ID: "gametitle", Label: m.t("Game window title"), Value: m.gameTitleValue(),
			Description: m.t("Rename the Minecraft window (Windows only); leave empty to keep the game's own title.")},
		{ID: "quitgame", Label: m.t("Keep game running"), Value: m.onOffLabel(m.cfg.Settings.KeepGameRunning),
			Description: m.t("Keep Minecraft running after the launcher quits.")},
		{ID: "threads", Label: m.t("Download threads"), Value: fmt.Sprintf("%d", m.cfg.Settings.DownloadThreads),
			Description: m.t("Parallel downloads used while installing game files.")},
		{ID: "language", Label: m.t("Language"), Value: language,
			Description: m.t("Interface language. System follows your operating system locale.")},
		{ID: "theme", Label: m.t("Theme"), Value: m.themeValue(),
			Description: m.t("Color palette used by the launcher. Custom accent overrides the primary color.")},
		{ID: "titlefont", Label: m.t("Title style"), Value: m.titleFontLabel(),
			Description: m.t("The glyph style used by the home title.")},
		{ID: "titleanim", Label: m.t("Title animation"), Value: m.titleAnimLabel(),
			Description: m.t("Animation shown on the home title.")},
		{ID: "data", Label: m.t("Data directory"), Value: util.ShortenPath(m.layout.Root, 56),
			Description: m.t("Versions, assets, libraries and instance game directories.")},
		{ID: "microsoft", Label: m.t("Microsoft Client ID"), Value: microsoft,
			Description: m.t("Optional Azure application (client) ID. Leave empty to use the built-in client.")},
		{ID: "version", Label: m.t("Launcher version"), Value: versionValue,
			Description: m.t("Press enter to check for updates. GitHub mirrors are used when needed.")},
	}
}

// t translates a UI key for the current language.
func (m *Model) t(key string) string { return i18n.T(m.lang, key) }

// tf translates and formats a UI template.
func (m *Model) tf(key string, args ...any) string { return i18n.TF(m.lang, key, args...) }

// trKnown translates s when a catalog entry exists, otherwise returns it
// unchanged. It is used for stage names coming from the installer.
func (m *Model) trKnown(s string) string {
	if s == "" {
		return ""
	}
	if m.lang == "" || m.lang == i18n.EN || i18n.Has(m.lang, s) {
		return m.t(s)
	}
	return s
}

// count renders "3 instances" (or the language equivalent).
func (m *Model) count(n int, one, many string) string {
	return i18n.Count(m.lang, n, one, many)
}

// relTime is the localized relative timestamp used by lists.
func (m *Model) relTime(ts int64) string { return i18n.RelativeTime(m.lang, ts) }

// rtl reports whether the current language uses a right-to-left layout.
func (m *Model) rtl() bool { return i18n.IsRTL(m.lang) }

// setLanguage applies a language immediately and persists it.
func (m *Model) setLanguage(lang i18n.Lang) {
	if !i18n.SupportedCode(lang) {
		lang = i18n.EN
	}
	m.lang = lang
	m.cfg.Settings.Language = string(lang)
	m.search.Placeholder = m.t("search versions")
	m.saveConfig()
	m.setToast(m.tf("Language: %s", i18n.NativeName(lang)), toastSuccess)
}

func (m *Model) themeValue() string {
	value := themeName(m.cfg.Settings.Theme)
	if accent := m.cfg.Settings.ThemeAccent; accent != "" {
		value += " · " + accent
	}
	return value
}

func (m *Model) setTheme(id, accent string) {
	m.cfg.Settings.Theme = normalizeThemeID(id)
	if normalized, ok := util.NormalizeHexColor(accent); ok {
		m.cfg.Settings.ThemeAccent = normalized
	} else {
		m.cfg.Settings.ThemeAccent = ""
	}
	applyTheme(m.cfg.Settings.Theme, m.cfg.Settings.ThemeAccent)
	m.restyleTextInputs()
	m.saveConfig()
}

func (m *Model) restyleTextInputs() {
	m.search.PromptStyle = styleSearchPrompt
	m.search.TextStyle = styleSearchText
	m.search.PlaceholderStyle = styleSearchPlaceholder
	m.search.Cursor.Style = styleSearchCursor
	if len(m.form.Fields) > 0 {
		for i := range m.form.Fields {
			m.form.Fields[i].PromptStyle = stylePrimary
			m.form.Fields[i].TextStyle = styleText
			m.form.Fields[i].PlaceholderStyle = styleDim
			m.form.Fields[i].Cursor.Style = stylePrimary
		}
	}
	m.prompt.Input.PromptStyle = stylePrimary
	m.prompt.Input.TextStyle = styleText
	m.prompt.Input.PlaceholderStyle = styleDim
	m.prompt.Input.Cursor.Style = stylePrimary
}

func (m *Model) useSystemLanguage() {
	m.lang = i18n.Resolve("")
	m.cfg.Settings.Language = ""
	m.search.Placeholder = m.t("search versions")
	m.saveConfig()
	m.setToast(m.tf("Language: System (%s)", i18n.NativeName(m.lang)), toastSuccess)
}

func (m *Model) currentSettingID() string {
	rows := m.settingsRows()
	if m.settingsCursor < 0 || m.settingsCursor >= len(rows) {
		return ""
	}
	return rows[m.settingsCursor].ID
}
