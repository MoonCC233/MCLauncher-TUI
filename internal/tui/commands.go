package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/MoonCC233/MCLauncher-TUI/internal/auth"
	"github.com/MoonCC233/MCLauncher-TUI/internal/config"
	"github.com/MoonCC233/MCLauncher-TUI/internal/install"
	"github.com/MoonCC233/MCLauncher-TUI/internal/java"
	"github.com/MoonCC233/MCLauncher-TUI/internal/launch"
	"github.com/MoonCC233/MCLauncher-TUI/internal/loader"
	"github.com/MoonCC233/MCLauncher-TUI/internal/mojang"
	"github.com/MoonCC233/MCLauncher-TUI/internal/util"
	"github.com/MoonCC233/MCLauncher-TUI/internal/version"
)

type tickMsg time.Time

type manifestMsg struct {
	manifest *mojang.Manifest
	err      error
	at       time.Time
}

type javaScanMsg struct {
	runtimes []java.Info
	err      error
}

type javaDetectMsg struct {
	info *java.Info
	err  error
}

type installEventMsg struct {
	ev install.Event
}

type taskStageMsg struct {
	Stage  string
	Detail string
}

type taskDoneMsg struct {
	kind       taskKind
	versionID  string
	instanceID string
	err        error
	cancelled  bool
}

type gameStartedMsg struct {
	session      *launch.Session
	instanceID   string
	instanceName string
	versionID    string
	pid          int
}

type gameLineMsg struct {
	line launch.Line
}

type gameExitMsg struct {
	instanceID   string
	instanceName string
	err          error
	played       time.Duration
	exitCode     int
}

type toastMsg struct {
	text string
	kind toastKind
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// animTickMsg drives the home screen wordmark animation.
type animTickMsg struct{}

func animTickCmd() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(time.Time) tea.Msg { return animTickMsg{} })
}

func scanJavaCmd(extraRoots ...string) tea.Cmd {
	return func() tea.Msg {
		return javaScanMsg{runtimes: java.Scan(extraRoots...)}
	}
}

func detectJavaCmd(path string) tea.Cmd {
	return func() tea.Msg {
		info := java.Detect(path)
		if info == nil {
			return javaDetectMsg{err: fmt.Errorf("no Java runtime found at %s", path)}
		}
		return javaDetectMsg{info: info}
	}
}

func (m *Model) startInstall(versionID string, createName string, createMemory int) tea.Cmd {
	if m.task != nil {
		m.alert = AlertState{Title: m.t("Another task is running"), Body: m.t("Wait for the current installation or launch to finish.")}
		m.modal = modalAlert
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	task := &TaskState{
		Kind:         taskInstall,
		Title:        m.tf("Installing %s", versionID),
		VersionID:    versionID,
		CreateName:   createName,
		CreateMemory: createMemory,
		Stage:        "Preparing",
		Detail:       m.t("Contacting Mojang"),
		Started:      time.Now(),
		Cancel:       cancel,
	}
	m.task = task
	m.modal = modalProgress
	m.addLog("launcher", "install "+versionID)

	layout := m.layout
	client := m.client
	manifest := m.manifest
	threads := m.cfg.Settings.DownloadThreads
	send := m.send

	return func() tea.Msg {
		go func() {
			emitter := func(msg tea.Msg) {
				if send != nil {
					send(msg)
				}
			}
			installer := &install.Installer{
				Layout:    layout,
				VersionID: versionID,
				Manifest:  manifest,
				Threads:   threads,
				Client:    client,
				OnEvent: func(ev install.Event) {
					emitter(installEventMsg{ev: ev})
				},
			}
			err := installer.Install(ctx)
			emitter(taskDoneMsg{
				kind:      taskInstall,
				versionID: versionID,
				err:       err,
				cancelled: errors.Is(err, context.Canceled),
			})
		}()
		return nil
	}
}

func (m *Model) uninstallVersion(id string) error {
	if id == "" || !m.installed[id] {
		return fmt.Errorf("version %s is not installed", id)
	}
	dir := m.layout.VersionDir(id)
	// Move the version aside first so a failure while deleting leaves the
	// launcher and the game directory consistent: either the version is still
	// complete, or it is gone and only leftover files remain to clean up.
	staged := dir + ".uninstall-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	if err := os.Rename(dir, staged); err != nil {
		if _, statErr := os.Stat(dir); statErr != nil {
			// Already gone; treat the uninstall as done.
			staged = ""
		} else {
			return err
		}
	}
	m.installed = install.InstalledSet(m.layout)
	m.loaderMeta = loader.Scan(m.layout)
	m.rebuildVersions()
	m.addLog("launcher", "uninstalled "+id)
	if staged != "" {
		if err := os.RemoveAll(staged); err != nil {
			m.addLog("error", fmt.Sprintf("remove %s: %v", staged, err))
		}
	}
	return nil
}

