package tui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/MoonCC233/MCLauncher-TUI/internal/config"
	"github.com/MoonCC233/MCLauncher-TUI/internal/loader"
	"github.com/MoonCC233/MCLauncher-TUI/internal/mods"
	"github.com/MoonCC233/MCLauncher-TUI/internal/util"
)

func (m *Model) openMods(inst *config.Instance) {
	if inst == nil {
		return
	}
	m.modState = modsState{
		InstanceID:   inst.ID,
		InstanceName: inst.Name,
	}
	m.reloadMods("")
	m.modal = modalMods
}

func (m *Model) reloadMods(selectName string) {
	items, err := mods.Scan(m.layout.ModsDir(m.modState.InstanceID))
	m.modState.Items = items
	m.modState.Err = ""
	if err != nil {
		m.modState.Err = err.Error()
	}
	if selectName != "" {
		for i := range items {
			if items[i].FileName == selectName {
				m.modState.Cursor = i
				break
			}
		}
	}
	m.modState.Cursor = clampInt(m.modState.Cursor, 0, max(0, len(items)-1))
	m.modState.Offset = 0
}

func (m *Model) currentMod() *mods.Mod {
	if len(m.modState.Items) == 0 {
		return nil
	}
	m.modState.Cursor = clampInt(m.modState.Cursor, 0, len(m.modState.Items)-1)
	return &m.modState.Items[m.modState.Cursor]
}

func (m *Model) updateMods(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.modal = modalNone
		return m, nil
	case "up", "k":
		m.modState.Cursor--
	case "down", "j":
		m.modState.Cursor++
	case "pgup":
		m.modState.Cursor -= m.modsVisibleRows()
	case "pgdown":
		m.modState.Cursor += m.modsVisibleRows()
	case "home":
		m.modState.Cursor = 0
	case "end":
		m.modState.Cursor = len(m.modState.Items) - 1
	case "a":
		m.openPrompt(promptModPath, m.t("Add mod"), "",
			m.t("Full path to a .jar mod file"), 260)
		return m, nil
	case "d":
		mod := m.currentMod()
		if mod == nil {
			return m, nil
		}
		m.confirm = ConfirmState{
			Kind:   confirmRemoveMod,
			Title:  m.t("Remove mod?"),
			Body:   m.tf("Delete %s from this instance? The file is removed from disk.", mod.Name),
			Target: mod.FileName,
			Aux:    mod.Name,
		}
		m.modal = modalConfirm
		return m, nil
	case " ", "t":
		return m, m.toggleMod()
	case "r":
		m.reloadMods("")
		m.setToast(m.t("Mods reloaded"), toastInfo)
		return m, nil
	case "o":
		_ = util.OpenPath(m.layout.ModsDir(m.modState.InstanceID))
		return m, nil
	}
	m.modState.Cursor = clampInt(m.modState.Cursor, 0, max(0, len(m.modState.Items)-1))
	return m, nil
}

func (m *Model) toggleMod() tea.Cmd {
	mod := m.currentMod()
	if mod == nil {
		return nil
	}
	dir := m.layout.ModsDir(m.modState.InstanceID)
	target := !mod.Enabled
	newName, err := mods.SetEnabled(dir, mod.FileName, target)
	if err != nil {
		m.modState.Err = err.Error()
		return nil
	}
	m.reloadMods(newName)
	if target {
		m.setToast(m.tf("Enabled %s", mod.Name), toastSuccess)
	} else {
		m.setToast(m.tf("Disabled %s", mod.Name), toastSuccess)
	}
	return nil
}

func (m *Model) addModFromPath(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return errors.New(m.t("Enter a path to a .jar file"))
	}
	if _, err := mods.Add(m.layout.ModsDir(m.modState.InstanceID), value); err != nil {
		if errors.Is(err, mods.ErrExists) {
			return errors.New(m.t("A mod with this file name already exists"))
		}
		return err
	}
	m.reloadMods(filepath.Base(value))
	m.modal = modalMods
	m.setToast(m.t("Mod added"), toastSuccess)
	return nil
}

