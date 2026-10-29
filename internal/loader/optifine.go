package loader

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/MoonCC233/MCLauncher-TUI/internal/install"
	"github.com/MoonCC233/MCLauncher-TUI/internal/mojang"
	"github.com/MoonCC233/MCLauncher-TUI/internal/util"
)

var launchWrapperEntryRE = regexp.MustCompile(`(?i)^launchwrapper-of-(.+)\.jar$`)

func (in *Installer) installOptiFine(ctx context.Context) (string, error) {
	source := strings.TrimSpace(in.OptiFineJar)
	if st, err := os.Stat(source); err != nil || st.IsDir() {
		return "", fmt.Errorf("OptiFine jar not found: %s", source)
	}
	minecraft, edition, ok := ParseOptiFineJarName(filepath.Base(source))
	if !ok {
		return "", fmt.Errorf("cannot read the Minecraft version from %s (expected OptiFine_<version>_<edition>.jar)", filepath.Base(source))
	}
	if in.Minecraft != "" && in.Minecraft != minecraft {
		return "", fmt.Errorf("OptiFine jar is for Minecraft %s, not %s", minecraft, in.Minecraft)
	}
	in.Minecraft = minecraft
	versionID := OptiFineVersionID(minecraft, edition)

	in.emit(install.Event{Type: install.EventStage, Stage: "Reading OptiFine jar", Detail: edition})
	launchWrapperVersion, launchWrapperData, err := readLaunchWrapper(source)
	if err != nil {
		return "", err
	}

	in.emit(install.Event{Type: install.EventStage, Stage: "Preparing OptiFine libraries", Detail: edition})
	optifineRel := filepath.ToSlash(filepath.Join("optifine", "OptiFine", minecraft+"_"+edition, "OptiFine-"+minecraft+"_"+edition+".jar"))
	optifineDest := filepath.Join(in.Layout.Libraries, filepath.FromSlash(optifineRel))
	if err := copyFile(source, optifineDest); err != nil {
		return "", fmt.Errorf("copy OptiFine jar: %w", err)
	}
	launcherRel := filepath.ToSlash(filepath.Join("optifine", "launchwrapper-of", launchWrapperVersion, "launchwrapper-of-"+launchWrapperVersion+".jar"))
	if _, _, err := writeLibrary(in.Layout, launcherRel, launchWrapperData); err != nil {
		return "", fmt.Errorf("write launchwrapper: %w", err)
	}

	baseJar := in.Layout.ClientJar(minecraft)
	if st, err := os.Stat(baseJar); err != nil || st.Size() == 0 {
		return "", fmt.Errorf("client jar for Minecraft %s is missing", minecraft)
	}
	patchedJar := in.Layout.ClientJar(versionID)
	in.emit(install.Event{Type: install.EventStage, Stage: "Patching Minecraft", Detail: edition})
	if err := util.EnsureDir(filepath.Dir(patchedJar)); err != nil {
		return "", err
	}
	_ = os.Remove(patchedJar)
	javaPath := in.JavaPath
	if javaPath == "" {
		javaPath = "java"
	}
	args := []string{"-cp", source, "optifine.Patcher", baseJar, source, patchedJar}
	if err := runJava(ctx, javaPath, args, filepath.Dir(source), func(line string) {
		in.emit(install.Event{Type: install.EventLog, Stage: "Patching Minecraft", Detail: line})
	}); err != nil {
		return "", fmt.Errorf("OptiFine patcher: %w", err)
	}
	if st, err := os.Stat(patchedJar); err != nil || st.Size() == 0 {
		return "", fmt.Errorf("OptiFine patcher did not create %s", filepath.Base(patchedJar))
	}

	base, err := mojang.LoadVersionJSON(in.Layout, minecraft)
	if err != nil {
		return "", fmt.Errorf("load base version %s: %w", minecraft, err)
	}
	profile := mojang.VersionJSON{
		ID:           versionID,
		InheritsFrom: minecraft,
		JarID:        versionID,
		Type:         base.TypeLabel(),
		MainClass:    "net.minecraft.launchwrapper.Launch",
		Libraries: []mojang.Library{
			artifactRef(in.Layout, "optifine:OptiFine:"+minecraft+"_"+edition, optifineRel),
			artifactRef(in.Layout, "optifine:launchwrapper-of:"+launchWrapperVersion, launcherRel),
		},
	}
	if base.Arguments != nil && len(base.Arguments.Game) > 0 {
		profile.Arguments = &mojang.Arguments{
			Game: []mojang.Argument{{Values: []string{"--tweakClass", "optifine.OptiFineTweaker"}}},
		}
	} else {
		legacy := strings.TrimSpace(base.MinecraftArguments)
		if legacy == "" {
			legacy = "--username ${auth_player_name} --version ${version_name} --gameDir ${game_directory} --assetsDir ${assets_root} --assetIndex ${assets_index_name} --uuid ${auth_uuid} --accessToken ${auth_access_token} --userType ${user_type}"
		}
		profile.MinecraftArguments = legacy + " --tweakClass optifine.OptiFineTweaker"
	}
	if err := util.WriteJSONAtomic(in.Layout.VersionJSON(versionID), &profile); err != nil {
		return "", err
	}
	meta := Meta{
		Kind:          OptiFine,
		Minecraft:     minecraft,
		LoaderVersion: launchWrapperVersion,
		Edition:       edition,
		VersionID:     versionID,
		InstalledAt:   time.Now().Unix(),
	}
	if err := WriteMeta(in.Layout, meta); err != nil {
		return "", err
	}
	return versionID, nil
}

func readLaunchWrapper(path string) (version string, data []byte, err error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return "", nil, fmt.Errorf("open OptiFine jar: %w", err)
	}
	defer reader.Close()
	for _, file := range reader.File {
		if strings.EqualFold(file.Name, "launchwrapper-of.txt") {
			rc, openErr := file.Open()
			if openErr != nil {
				return "", nil, openErr
			}
			raw, readErr := io.ReadAll(io.LimitReader(rc, 1024))
			rc.Close()
			if readErr != nil {
				return "", nil, readErr
			}
			version = strings.TrimSpace(string(raw))
			continue
		}
		if m := launchWrapperEntryRE.FindStringSubmatch(file.Name); m != nil {
			rc, openErr := file.Open()
			if openErr != nil {
				return "", nil, openErr
			}
			raw, readErr := io.ReadAll(io.LimitReader(rc, 32<<20))
			rc.Close()
			if readErr != nil {
				return "", nil, readErr
			}
			if version == "" {
				version = m[1]
			}
			data = raw
		}
	}
	if version == "" {
		return "", nil, fmt.Errorf("OptiFine jar has no launchwrapper-of version")
	}
	if data == nil {
		return "", nil, fmt.Errorf("OptiFine jar has no launchwrapper-of jar")
	}
	return version, data, nil
}
