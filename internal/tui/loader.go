package tui

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MoonCC233/MCLauncher-TUI/internal/install"
	"github.com/MoonCC233/MCLauncher-TUI/internal/loader"
	"github.com/MoonCC233/MCLauncher-TUI/internal/util"
)

const (
	loaderStepKind = iota
	loaderStepVersion
	loaderStepJava
)

type loaderVersionsMsg struct {
	kind      loader.Kind
	minecraft string
	versions  []loader.RemoteVersion
	err       error
}

func (m *Model) openLoaderPicker(versionID string) {
	minecraft := strings.TrimSpace(versionID)
	if meta, ok := m.loaderMeta[versionID]; ok && meta.Minecraft != "" {
		minecraft = meta.Minecraft
	}
	m.loaderPick = loaderPicker{
		Step:      loaderStepKind,
		Minecraft: minecraft,
	}
	m.modal = modalLoader
}

func fetchLoaderVersionsCmd(client *http.Client, kind loader.Kind, minecraft string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		versions, err := loader.FetchVersions(ctx, client, kind, minecraft)
		return loaderVersionsMsg{kind: kind, minecraft: minecraft, versions: versions, err: err}
	}
}

func (m *Model) handleLoaderVersions(msg loaderVersionsMsg) {
	if m.modal != modalLoader || m.loaderPick.Step != loaderStepVersion {
		return
	}
	if msg.kind != m.loaderPick.Kind || msg.minecraft != m.loaderPick.Minecraft {
		return
	}
	m.loaderPick.Loading = false
	m.loaderPick.Versions = msg.versions
	m.loaderPick.Cursor = 0
	if msg.err != nil {
		m.loaderPick.Err = msg.err.Error()
		m.addLog("error", "loader list: "+msg.err.Error())
	}
}

func (m *Model) updateLoaderPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		switch m.loaderPick.Step {
		case loaderStepJava:
			m.loaderPick.Err = ""
			if m.loaderPick.Kind == loader.OptiFine {
				m.loaderPick.Step = loaderStepKind
				m.loaderPick.Cursor = 0
				m.loaderPick.OptiFineJar = ""
				return m, nil
			}
			m.loaderPick.Step = loaderStepVersion
			m.loaderPick.Cursor = 0
			return m, nil
		case loaderStepVersion:
			m.loaderPick.Step = loaderStepKind
			m.loaderPick.Cursor = 0
			m.loaderPick.Err = ""
			m.loaderPick.Versions = nil
			m.loaderPick.Loading = false
			return m, nil
		default:
			m.modal = modalNone
			return m, nil
		}
	case "up", "k":
		m.loaderPick.Cursor--
	case "down", "j":
		m.loaderPick.Cursor++
	case "home":
		m.loaderPick.Cursor = 0
	case "end":
		m.loaderPick.Cursor = m.loaderPickerCount() - 1
	case "enter":
		switch m.loaderPick.Step {
		case loaderStepKind:
			return m.chooseLoaderKind()
		case loaderStepVersion:
			return m.chooseLoaderVersion()
		case loaderStepJava:
			return m.startLoaderInstall()
		}
	}
	m.clampLoaderCursor()
	return m, nil
}

func (m *Model) clampLoaderCursor() {
	count := m.loaderPickerCount()
	if count <= 0 {
		m.loaderPick.Cursor = 0
		return
	}
	m.loaderPick.Cursor = clampInt(m.loaderPick.Cursor, 0, count-1)
}

func (m *Model) loaderPickerCount() int {
	switch m.loaderPick.Step {
	case loaderStepKind:
		return len(loader.Kinds)
	case loaderStepVersion:
		return len(m.loaderPick.Versions)
	case loaderStepJava:
		return len(m.javaRuntimes) + 1
	}
	return 0
}

