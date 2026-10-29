package tui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MoonCC233/MCLauncher-TUI/internal/loader"
)

// chooseOptiFineJar validates an OptiFine installer path and moves the loader
// picker to its Java step.
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
