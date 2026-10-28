package mojang

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MoonCC233/MCLauncher-TUI/internal/paths"
)

func writeVersionFile(t *testing.T, layout paths.Layout, id, body string) {
	t.Helper()
	path := layout.VersionJSON(id)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadResolvedVersionJSON(t *testing.T) {
	layout := paths.New(t.TempDir())
	writeVersionFile(t, layout, "1.21.4", `{
		"id": "1.21.4",
		"type": "release",
		"mainClass": "net.minecraft.client.main.Main",
		"assets": "17",
		"downloads": {"client": {"url": "https://example.invalid/client.jar", "size": 42}},
		"libraries": [
			{"name": "org.example:parent:1.0"},
			{"name": "org.example:shared:1.0", "url": "https://parent.invalid/"}
		],
		"arguments": {
			"game": ["--gameDir", "${game_directory}"],
			"jvm": ["-Dparent=1"]
		}
	}`)
	writeVersionFile(t, layout, "fabric-loader-0.16.10-1.21.4", `{
		"id": "fabric-loader-0.16.10-1.21.4",
		"inheritsFrom": "1.21.4",
		"mainClass": "net.fabricmc.loader.impl.launch.knot.KnotClient",
		"libraries": [
			{"name": "org.example:shared:1.0", "url": "https://child.invalid/"},
			{"name": "org.example:child:1.0"}
		],
		"arguments": {
			"game": ["--fabric"],
			"jvm": ["-Dchild=1"]
		}
	}`)

	merged, jarID, err := LoadResolvedVersionJSON(layout, "fabric-loader-0.16.10-1.21.4")
	if err != nil {
		t.Fatalf("LoadResolvedVersionJSON: %v", err)
	}
	if jarID != "1.21.4" {
		t.Errorf("jarID = %q, want 1.21.4", jarID)
	}
	if merged.ID != "fabric-loader-0.16.10-1.21.4" || merged.InheritsFrom != "" {
		t.Errorf("merged identity = %q inherits %q", merged.ID, merged.InheritsFrom)
	}
	if merged.MainClass != "net.fabricmc.loader.impl.launch.knot.KnotClient" {
		t.Errorf("main class = %q", merged.MainClass)
	}
	if merged.Assets != "17" || merged.Downloads.Client.Size != 42 {
		t.Errorf("parent scalars missing: %+v", merged)
	}
	if len(merged.Libraries) != 3 {
		t.Fatalf("libraries = %+v, want 3 entries", merged.Libraries)
	}
	if merged.Libraries[1].Name != "org.example:shared:1.0" || merged.Libraries[1].URL != "https://child.invalid/" {
		t.Errorf("child library should override parent in place: %+v", merged.Libraries[1])
	}
	if got := merged.Arguments.Game; len(got) != 3 || got[0].Values[0] != "--gameDir" || got[2].Values[0] != "--fabric" {
		t.Errorf("game arguments = %+v", got)
	}
	if got := merged.Arguments.JVM; len(got) != 2 || got[0].Values[0] != "-Dparent=1" || got[1].Values[0] != "-Dchild=1" {
		t.Errorf("JVM arguments = %+v", got)
	}

	wantJar := filepath.Join(layout.Versions, "1.21.4", "1.21.4.jar")
	if got := ClientJarFor(layout, "fabric-loader-0.16.10-1.21.4"); got != wantJar {
		t.Errorf("ClientJarFor = %q, want %q", got, wantJar)
	}
}

func TestInheritanceCycle(t *testing.T) {
	layout := paths.New(t.TempDir())
	writeVersionFile(t, layout, "a", `{"id":"a","inheritsFrom":"b","mainClass":"A","libraries":[{"name":"a:a:1"}]}`)
	writeVersionFile(t, layout, "b", `{"id":"b","inheritsFrom":"a","mainClass":"B","libraries":[{"name":"b:b:1"}]}`)
	if _, _, err := LoadResolvedVersionJSON(layout, "a"); err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("cycle error = %v", err)
	}
	if _, err := ClientJarID(layout, "a"); err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("ClientJarID cycle error = %v", err)
	}
}

func TestClientJarFallsBack(t *testing.T) {
	layout := paths.New(t.TempDir())
	want := layout.ClientJar("missing")
	if got := ClientJarFor(layout, "missing"); got != want {
		t.Errorf("ClientJarFor(missing) = %q, want %q", got, want)
	}
}
