package loader

import (
	"archive/zip"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/MoonCC233/MCLauncher-TUI/internal/mojang"
	"github.com/MoonCC233/MCLauncher-TUI/internal/paths"
	"github.com/MoonCC233/MCLauncher-TUI/internal/util"
)

func TestValidProfileID(t *testing.T) {
	for _, id := range []string{"1.21.4", "fabric-loader-0.16.10-1.21.4", "21.1.209", "1.20.1-forge-47.2.0"} {
		if !validProfileID(id) {
			t.Errorf("validProfileID(%q) = false", id)
		}
	}
	for _, id := range []string{"", ".", "..", "a/b", "a b", "a\\b", "a..b/evil"} {
		if validProfileID(id) {
			t.Errorf("validProfileID(%q) = true", id)
		}
	}
}

func TestNeoForgePrefix(t *testing.T) {
	tests := []struct {
		game   string
		prefix string
		legacy bool
	}{
		{"1.20.1", "1.20.1-", true},
		{"1.20.2", "20.2.", false},
		{"1.20.4", "20.4.", false},
		{"1.21", "21.0.", false},
		{"1.21.4", "21.4.", false},
		{"26.1", "26.1.", false},
		{"garbage", "", false},
	}
	for _, test := range tests {
		prefix, legacy := neoForgePrefix(test.game)
		if prefix != test.prefix || legacy != test.legacy {
			t.Errorf("neoForgePrefix(%q) = %q, %v; want %q, %v", test.game, prefix, legacy, test.prefix, test.legacy)
		}
	}
}

func TestFilterNeoForgeVersions(t *testing.T) {
	all := []string{"21.4.209", "21.4.100-beta", "21.4.0", "20.4.237", "1.20.1-47.2.0", "bad/version"}
	versions := filterNeoForgeVersions(all, "1.21.4", false)
	if len(versions) != 3 {
		t.Fatalf("filtered versions = %+v", versions)
	}
	if versions[0].Version != "21.4.209" || !versions[0].Stable {
		t.Errorf("first version = %+v", versions[0])
	}
	if versions[1].Version != "21.4.100-beta" || versions[1].Stable {
		t.Errorf("beta version = %+v", versions[1])
	}
	if versions[2].Version != "21.4.0" {
		t.Errorf("last version = %+v", versions[2])
	}

	legacy := filterNeoForgeVersions(all, "1.20.1", true)
	if len(legacy) != 1 || legacy[0].Version != "1.20.1-47.2.0" {
		t.Errorf("legacy versions = %+v", legacy)
	}
}

func TestNeoForgeArtifactFor(t *testing.T) {
	newArtifact, err := neoForgeArtifactFor("1.21.4", "21.4.209")
	if err != nil {
		t.Fatal(err)
	}
	if newArtifact.Version != "21.4.209" {
		t.Errorf("version = %q", newArtifact.Version)
	}
	wantNew := "https://maven.neoforged.net/releases/net/neoforged/neoforge/21.4.209/neoforge-21.4.209-installer.jar"
	if newArtifact.URL != wantNew {
		t.Errorf("URL = %q, want %q", newArtifact.URL, wantNew)
	}
	if newArtifact.File != "neoforge-21.4.209-installer.jar" {
		t.Errorf("File = %q", newArtifact.File)
	}

	legacyArtifact, err := neoForgeArtifactFor("1.20.1", "47.2.0")
	if err != nil {
		t.Fatal(err)
	}
	if legacyArtifact.Version != "1.20.1-47.2.0" {
		t.Errorf("legacy version = %q", legacyArtifact.Version)
	}
	wantLegacy := "https://maven.neoforged.net/releases/net/neoforged/forge/1.20.1-47.2.0/forge-1.20.1-47.2.0-installer.jar"
	if legacyArtifact.URL != wantLegacy {
		t.Errorf("legacy URL = %q, want %q", legacyArtifact.URL, wantLegacy)
	}
	if legacyArtifact.File != "forge-1.20.1-47.2.0-installer.jar" {
		t.Errorf("legacy File = %q", legacyArtifact.File)
	}

	if _, err := neoForgeArtifactFor("1.21.4", "../bad"); err == nil {
		t.Error("invalid version should be rejected")
	}
}