func (m *Model) startLaunch(inst config.Instance) tea.Cmd {
	if m.task != nil {
		m.alert = AlertState{Title: m.t("Another task is running"), Body: m.t("Wait for the current installation to finish.")}
		m.modal = modalAlert
		return nil
	}
	if m.game != nil {
		m.alert = AlertState{
			Title: m.t("Minecraft is already running"),
			Body:  m.tf("%s is still running. Stop it from the Console screen before launching another instance.", m.game.InstanceName),
		}
		m.modal = modalAlert
		return nil
	}
	account := m.cfg.ActiveAccount()
	if account == nil {
		m.alert = AlertState{Title: m.t("No account selected"), Body: m.t("Add an offline account from the Accounts screen first.")}
		m.modal = modalAlert
		return nil
	}
	accountCopy := *account
	accountUUID := account.UUID
	settings := m.cfg.Settings
	memory := m.cfg.MemoryFor(&inst)
	runtimes := append([]java.Info(nil), m.javaRuntimes...)
	javaScanned := m.javaScanned
	javaNeeded := m.t("needs Java %d+")
	microsoftExpired := m.t("Microsoft session expired. Sign in again from the Accounts screen.")

	ctx, cancel := context.WithCancel(context.Background())
	m.task = &TaskState{
		Kind:       taskLaunch,
		Title:      m.tf("Launching %s", inst.Name),
		VersionID:  inst.VersionID,
		InstanceID: inst.ID,
		Stage:      "Preparing",
		Detail:     m.t("Checking installed files"),
		Started:    time.Now(),
		Cancel:     cancel,
	}
	m.modal = modalProgress
	m.addLog("launcher", fmt.Sprintf("launch %s (%s)", inst.Name, inst.VersionID))

	layout := m.layout
	client := m.client
	manifest := m.manifest
	threads := m.cfg.Settings.DownloadThreads
	launcherVersion := version.Full()
	send := m.send

	return func() tea.Msg {
		go func() {
			emitter := func(msg tea.Msg) {
				if send != nil {
					send(msg)
				}
			}
			stage := func(name, detail string) {
				emitter(taskStageMsg{Stage: name, Detail: detail})
			}
			fail := func(err error) {
				emitter(taskDoneMsg{
					kind:       taskLaunch,
					instanceID: inst.ID,
					versionID:  inst.VersionID,
					err:        err,
					cancelled:  errors.Is(err, context.Canceled) || ctx.Err() != nil,
				})
			}
			checkCancel := func() bool {
				if ctx.Err() != nil {
					fail(ctx.Err())
					return true
				}
				return false
			}

			if accountCopy.IsMicrosoft() && auth.NeedsRefresh(accountCopy.ExpiresAt) {
				stage("Refreshing Microsoft session", accountCopy.Username)
				session, refreshErr := auth.RefreshMicrosoft(ctx, client, accountCopy.ClientID, accountCopy.RefreshToken)
				if refreshErr != nil {
					fail(fmt.Errorf("%s: %w", microsoftExpired, refreshErr))
					return
				}
				accountCopy.Username = session.Username
				accountCopy.UUID = session.UUID
				accountCopy.AccessToken = session.AccessToken
				accountCopy.RefreshToken = session.RefreshToken
				accountCopy.XUID = session.XUID
				accountCopy.ClientID = session.ClientID
				accountCopy.ExpiresAt = session.ExpiresAt
				emitter(microsoftRefreshedMsg{uuid: accountUUID, session: session})
			}
			if checkCancel() {
				return
			}

			stage("Checking installed files", inst.VersionID)
			if !install.IsInstalled(layout, inst.VersionID) {
				emitter(taskStageMsg{Stage: "Installing game files", Detail: inst.VersionID})
				installer := &install.Installer{
					Layout:    layout,
					VersionID: inst.VersionID,
					Manifest:  manifest,
					Threads:   threads,
					Client:    client,
					OnEvent: func(ev install.Event) {
						emitter(installEventMsg{ev: ev})
					},
				}
				if err := installer.Install(ctx); err != nil {
					fail(err)
					return
				}
			}
			if checkCancel() {
				return
			}

			stage("Loading version metadata", inst.VersionID)
			versionJSON, err := mojang.LoadVersionJSON(layout, inst.VersionID)
			if err != nil {
				man := manifest
				if man == nil {
					man, _ = mojang.FetchManifest(ctx, client, layout, false)
				}
				versionJSON, err = mojang.FetchVersionJSON(ctx, client, layout, man, inst.VersionID, true)
			}
			if err != nil {
				fail(fmt.Errorf("load version %s: %w", inst.VersionID, err))
				return
			}
			if checkCancel() {
				return
			}

			stage("Selecting Java runtime", fmt.Sprintf(javaNeeded, versionJSON.JavaMajor()))
			var runtime java.Info
			if strings.TrimSpace(settings.JavaPath) != "" {
				info := java.Detect(settings.JavaPath)
				if info == nil {
					fail(fmt.Errorf("no Java runtime at %s", settings.JavaPath))
					return
				}
				runtime = *info
			} else {
				if !javaScanned || len(runtimes) == 0 {
					runtimes = java.Scan(layout.Java)
					emitter(javaScanMsg{runtimes: runtimes})
				}
				selected, warning, selectErr := java.Select(runtimes, versionJSON.JavaMajor(), "")
				if selectErr != nil && javaScanned {
					runtimes = java.Scan(layout.Java)
					emitter(javaScanMsg{runtimes: runtimes})
					selected, warning, selectErr = java.Select(runtimes, versionJSON.JavaMajor(), "")
				}
				if selectErr != nil {
					fail(selectErr)
					return
				}
				if warning != "" {
					emitter(consoleLineMsg{source: "launcher", text: warning})
				}
				runtime = selected
			}
			if checkCancel() {
				return
			}

			stage("Preparing game directory", inst.Name)
			gameDir := layout.InstanceDir(inst.ID)
			if err := os.MkdirAll(gameDir, 0o755); err != nil {
				fail(err)
				return
			}

			stage("Preparing assets", versionJSON.AssetID())
			assetsDir, err := install.EnsureGameAssets(ctx, layout, versionJSON, gameDir)
			if err != nil {
				fail(fmt.Errorf("prepare assets: %w", err))
				return
			}
			logConfig := ""
			if versionJSON.Logging != nil && versionJSON.Logging.Client != nil {
				candidate := install.LogConfigPath(layout, versionJSON)
				if st, statErr := os.Stat(candidate); statErr == nil && !st.IsDir() {
					if consoleConfig, configErr := install.ConsoleLogConfig(layout, candidate); configErr == nil {
						logConfig = consoleConfig
					} else {
						logConfig = candidate
					}
				}
			}

			stage("Starting Minecraft", runtime.Short())
			spec, err := launch.Build(launch.Options{
				Layout:          layout,
				Version:         versionJSON,
				VersionID:       inst.VersionID,
				Account:         accountCopy,
				Java:            runtime,
				GameDir:         gameDir,
				AssetsDir:       assetsDir,
				MemoryMB:        memory,
				JVMArgs:         settings.JVMArgs,
				Width:           settings.Width,
				Height:          settings.Height,
				LogConfig:       logConfig,
				LauncherVersion: launcherVersion,
			})
			if err != nil {
				fail(err)
				return
			}
			if checkCancel() {
				return
			}
			session, err := launch.Start(ctx, spec)
			if err != nil {
				fail(err)
				return
			}
			if checkCancel() {
				_ = session.Stop()
				return
			}
			started := time.Now()
			emitter(gameStartedMsg{
				session:      session,
				instanceID:   inst.ID,
				instanceName: inst.Name,
				versionID:    inst.VersionID,
				pid:          session.Pid(),
			})
			for line := range session.Lines() {
				emitter(gameLineMsg{line: line})
			}
			waitErr := session.Wait()
			emitter(gameExitMsg{
				instanceID:   inst.ID,
				instanceName: inst.Name,
				err:          waitErr,
				played:       time.Since(started),
				exitCode:     launch.ExitCode(waitErr),
			})
		}()
		return nil
	}
}