func (m *Model) chooseLoaderKind() (tea.Model, tea.Cmd) {
	idx := clampInt(m.loaderPick.Cursor, 0, len(loader.Kinds)-1)
	kind := loader.Kinds[idx]
	m.loaderPick.Kind = kind
	m.loaderPick.Err = ""
	m.loaderPick.Cursor = 0
	if kind == loader.OptiFine {
		m.openPrompt(promptOptiFineJar, m.t("OptiFine jar"), "",
			m.t("Full path to an OptiFine_<version>_<edition>.jar file"), 260)
		return m, nil
	}
	m.loaderPick.Step = loaderStepVersion
	m.loaderPick.Loading = true
	m.loaderPick.Versions = nil
	return m, fetchLoaderVersionsCmd(m.client, kind, m.loaderPick.Minecraft)
}

func (m *Model) chooseLoaderVersion() (tea.Model, tea.Cmd) {
	if len(m.loaderPick.Versions) == 0 {
		return m, nil
	}
	idx := clampInt(m.loaderPick.Cursor, 0, len(m.loaderPick.Versions)-1)
	version := m.loaderPick.Versions[idx].Version
	versionID := loaderVersionID(m.loaderPick.Kind, m.loaderPick.Minecraft, version)
	if m.installed[versionID] {
		m.loaderPick.Err = m.tf("%s is already installed", versionID)
		return m, nil
	}
	m.loaderPick.LoaderVersion = version
	m.loaderPick.Step = loaderStepJava
	m.loaderPick.Cursor = m.defaultLoaderJavaCursor()
	m.loaderPick.Err = ""
	return m, nil
}

func loaderVersionID(kind loader.Kind, minecraft, version string) string {
	switch kind {
	case loader.Fabric:
		return fmt.Sprintf("fabric-loader-%s-%s", version, minecraft)
	case loader.Quilt:
		return fmt.Sprintf("quilt-loader-%s-%s", version, minecraft)
	case loader.Forge:
		return loader.ForgeVersionID(minecraft, version)
	}
	return ""
}

func (m *Model) defaultLoaderJavaCursor() int {
	if strings.TrimSpace(m.cfg.Settings.JavaPath) != "" {
		for i, rt := range m.javaRuntimes {
			if strings.EqualFold(rt.Path, m.cfg.Settings.JavaPath) {
				return i + 1
			}
		}
	}
	return 0
}

// loaderJavaPath resolves the runtime used for the Forge installer and the
// OptiFine patcher. Auto prefers a runtime close to what the base version asks
// for, so old Forge installers are not run on the newest Java by accident.
func (m *Model) loaderJavaPath() string {
	if m.loaderPick.Cursor > 0 && m.loaderPick.Cursor <= len(m.javaRuntimes) {
		return m.javaRuntimes[m.loaderPick.Cursor-1].Path
	}
	if strings.TrimSpace(m.cfg.Settings.JavaPath) != "" {
		return m.cfg.Settings.JavaPath
	}
	required := requiredJavaFor(m.loaderPick.Minecraft)
	best := -1
	for i, rt := range m.javaRuntimes {
		if rt.Major < required {
			continue
		}
		if best < 0 || rt.Major < m.javaRuntimes[best].Major {
			best = i
		}
	}
	if best < 0 && len(m.javaRuntimes) > 0 {
		best = 0
		for i, rt := range m.javaRuntimes {
			if rt.Major > m.javaRuntimes[best].Major {
				best = i
			}
		}
	}
	if best >= 0 {
		return m.javaRuntimes[best].Path
	}
	return "java"
}

func requiredJavaFor(minecraft string) int {
	parts := strings.Split(minecraft, ".")
	if len(parts) >= 2 && parts[0] == "1" {
		minor, err := strconv.Atoi(parts[1])
		if err == nil {
			if minor == 20 && len(parts) >= 3 {
				if patch, patchErr := strconv.Atoi(parts[2]); patchErr == nil && patch >= 5 {
					return 21
				}
			}
			switch {
			case minor >= 21:
				return 21
			case minor >= 18:
				return 17
			case minor == 17:
				return 16
			default:
				return 8
			}
		}
	}
	// Snapshots and unknown versions are safest on a current runtime.
	return 21
}

func (m *Model) loaderInstallTitle() string {
	name := loaderKindName(m.loaderPick.Kind)
	if m.loaderPick.Kind == loader.OptiFine {
		if _, edition, ok := loader.ParseOptiFineJarName(filepath.Base(m.loaderPick.OptiFineJar)); ok {
			return m.tf("Installing %s", "OptiFine "+edition)
		}
	}
	return m.tf("Installing %s %s for %s", name, m.loaderPick.LoaderVersion, m.loaderPick.Minecraft)
}

