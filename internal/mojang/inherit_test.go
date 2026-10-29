package mojang

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MoonCC233/MCLauncher-TUI/internal/paths"
	"github.com/MoonCC233/MCLauncher-TUI/internal/util"
)

func writeVersion(t *testing.T, layout paths.Layout, id string, v *VersionJSON) {
	t.Helper()
	if err := util.WriteJSONAtomic(layout.VersionJSON(id), v); err != nil {
		t.Fatal(err)
	}
}

func TestLoadVersionJSONMergesInheritance(t *testing.T) {
	layout := paths.New(t.TempDir())
	parent := &VersionJSON{
		ID:        "1.20.1",
		MainClass: "net.minecraft.client.main.Main",
		Downloads: VersionDownloads{Client: DownloadInfo{
			URL: "https://example.com/client.jar", SHA1: "abc", Size: 10,
		}},
		Libraries: []Library{
			{Name: "net.minecraft:client:1.20.1", Downloads: LibraryDownloads{Artifact: DownloadInfo{Path: "a.jar"}}},
			{Name: "org.lwjgl:lwjgl:3.3.1", Downloads: LibraryDownloads{Artifact: DownloadInfo{Path: "b.jar"}}},
		},
		Arguments: &Arguments{
			Game: []Argument{{Values: []string{"--username", "${auth_player_name}"}}},
			JVM:  []Argument{{Values: []string{"-cp", "${classpath}"}}},
		},
	}
	child := &VersionJSON{
		ID:           "fabric-loader-0.15-1.20.1",
		InheritsFrom: "1.20.1",
		MainClass:    "net.fabricmc.loader.impl.launch.knot.KnotClient",
		Libraries: []Library{
			{Name: "net.fabricmc:fabric-loader:0.15", URL: "https://maven.fabricmc.net/"},
			// Replacing a parent library keeps its position.
			{Name: "org.lwjgl:lwjgl:3.3.1", Downloads: LibraryDownloads{Artifact: DownloadInfo{Path: "b2.jar"}}},
		},
		Arguments: &Arguments{
			Game: []Argument{{Values: []string{"--tweakClass", "x"}}},
			JVM:  []Argument{{Values: []string{"-DFabricMcEmu= net.minecraft.client.main.Main "}}},
		},
	}
	writeVersion(t, layout, "1.20.1", parent)
	writeVersion(t, layout, "fabric-loader-0.15-1.20.1", child)

	merged, err := LoadVersionJSON(layout, "fabric-loader-0.15-1.20.1")
	if err != nil {
		t.Fatal(err)
	}
	if merged.MainClass != child.MainClass {
		t.Errorf("main class = %q", merged.MainClass)
	}
	if merged.JarID != "1.20.1" {
		t.Errorf("jar id = %q, want 1.20.1", merged.JarID)
	}
	if len(merged.Libraries) != 3 {
		t.Fatalf("libraries = %d, want 3", len(merged.Libraries))
	}
	if merged.Libraries[1].Downloads.Artifact.Path != "b2.jar" {
		t.Errorf("replaced library kept parent value: %+v", merged.Libraries[1])
	}
	if got := len(merged.Arguments.Game); got != 2 {
		t.Errorf("game args = %d, want 2", got)
	}
	if got := len(merged.Arguments.JVM); got != 2 {
		t.Errorf("jvm args = %d, want 2", got)
	}
	if merged.Downloads.Client.URL != parent.Downloads.Client.URL {
		t.Error("parent client download should be inherited")
	}
}

func TestLoadVersionJSONDetectsCycle(t *testing.T) {
	layout := paths.New(t.TempDir())
	writeVersion(t, layout, "a", &VersionJSON{ID: "a", InheritsFrom: "b", MainClass: "A"})
	writeVersion(t, layout, "b", &VersionJSON{ID: "b", InheritsFrom: "a", MainClass: "B"})
	if _, err := LoadVersionJSON(layout, "a"); err == nil {
		t.Fatal("expected an inheritance cycle error")
	}
}

func TestMergeVersionsKeepsExplicitJarID(t *testing.T) {
	parent := &VersionJSON{ID: "1.20.1", MainClass: "P", Downloads: VersionDownloads{Client: DownloadInfo{URL: "u"}}}
	child := &VersionJSON{ID: "1.20.1-OptiFine-HD_U_I6", InheritsFrom: "1.20.1", MainClass: "net.minecraft.launchwrapper.Launch", JarID: "1.20.1-OptiFine-HD_U_I6"}
	merged := MergeVersions(parent, child)
	if merged.JarID != child.JarID {
		t.Errorf("jar id = %q, want %q", merged.JarID, child.JarID)
	}
	if merged.ClientJarID("fallback") != child.JarID {
		t.Errorf("ClientJarID = %q", merged.ClientJarID("fallback"))
	}
}

func TestClientJarPathUsesVersionDir(t *testing.T) {
	layout := paths.New(filepath.Join(t.TempDir(), "data"))
	want := filepath.Join(layout.Root, "versions", "1.20.1", "1.20.1.jar")
	if got := ClientJarPath(layout, "1.20.1"); got != want {
		t.Errorf("ClientJarPath = %q, want %q", got, want)
	}
	if _, err := os.Stat(layout.Root); !os.IsNotExist(err) {
		t.Error("ClientJarPath should not create directories")
	}
}