type consoleLineMsg struct {
	source string
	text   string
}

func (m *Model) handleInstallEvent(ev install.Event) {
	if m.task == nil {
		if ev.Type == install.EventLog && ev.Detail != "" {
			m.addLog("launcher", ev.Detail)
		}
		return
	}
	switch ev.Type {
	case install.EventStage:
		m.task.Stage = m.trKnown(ev.Stage)
		if ev.Detail != "" {
			m.task.Detail = m.trKnown(ev.Detail)
		}
		if ev.Total > 0 {
			m.task.Total = ev.Total
		}
		if ev.FilesTotal > 0 {
			m.task.FilesTotal = ev.FilesTotal
		}
		if ev.Stage != "" && ev.Detail != "" {
			m.addLog("launcher", ev.Stage+" · "+ev.Detail)
		}
	case install.EventProgress:
		m.task.Stage = m.trKnown(ev.Stage)
		m.task.Detail = m.trKnown(ev.Detail)
		m.task.Done = ev.Done
		m.task.Total = ev.Total
		m.task.FilesDone = ev.FilesDone
		m.task.FilesTotal = ev.FilesTotal
	case install.EventLog:
		m.addLog("launcher", ev.Detail)
	case install.EventDone:
		m.task.Done = ev.Total
		m.task.Total = ev.Total
		m.task.FilesDone = ev.FilesDone
		m.task.FilesTotal = ev.FilesTotal
		m.task.Stage = m.trKnown("Ready")
	}
}

