//go:build manual

package loader

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MoonCC233/MCLauncher-TUI/internal/config"
	"github.com/MoonCC233/MCLauncher-TUI/internal/install"
	"github.com/MoonCC233/MCLauncher-TUI/internal/java"
	"github.com/MoonCC233/MCLauncher-TUI/internal/launch"
	"github.com/MoonCC233/MCLauncher-TUI/internal/mojang"
	"github.com/MoonCC233/MCLauncher-TUI/internal/paths"
	"github.com/MoonCC233/MCLauncher-TUI/internal/util"
)

func TestManualNeoForgeVersions(t *testing.T) {
	if os.Getenv("MCLAUNCHER_NEOFORGE_MIRROR") == "" {
		t.Skip("set MCLAUNCHER_NEOFORGE_MIRROR to exercise the NeoForge metadata endpoint")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	client := util.NewHTTPClient()
	for _, minecraft := range []string{"1.20.1", "1.21.1"} {
		versions, err := FetchVersions(ctx, client, NeoForge, minecraft)
		if err != nil {
			t.Fatalf("Minecraft %s: %v", minecraft, err)
		}
		if len(versions) == 0 {
			t.Fatalf("Minecraft %s: no NeoForge versions", minecraft)
		}
		t.Logf("Minecraft %s: %d versions, newest %s", minecraft, len(versions), versions[0].Version)
	}
}

func TestManualInstall(t *testing.T) {
	root := os.Getenv("MANUAL_ROOT")
	if root == "" {
		root = filepath.Join(os.TempDir(), "mclauncher-loader-manual")
	}
	kind := Kind(os.Getenv("MANUAL_KIND"))
	if kind == "" {
		kind = Forge
	}
	mc := os.Getenv("MANUAL_MC")
	if mc == "" {
		mc = "1.20.1"
	}
	loaderVersion := os.Getenv("MANUAL_LOADER")
	if loaderVersion == "" && kind == Forge {
		loaderVersion = "47.2.0"
	}
	optifineJar := os.Getenv("MANUAL_OPTIFINE")
	javaPath := os.Getenv("MANUAL_JAVA")
	if javaPath == "" {
		javaPath = "java"
	}
	layout := paths.New(root)
	client := util.NewHTTPClient()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	defer cancel()
	t.Logf("data root: %s", root)

	manifest, err := mojang.FetchManifest(ctx, client, layout, false)
	if err != nil {
		t.Fatal(err)
	}
	in := &Installer{
		Layout:        layout,
		Kind:          kind,
		Minecraft:     mc,
		LoaderVersion: loaderVersion,
		OptiFineJar:   optifineJar,
		JavaPath:      javaPath,
		Client:        client,
		Manifest:      manifest,
		Threads:       8,
		OnEvent: func(ev install.Event) {
			if ev.Type == install.EventStage {
				t.Logf("stage: %s · %s", ev.Stage, ev.Detail)
			}
		},
	}
	if err := in.Install(ctx); err != nil {
		t.Fatal(err)
	}
	versionID := in.VersionID()
	t.Logf("installed %s", versionID)
	if !install.IsInstalled(layout, versionID) {
		t.Fatal("IsInstalled returned false")
	}
	v, err := mojang.LoadVersionJSON(layout, versionID)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("main=%s jar=%s libraries=%d", v.MainClass, v.ClientJarID(versionID), len(v.Libraries))
	runtime := java.Info{Path: javaPath}
	spec, err := launch.Build(launch.Options{
		Layout:          layout,
		Version:         v,
		VersionID:       versionID,
		Account:         config.Account{Username: "Dev", UUID: "00000000-0000-0000-0000-000000000000"},
		Java:            runtime,
		GameDir:         filepath.Join(root, "instances", "manual"),
		AssetsDir:       layout.Assets,
		MemoryMB:        2048,
		Width:           854,
		Height:          480,
		LauncherVersion: "manual",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("command: %s", spec.CommandLine())
}