func (m *Model) chooseOptiFineJar(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return errors.New(m.t("Enter the path to an OptiFine jar"))
	}
	info, err := os.Stat(value)
	if err != nil || info.IsDir() {
		return errors.New(m.t("OptiFine jar not found"))
	}
	minecraft, _, ok := loader.ParseOptiFineJarName(filepath.Base(value))
	if !ok {
		return errors.New(m.t("The file name must look like OptiFine_<version>_<edition>.jar"))
	}
	if m.loaderPick.Minecraft != "" && m.loaderPick.Minecraft != minecraft {
		return fmt.Errorf("%s", m.tf("This jar is for Minecraft %s, not %s", minecraft, m.loaderPick.Minecraft))
	}
	m.loaderPick.Minecraft = minecraft
	m.loaderPick.OptiFineJar = value
	m.loaderPick.Step = loaderStepJava
	m.loaderPick.Cursor = m.defaultLoaderJavaCursor()
	m.loaderPick.Err = ""
	m.modal = modalLoader
	return nil
}

func (m *Model) modsVisibleRows() int {
	visible := m.height - 15
	if visible < 5 {
		visible = 5
	}
	return visible
}

func (m *Model) modsBody(contentW int) ([]string, []hitRegion) {
	var body []string
	var hits []hitRegion
	dir := m.layout.ModsDir(m.modState.InstanceID)
	body = append(body, styleMuted.Render(util.ShortenPath(dir, contentW)), "")
	if m.modState.Err != "" {
		body = append(body, styleRed.Render("✗ "+util.Truncate(m.modState.Err, contentW-4)), "")
	}
	if len(m.modState.Items) == 0 {
		body = append(body,
			styleMuted.Render(m.t("No mods installed.")),
			styleDim.Render(m.t("Press a to add a .jar file, or drop mods into the folder above.")),
		)
		return body, hits
	}

	count := len(m.modState.Items)
	visible := m.modsVisibleRows()
	m.modState.Cursor = clampInt(m.modState.Cursor, 0, count-1)
	start := m.modState.Cursor - visible/2
	if start < 0 {
		start = 0
	}
	if start+visible > count {
		start = count - visible
	}
	if start < 0 {
		start = 0
	}
	m.modState.Offset = start
	end := start + visible
	if end > count {
		end = count
	}
	for i := start; i < end; i++ {
		mod := m.modState.Items[i]
		selected := i == m.modState.Cursor
		marker := "  "
		if selected {
			marker = stylePrimary.Render("▸ ")
		}
		check := styleGreen.Render("●")
		if !mod.Enabled {
			check = styleDim.Render("○")
		}
		nameStyle := styleText
		if selected {
			nameStyle = stylePrimaryB
		}
		left := marker + check + " " + nameStyle.Render(mod.Name)
		if mod.Version != "" {
			left += styleDim.Render(" " + mod.Version)
		}
		var tags []string
		for _, kind := range mod.Loaders {
			tags = append(tags, loaderKindName(loader.Kind(kind)))
		}
		right := strings.Join(tags, styleBorder.Render(" · "))
		if !mod.Enabled {
			right += styleDim.Render(" · " + m.t("disabled"))
		}
		body = append(body, composeLR("", left, styleDim.Render(right), contentW))
		if ansi.StringWidth(left)+ansi.StringWidth(right)+2 < contentW {
			body = append(body, "  "+styleDim.Render(util.Truncate(mod.FileName, max(8, contentW-4))))
			hits = append(hits, hitRegion{Kind: hitModRow, Index: i, Y: 3 + len(body) - 2, W: contentW, H: 2})
		} else {
			hits = append(hits, hitRegion{Kind: hitModRow, Index: i, Y: 3 + len(body) - 1, W: contentW, H: 1})
		}
	}
	if count > visible {
		body = append(body, styleDim.Render(fmt.Sprintf("  %d / %d", m.modState.Cursor+1, count)))
	}
	return body, hits
}