func (m *Model) handleTaskDone(msg taskDoneMsg) (tea.Model, tea.Cmd) {
	task := m.task
	backgrounded := task != nil && task.Backgrounded
	m.task = nil
	if !backgrounded && m.modal == modalProgress {
		m.modal = modalNone
	}
	if msg.cancelled || errors.Is(msg.err, context.Canceled) {
		m.recordTask(task, taskStatusCancelled, nil)
		m.addLog("launcher", "task cancelled")
		m.setToast(m.t("Cancelled"), toastInfo)
		return m, nil
	}
	if msg.err != nil {
		m.recordTask(task, taskStatusFailed, msg.err)
		m.addLog("error", msg.err.Error())
		if backgrounded {
			m.setToast(m.t("Task failed — see Downloads"), toastError)
			return m, nil
		}
		title := m.t("Installation failed")
		if msg.kind == taskLaunch {
			title = m.t("Could not launch Minecraft")
		}
		if msg.kind == taskLoader {
			title = m.t("Loader installation failed")
		}
		if msg.kind == taskUpdate {
			title = m.t("Update failed")
		}
		if msg.kind == taskJava {
			title = m.t("Java installation failed")
		}
		m.alert = AlertState{Title: title, Body: msg.err.Error()}
		m.modal = modalAlert
		return m, nil
	}
	if msg.kind == taskJava {
		m.recordTask(task, taskStatusCompleted, nil)
		label := msg.versionID
		if info := java.Detect(msg.versionID); info != nil {
			label = info.Short()
		}
		m.cfg.Settings.JavaPath = msg.versionID
		m.saveConfig()
		m.addLog("launcher", "java runtime: "+msg.versionID)
		m.setToast(m.tf("Java runtime: %s", label), toastSuccess)
		m.javaScanning = true
		m.javaScanned = false
		return m, scanJavaCmd(m.layout.Java)
	}
	if msg.kind == taskUpdate {
		m.recordTask(task, taskStatusCompleted, nil)
		m.updateRelease = nil
		m.updateInstalled = msg.versionID
		m.alert = AlertState{
			Title: m.t("Update installed"),
			Body:  m.tf("Restart the launcher to finish updating to %s.", msg.versionID),
		}
		m.modal = modalAlert
		return m, nil
	}
	m.installed = install.InstalledSet(m.layout)
	m.loaderMeta = loader.Scan(m.layout)
	if msg.kind == taskLoader {
		m.rebuildVersions()
		for i := range m.visible {
			if m.visible[i].ID == msg.versionID {
				m.versionCursor = i
				m.ensureVersionVisible()
				break
			}
		}
		m.recordTask(task, taskStatusCompleted, nil)
		m.setToast(m.tf("Installed %s", msg.versionID), toastSuccess)
		m.addLog("launcher", "installed "+msg.versionID)
		return m, nil
	}
	if msg.kind == taskInstall && task != nil && task.CreateName != "" {
		versionID := task.VersionID
		if msg.versionID != "" {
			versionID = msg.versionID
		}
		inst, err := m.cfg.AddInstance(task.CreateName, versionID, task.CreateMemory)
		if err != nil {
			// The files are installed but the instance could not be saved, so the
			// task has to show up as failed instead of completed.
			m.recordTask(task, taskStatusFailed, err)
			m.addLog("error", err.Error())
			m.alert = AlertState{Title: m.t("Could not create instance"), Body: err.Error()}
			if backgrounded {
				m.setToast(m.t("Could not create instance"), toastError)
				return m, nil
			}
			m.modal = modalAlert
			return m, nil
		}
		m.recordTask(task, taskStatusCompleted, nil)
		m.instanceCursor = len(m.cfg.Instances) - 1
		m.homeCursor = 0
		if !backgrounded {
			m.screen = ScreenHome
		}
		m.setToast(m.tf("Created %s", inst.Name), toastSuccess)
		m.addLog("launcher", "created instance "+inst.Name)
		return m, nil
	}
	m.recordTask(task, taskStatusCompleted, nil)
	if msg.kind == taskInstall {
		m.setToast(m.tf("Installed %s", msg.versionID), toastSuccess)
		m.addLog("launcher", "installed "+msg.versionID)
	}
	return m, nil
}