func (m *Model) startLoaderInstall() (tea.Model, tea.Cmd) {
	if m.task != nil {
		m.alert = AlertState{Title: m.t("Another task is running"), Body: m.t("Wait for the current installation or launch to finish.")}
		m.modal = modalAlert
		return m, nil
	}
	pick := m.loaderPick
	if pick.Kind == "" {
		return m, nil
	}
	versionID := ""
	switch pick.Kind {
	case loader.OptiFine:
		_, edition, ok := loader.ParseOptiFineJarName(filepath.Base(pick.OptiFineJar))
		if !ok {
			m.loaderPick.Err = m.t("OptiFine jar not found")
			m.loaderPick.Step = loaderStepKind
			return m, nil
		}
		versionID = loader.OptiFineVersionID(pick.Minecraft, edition)
	default:
		versionID = loaderVersionID(pick.Kind, pick.Minecraft, pick.LoaderVersion)
	}
	if versionID == "" {
		return m, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	task := &TaskState{
		Kind:            taskLoader,
		Title:           m.loaderInstallTitle(),
		VersionID:       versionID,
		LoaderKind:      pick.Kind,
		LoaderMinecraft: pick.Minecraft,
		LoaderVersion:   pick.LoaderVersion,
		LoaderJar:       pick.OptiFineJar,
		Stage:           "Preparing",
		Detail:          m.t("Checking installed files"),
		Started:         time.Now(),
		Cancel:          cancel,
	}
	m.task = task
	m.modal = modalProgress

	layout := m.layout
	client := m.client
	manifest := m.manifest
	threads := m.cfg.Settings.DownloadThreads
	javaPath := m.loaderJavaPath()
	send := m.send

	return m, func() tea.Msg {
		go func() {
			emitter := func(msg tea.Msg) {
				if send != nil {
					send(msg)
				}
			}
			installer := &loader.Installer{
				Layout:        layout,
				Kind:          pick.Kind,
				Minecraft:     pick.Minecraft,
				LoaderVersion: pick.LoaderVersion,
				OptiFineJar:   pick.OptiFineJar,
				JavaPath:      javaPath,
				Client:        client,
				Manifest:      manifest,
				Threads:       threads,
				OnEvent: func(ev install.Event) {
					emitter(installEventMsg{ev: ev})
				},
			}
			err := installer.Install(ctx)
			emitter(taskDoneMsg{
				kind:      taskLoader,
				versionID: versionID,
				err:       err,
				cancelled: errors.Is(err, context.Canceled) || ctx.Err() != nil,
			})
		}()
		return nil
	}
}

func loaderKindName(kind loader.Kind) string {
	switch kind {
	case loader.Fabric:
		return "Fabric"
	case loader.Quilt:
		return "Quilt"
	case loader.Forge:
		return "Forge"
	case loader.OptiFine:
		return "OptiFine"
	}
	return string(kind)
}

func (m *Model) loaderKindBody(contentW int) ([]string, []hitRegion) {
	var body []string
	var hits []hitRegion
	for i, kind := range loader.Kinds {
		selected := i == m.loaderPick.Cursor
		marker := "  "
		if selected {
			marker = stylePrimary.Render("▸ ")
		}
		nameStyle := styleText
		if selected {
			nameStyle = stylePrimaryB
		}
		body = append(body, fitLine("  "+marker+nameStyle.Render(loaderKindName(kind)), contentW))
		hits = append(hits, hitRegion{Kind: hitLoaderKindRow, Index: i, Y: 3 + len(body) - 1, W: contentW, H: 1})
	}
	body = append(body, "")
	return body, hits
}

func (m *Model) loaderVersionBody(contentW int) ([]string, []hitRegion) {
	var body []string
	var hits []hitRegion
	body = append(body, styleMuted.Render("Minecraft "+m.loaderPick.Minecraft), "")
	if m.loaderPick.Loading {
		body = append(body, stylePrimary.Render(spinnerFrame(m.spinner))+" "+styleMuted.Render(m.t("loading…")))
		return body, hits
	}
	if m.loaderPick.Err != "" {
		body = append(body, styleRed.Render("✗ "+util.Truncate(m.loaderPick.Err, contentW-4)))
	}
	if len(m.loaderPick.Versions) == 0 {
		body = append(body, styleMuted.Render(m.t("No loader builds found for this Minecraft version.")))
		return body, hits
	}
	count := len(m.loaderPick.Versions)
	visible := m.loaderVisibleRows()
	m.loaderPick.Cursor = clampInt(m.loaderPick.Cursor, 0, count-1)
	start := m.loaderPick.Cursor - visible/2
	if start < 0 {
		start = 0
	}
	if start+visible > count {
		start = count - visible
	}
	if start < 0 {
		start = 0
	}
	end := start + visible
	if end > count {
		end = count
	}
	for i := start; i < end; i++ {
		item := m.loaderPick.Versions[i]
		selected := i == m.loaderPick.Cursor
		marker := "  "
		if selected {
			marker = stylePrimary.Render("▸ ")
		}
		label := styleText.Render(item.Version)
		if selected {
			label = stylePrimaryB.Render(item.Version)
		}
		tag := m.t("stable")
		tagStyle := styleGreen
		if !item.Stable {
			tag = m.t("unstable")
			tagStyle = styleYellow
		}
		versionID := loaderVersionID(m.loaderPick.Kind, m.loaderPick.Minecraft, item.Version)
		right := tagStyle.Render(tag)
		if m.installed[versionID] {
			right += styleDim.Render(" · " + m.t("installed"))
		}
		body = append(body, composeLR("", marker+label, right, contentW))
		hits = append(hits, hitRegion{Kind: hitLoaderVersionRow, Index: i, Y: 3 + len(body) - 1, W: contentW, H: 1})
	}
	if count > visible {
		body = append(body, styleDim.Render(fmt.Sprintf("  %d / %d", m.loaderPick.Cursor+1, count)))
	}
	return body, hits
}

func (m *Model) loaderVisibleRows() int {
	visible := m.height - 15
	if visible < 5 {
		visible = 5
	}
	return visible
}

func (m *Model) loaderJavaBody(contentW int) ([]string, []hitRegion) {
	var body []string
	var hits []hitRegion
	body = append(body, styleMuted.Render(m.t("Choose the Java runtime used to run the loader installer.")), "")
	count := len(m.javaRuntimes) + 1
	visible := m.loaderVisibleRows()
	m.loaderPick.Cursor = clampInt(m.loaderPick.Cursor, 0, count-1)
	start := m.loaderPick.Cursor - visible/2
	if start < 0 {
		start = 0
	}
	if start+visible > count {
		start = count - visible
	}
	if start < 0 {
		start = 0
	}
	required := requiredJavaFor(m.loaderPick.Minecraft)
	end := start + visible
	if end > count {
		end = count
	}
	for i := start; i < end; i++ {
		selected := i == m.loaderPick.Cursor
		marker := "  "
		if selected {
			marker = stylePrimary.Render("▸ ")
		}
		var label, right string
		if i == 0 {
			label = m.t("Auto-detect (recommended)")
			right = m.t("Auto-detect")
		} else {
			rt := m.javaRuntimes[i-1]
			label = rt.Short()
			right = util.ShortenPath(rt.Path, max(12, contentW/2))
			if rt.Major < required {
				right = m.tf("Java %d is older than Java %d", rt.Major, required)
			}
		}
		labelStyle := styleText
		if selected {
			labelStyle = stylePrimaryB
		}
		body = append(body, composeLR("", marker+labelStyle.Render(label), styleDim.Render(right), contentW))
		hits = append(hits, hitRegion{Kind: hitLoaderJavaRow, Index: i, Y: 3 + len(body) - 1, W: contentW, H: 1})
	}
	if count > visible {
		body = append(body, styleDim.Render(fmt.Sprintf("  %d / %d", m.loaderPick.Cursor+1, count)))
	}
	return body, hits
}