func TestFabricVersionsAndInstall(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/versions/loader/1.21.4":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[
				{"loader": {"version": "0.16.10", "stable": true}},
				{"loader": {"version": "0.17.0-beta.1", "stable": false}},
				{"loader": {"version": "bad/version", "stable": true}}
			]`))
		case "/versions/loader/1.21.4/0.16.10/profile/json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"id": "fabric-loader-0.16.10-1.21.4",
				"inheritsFrom": "1.21.4",
				"mainClass": "net.fabricmc.loader.impl.launch.knot.KnotClient",
				"libraries": [{"name": "net.fabricmc:fabric-loader:0.16.10"}],
				"arguments": {
					"jvm": [{"rules": [{"action": "allow", "os": {"name": "windows"}}], "value": "-Dfabric=1"}]
				}
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	oldBase := fabricMetaBase
	fabricMetaBase = server.URL
	t.Cleanup(func() { fabricMetaBase = oldBase })

	versions, err := fabricVersions(context.Background(), server.Client(), "1.21.4")
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 2 || versions[0].Version != "0.16.10" || !versions[0].Stable || versions[1].Stable {
		t.Errorf("versions = %+v", versions)
	}

	layout := paths.New(t.TempDir())
	var stages []string
	result, err := Install(context.Background(), Options{
		Kind:          Fabric,
		GameVersion:   "1.21.4",
		LoaderVersion: "0.16.10",
		Layout:        layout,
		Client:        server.Client(),
		OnStage:       func(stage, detail string) { stages = append(stages, stage) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.VersionID != "fabric-loader-0.16.10-1.21.4" || result.Loader != Fabric {
		t.Errorf("result = %+v", result)
	}
	if len(stages) == 0 || stages[0] != "Fetching Fabric profile" {
		t.Errorf("stages = %+v", stages)
	}
	var profile mojang.VersionJSON
	if err := util.ReadJSON(layout.VersionJSON(result.VersionID), &profile); err != nil {
		t.Fatal(err)
	}
	if profile.InheritsFrom != "1.21.4" || profile.MainClass == "" {
		t.Errorf("profile = %+v", profile)
	}
	if profile.Arguments == nil || len(profile.Arguments.JVM) != 1 || profile.Arguments.JVM[0].Values[0] != "-Dfabric=1" {
		t.Errorf("profile arguments = %+v", profile.Arguments)
	}
}

func TestReadInstallProfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "installer.jar")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	entry, err := writer.Create("install_profile.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte(`{"version":"1.21.4-neoforge-21.1.209","minecraft":"1.21.4","install":{"version":"21.1.209"}}`)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	profile, err := readInstallProfile(path)
	if err != nil {
		t.Fatal(err)
	}
	if profile.Version != "1.21.4-neoforge-21.1.209" || profile.Minecraft != "1.21.4" || profile.Install.Version != "21.1.209" {
		t.Errorf("profile = %+v", profile)
	}
}

func TestPrepareLauncherProfilesAndFindProfile(t *testing.T) {
	layout := paths.New(t.TempDir())
	if err := layout.Ensure(); err != nil {
		t.Fatal(err)
	}
	if err := prepareLauncherProfiles(layout); err != nil {
		t.Fatal(err)
	}
	var profiles map[string]any
	if err := util.ReadJSON(filepath.Join(layout.Root, "launcher_profiles.json"), &profiles); err != nil {
		t.Fatal(err)
	}
	if profiles["version"] != float64(3) {
		t.Errorf("launcher_profiles version = %v", profiles["version"])
	}

	id := "1.20.1-forge-47.2.0"
	if err := os.MkdirAll(layout.VersionDir(id), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := util.WriteJSONAtomic(layout.VersionJSON(id), map[string]string{"inheritsFrom": "1.20.1"}); err != nil {
		t.Fatal(err)
	}
	if got := findInstalledProfile(layout, "1.20.1", "47.2.0"); got != id {
		t.Errorf("findInstalledProfile = %q, want %q", got, id)
	}
	if got := findInstalledProfile(layout, "1.20.4", "47.2.0"); got != "" {
		t.Errorf("findInstalledProfile wrong game = %q", got)
	}
}

func TestVersionsRejectsUnknownKind(t *testing.T) {
	if _, err := Versions(context.Background(), nil, Kind("forge"), "1.21.4"); err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Errorf("error = %v", err)
	}
}
func TestLineWriterConcurrent(t *testing.T) {
	var mu sync.Mutex
	var lines []string
	writer := &lineWriter{onLine: func(line string) {
		mu.Lock()
		lines = append(lines, line)
		mu.Unlock()
	}}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_, _ = writer.Write([]byte("line\n"))
			}
		}()
	}
	wg.Wait()
	writer.Flush()
	if len(lines) != 400 {
		t.Fatalf("lines = %d, want 400", len(lines))
	}
}
func TestProfileInheritsFrom(t *testing.T) {
	layout := paths.New(t.TempDir())
	if profileInheritsFrom(layout, "missing", "1.21.4") {
		t.Error("missing profile should not match")
	}
	if err := os.MkdirAll(layout.VersionDir("neoforge-21.4.209"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := util.WriteJSONAtomic(layout.VersionJSON("neoforge-21.4.209"), map[string]string{"inheritsFrom": "1.21.4"}); err != nil {
		t.Fatal(err)
	}
	if !profileInheritsFrom(layout, "neoforge-21.4.209", "1.21.4") {
		t.Error("generated NeoForge profile should match its game version")
	}
	if profileInheritsFrom(layout, "neoforge-21.4.209", "1.20.1") {
		t.Error("profile should not match a different game version")
	}
}