func (m *Model) handleGameExit(msg gameExitMsg) (tea.Model, tea.Cmd) {
	game := m.game
	if game == nil {
		if msg.err != nil {
			m.addLog("error", msg.err.Error())
		}
		return m, nil
	}
	wasStopping := game.Stopping
	m.game = nil
	exitCode := msg.exitCode
	if msg.err != nil && exitCode == 0 {
		exitCode = -1
	}
	if msg.err != nil {
		m.addLog("error", fmt.Sprintf("Minecraft exited with code %d", exitCode))
	} else {
		m.addLog("launcher", fmt.Sprintf("Minecraft exited (code %d) after %s", exitCode, util.HumanDuration(msg.played)))
	}
	if inst := m.cfg.FindInstance(msg.instanceID); inst != nil {
		_ = m.cfg.TouchInstance(inst, game.Started, msg.played)
	}
	m.logFollow = true
	m.logScroll = 0
	if wasStopping {
		m.setToast(m.t("Minecraft stopped"), toastInfo)
	} else if exitCode == 0 {
		m.setToast(m.tf("Minecraft exited · played %s", util.HumanDuration(msg.played)), toastSuccess)
	} else {
		m.setToast(m.tf("Minecraft exited with code %d — check Console", exitCode), toastError)
	}
	return m, nil
}

func (m *Model) cancelTask() {
	if m.task == nil {
		return
	}
	if m.task.Cancel != nil {
		m.task.Cancel()
	}
	m.task.Cancelled = true
	m.task.Stage = "Cancelling"
	m.task.Detail = m.t("waiting for workers to stop")
}

func (m *Model) stopGame() {
	if m.game == nil || m.game.Session == nil {
		return
	}
	m.game.Stopping = true
	m.addLog("launcher", "stopping Minecraft…")
	_ = m.game.Session.Stop()
}

func (m *Model) openInstanceForm(versionID string) {
	name := versionID
	versionInstalled := m.installed[versionID]
	fields := make([]textinput.Model, 2)
	fields[0] = newTextInput(m.t("My World"), name, 34, 48)
	fields[0].Focus()
	fields[1] = newTextInput(m.t("global default"), "", 34, 12)
	m.form = FormState{
		Kind:      formInstance,
		Title:     m.t("New instance"),
		VersionID: versionID,
		Fields:    fields,
		Labels:    []string{m.t("Name"), m.t("Memory (MB, optional)")},
		Hints:     []string{"", m.t("Leave empty to use the global memory limit")},
		Focus:     0,
	}
	if versionInstalled {
		m.form.Hints = append(m.form.Hints, "")
	} else {
		m.form.Hints[0] = m.tf("Version %s will be installed first", versionID)
	}
	m.modal = modalForm
}

func (m *Model) openEditInstanceForm(inst *config.Instance) {
	fields := make([]textinput.Model, 2)
	fields[0] = newTextInput(m.t("My World"), inst.Name, 34, 48)
	fields[0].Focus()
	mem := ""
	if inst.MemoryMB > 0 {
		mem = fmt.Sprintf("%d", inst.MemoryMB)
	}
	fields[1] = newTextInput(m.t("global default"), mem, 34, 12)
	m.form = FormState{
		Kind:      formInstance,
		Title:     m.t("Edit instance"),
		VersionID: inst.VersionID,
		EditID:    inst.ID,
		Fields:    fields,
		Labels:    []string{m.t("Name"), m.t("Memory (MB, optional)")},
		Hints:     []string{m.tf("Version %s (fixed)", inst.VersionID), m.t("Leave empty to use the global memory limit")},
		Focus:     0,
	}
	m.modal = modalForm
}

func (m *Model) openAccountForm() {
	field := newTextInput(m.t("username"), "", 30, 16)
	field.Focus()
	m.form = FormState{
		Kind:   formAccount,
		Title:  m.t("Add offline account"),
		Fields: []textinput.Model{field},
		Labels: []string{m.t("Username")},
		Hints:  []string{m.t("3–16 characters · letters, numbers, underscore")},
		Focus:  0,
	}
	m.modal = modalForm
}

func newTextInput(placeholder, value string, width, limit int) textinput.Model {
	input := textinput.New()
	input.Placeholder = placeholder
	input.SetValue(value)
	input.CursorEnd()
	input.CharLimit = limit
	input.Width = width
	input.Prompt = "› "
	input.PromptStyle = stylePrimary
	input.TextStyle = styleText
	input.PlaceholderStyle = styleDim
	input.Cursor.Style = stylePrimary
	return input
}

func (m *Model) submitForm() (tea.Model, tea.Cmd) {
	values := make([]string, len(m.form.Fields))
	for i := range m.form.Fields {
		values[i] = strings.TrimSpace(m.form.Fields[i].Value())
	}
	switch m.form.Kind {
	case formAccount:
		name := values[0]
		if _, err := m.cfg.AddAccount(name); err != nil {
			m.form.Err = err.Error()
			return m, nil
		}
		m.clampAccountCursor()
		m.ensureAccountVisible()
		for i := range m.cfg.Accounts {
			if strings.EqualFold(m.cfg.Accounts[i].Username, name) {
				m.accountCursor = i
				break
			}
		}
		m.ensureAccountVisible()
		m.modal = modalNone
		m.setToast(m.tf("Added offline account %s", name), toastSuccess)
		m.addLog("launcher", "added account "+name)
		return m, nil
	case formInstance:
		name := values[0]
		if name == "" {
			m.form.Err = m.t("instance name cannot be empty")
			return m, nil
		}
		if len([]rune(name)) > 48 {
			m.form.Err = m.t("instance name is too long (max 48 characters)")
			return m, nil
		}
		memory := 0
		if values[1] != "" {
			n, err := parseInt(values[1])
			if err != nil || n < 512 || n > 65536 {
				m.form.Err = m.t("memory must be between 512 and 65536 MB")
				return m, nil
			}
			memory = n
		}
		if m.form.EditID != "" {
			inst := m.cfg.FindInstance(m.form.EditID)
			if inst == nil {
				m.form.Err = m.t("instance no longer exists")
				return m, nil
			}
			if err := m.cfg.UpdateInstance(inst, name, m.form.VersionID, memory); err != nil {
				m.form.Err = err.Error()
				return m, nil
			}
			m.modal = modalNone
			m.setToast(m.tf("Saved %s", name), toastSuccess)
			return m, nil
		}
		versionID := m.form.VersionID
		if !m.installed[versionID] {
			cmd := m.startInstall(versionID, name, memory)
			return m, cmd
		}
		inst, err := m.cfg.AddInstance(name, versionID, memory)
		if err != nil {
			m.form.Err = err.Error()
			return m, nil
		}
		m.instanceCursor = len(m.cfg.Instances) - 1
		m.homeCursor = 0
		m.ensureInstanceVisible()
		m.modal = modalNone
		m.screen = ScreenHome
		m.setToast(m.tf("Created %s", inst.Name), toastSuccess)
		m.addLog("launcher", "created instance "+inst.Name)
		return m, nil
	}
	m.modal = modalNone
	return m, nil
}

func parseInt(s string) (int, error) {
	return strconv.Atoi(strings.TrimSpace(s))
}

func (m *Model) gameDir(id string) string {
	return filepath.Join(m.layout.Instances, id)
}
