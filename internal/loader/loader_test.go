package loader

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/MoonCC233/MCLauncher-TUI/internal/mirror"
	"github.com/MoonCC233/MCLauncher-TUI/internal/paths"
	"github.com/MoonCC233/MCLauncher-TUI/internal/util"
)

func TestParseOptiFineJarName(t *testing.T) {
	cases := []struct {
		name      string
		minecraft string
		edition   string
		ok        bool
	}{
		{"OptiFine_1.20.1_HD_U_I6.jar", "1.20.1", "HD_U_I6", true},
		{"OptiFine_1.8.9_HD_U_M5.jar", "1.8.9", "HD_U_M5", true},
		{"OptiFine_1.12.2_HD_U_G5.jar", "1.12.2", "HD_U_G5", true},
		{"optifine_1.20.1_HD_U_I6.jar", "1.20.1", "HD_U_I6", true},
		{"OptiFine_1.20.1_HD_U_I6_pre1.jar", "1.20.1", "HD_U_I6_pre1", true},
		{"minecraft.jar", "", "", false},
		{"OptiFine.jar", "", "", false},
		{"OptiFine__HD_U_I6.jar", "", "", false},
	}
	for _, tc := range cases {
		mc, edition, ok := ParseOptiFineJarName(tc.name)
		if ok != tc.ok || mc != tc.minecraft || edition != tc.edition {
			t.Errorf("ParseOptiFineJarName(%q) = %q, %q, %v; want %q, %q, %v", tc.name, mc, edition, ok, tc.minecraft, tc.edition, tc.ok)
		}
	}
}

func TestVersionIDs(t *testing.T) {
	if got := ForgeVersionID("1.20.1", "47.2.0"); got != "1.20.1-forge-47.2.0" {
		t.Errorf("ForgeVersionID = %q", got)
	}
	if got := OptiFineVersionID("1.20.1", "HD_U_I6"); got != "1.20.1-OptiFine-HD_U_I6" {
		t.Errorf("OptiFineVersionID = %q", got)
	}
	in := &Installer{Kind: Fabric, Minecraft: "1.20.1", LoaderVersion: "0.16.9"}
	if got := in.VersionID(); got != "fabric-loader-0.16.9-1.20.1" {
		t.Errorf("Fabric VersionID = %q", got)
	}
}

func TestLoaderMetaRoundTrip(t *testing.T) {
	layout := paths.New(t.TempDir())
	meta := Meta{Kind: Forge, Minecraft: "1.20.1", LoaderVersion: "47.2.0", VersionID: "1.20.1-forge-47.2.0"}
	if err := WriteMeta(layout, meta); err != nil {
		t.Fatal(err)
	}
	got, err := ReadMeta(layout, meta.VersionID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != meta.Kind || got.Minecraft != meta.Minecraft || got.LoaderVersion != meta.LoaderVersion {
		t.Errorf("meta = %+v", got)
	}
	if got.InstalledAt == 0 {
		t.Error("InstalledAt was not filled in")
	}
	if len(Scan(layout)) != 1 {
		t.Errorf("Scan found %d versions, want 1", len(Scan(layout)))
	}
}

func TestReadLaunchWrapper(t *testing.T) {
	dir := t.TempDir()
	jar := filepath.Join(dir, "OptiFine_1.20.1_HD_U_I6.jar")
	f, err := os.Create(jar)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	if w, err := zw.Create("launchwrapper-of.txt"); err == nil {
		_, _ = w.Write([]byte("2.3\n"))
	}
	if w, err := zw.Create("launchwrapper-of-2.3.jar"); err == nil {
		_, _ = w.Write([]byte("payload"))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()

	version, data, err := readLaunchWrapper(jar)
	if err != nil {
		t.Fatal(err)
	}
	if version != "2.3" || string(data) != "payload" {
		t.Errorf("readLaunchWrapper = %q, %q", version, data)
	}
}

var testForgeBases = []string{"https://maven.minecraftforge.net/", "https://files.minecraftforge.net/maven/"}

func TestNeoForgeGroupAndPrefix(t *testing.T) {
	cases := []struct {
		minecraft string
		group     string
		prefix    string
		ok        bool
	}{
		{"1.20.1", "net/neoforged/forge", "1.20.1-", true},
		{"1.20.2", "net/neoforged/neoforge", "20.2.", true},
		{"1.20.4", "net/neoforged/neoforge", "20.4.", true},
		{"1.21", "net/neoforged/neoforge", "21.0.", true},
		{"1.21.1", "net/neoforged/neoforge", "21.1.", true},
		{"1.19.2", "", "", false},
		{"1.20", "", "", false},
	}
	for _, tc := range cases {
		group, prefix, err := neoforgeGroupAndPrefix(tc.minecraft)
		if (err == nil) != tc.ok || group != tc.group || prefix != tc.prefix {
			t.Errorf("neoforgeGroupAndPrefix(%q) = %q, %q, %v; want %q, %q, ok=%v",
				tc.minecraft, group, prefix, err, tc.group, tc.prefix, tc.ok)
		}
	}
}

func TestParseNeoForgeMetadata(t *testing.T) {
	body := []byte(`<metadata><versioning><versions>
  <version>1.20.1-47.1.105</version>
  <version>1.20.1-47.1.106</version>
  <version>1.20.1-47.1.106-installer</version>
  <version>47.1.106</version>
</versions></versioning></metadata>`)
	got, err := parseNeoForgeMetadata(body, "1.20.1-")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Version != "47.1.106" || got[1].Version != "47.1.105" {
		t.Errorf("versions = %#v", got)
	}
	if !got[0].Stable || !got[1].Stable {
		t.Errorf("stable flags = %#v", got)
	}

	modern := []byte(`<metadata><versioning><versions>
  <version>21.1.264</version>
  <version>21.1.265</version>
</versions></versioning></metadata>`)
	got, err = parseNeoForgeMetadata(modern, "21.1.")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Version != "21.1.265" || got[1].Version != "21.1.264" {
		t.Errorf("modern versions = %#v", got)
	}
}

func TestNeoForgeVersionID(t *testing.T) {
	if got, want := NeoForgeVersionID("1.21.1", "21.1.265"), "1.21.1-neoforge-21.1.265"; got != want {
		t.Errorf("NeoForgeVersionID = %q, want %q", got, want)
	}
}

func TestNeoForgeLibraryURLs(t *testing.T) {
	const (
		mirror = "https://mirror.test/bmclapi"
		rel    = "net/neoforged/neoforge/21.1.265/neoforge-21.1.265-universal.jar"
	)
	official := "https://maven.neoforged.net/releases/" + rel
	got := forgeLibraryURLs(
		forgeTestLibrary("net.neoforged:neoforge:21.1.265:universal", official, "", rel),
		mirror,
		[]string{"https://maven.neoforged.net/releases/"},
	)
	if len(got) != 2 || got[0] != mirror+"/"+rel || got[1] != official {
		t.Errorf("NeoForge library URLs = %#v", got)
	}
}

func TestAdoptVersion(t *testing.T) {
	layout := paths.New(t.TempDir())
	oldID, newID := "neoforge-21.1.265", "1.21.1-neoforge-21.1.265"
	oldDir := layout.VersionDir(oldID)
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := map[string]any{"id": oldID, "mainClass": "cpw.mods.bootstraplauncher.BootstrapLauncher"}
	if err := util.WriteJSONAtomic(filepath.Join(oldDir, oldID+".json"), body); err != nil {
		t.Fatal(err)
	}
	if err := adoptVersion(layout, oldID, newID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Errorf("old version directory still exists")
	}
	var got map[string]any
	if err := util.ReadJSON(layout.VersionJSON(newID), &got); err != nil {
		t.Fatal(err)
	}
	if got["id"] != newID {
		t.Errorf("id = %v, want %s", got["id"], newID)
	}
}

func TestMetaMirrors(t *testing.T) {
	t.Setenv("MCLAUNCHER_FABRIC_MIRROR", "https://mirror.test/fabric/v2/")
	if got, want := fabricMetaURL("versions/loader/1.20.1"), "https://mirror.test/fabric/v2/versions/loader/1.20.1"; got != want {
		t.Errorf("fabricMetaURL = %q, want %q", got, want)
	}
	t.Setenv("MCLAUNCHER_QUILT_MIRROR", "")
	if got, want := quiltMetaURL("versions/loader/1.20.1"), mirror.BMCLAPIBase+"/quilt-meta/v3/versions/loader/1.20.1"; got != want {
		t.Errorf("quiltMetaURL = %q, want %q", got, want)
	}
}

func TestParseQuiltLoaderList(t *testing.T) {
	raw := []loaderRootEntry{
		{Version: "0.27.0"},
		{Version: "0.27.0-beta.1"},
		{Version: ""},
	}
	versions := parseQuiltLoaderList(raw)
	if len(versions) != 2 {
		t.Fatalf("versions = %+v", versions)
	}
	if versions[0].Version != "0.27.0" || !versions[0].Stable {
		t.Errorf("first = %+v, want the stable release", versions[0])
	}
	if versions[1].Version != "0.27.0-beta.1" || versions[1].Stable {
		t.Errorf("second = %+v, want a beta", versions[1])
	}
}

func TestParseMetaVersions(t *testing.T) {
	body := []byte(`[
  {"loader":{"version":"0.16.14","stable":true}},
  {"loader":{"version":"0.16.0-beta.1","stable":false}},
  {"loader":{"version":""}}
]`)
	var raw []loaderVersionEntry
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatal(err)
	}
	got := parseMetaVersions(raw)
	if len(got) != 2 {
		t.Fatalf("versions = %#v, want 2", got)
	}
	if got[0].Version != "0.16.14" || !got[0].Stable {
		t.Errorf("first = %#v", got[0])
	}
	if got[1].Version != "0.16.0-beta.1" || got[1].Stable {
		t.Errorf("second = %#v", got[1])
	}

	// Quilt has no stable flag; beta versions are unstable.
	quilt := []byte(`[{"loader":{"version":"0.27.0"}},{"loader":{"version":"0.20.0-beta.9"}}]`)
	if err := json.Unmarshal(quilt, &raw); err != nil {
		t.Fatal(err)
	}
	got = parseMetaVersions(raw)
	if len(got) != 2 || !got[0].Stable || got[1].Stable {
		t.Errorf("quilt versions = %#v", got)
	}
}

func forgeTestLibrary(name, artifactURL, libraryURL, path string) forgeLibrary {
	lib := forgeLibrary{Name: name, URL: libraryURL}
	lib.Downloads.Artifact = forgeArtifact{Path: path, URL: artifactURL}
	return lib
}

func TestForgeLibraryURLs(t *testing.T) {
	const (
		mirror = "https://mirror.test/maven"
		rel    = "net/minecraftforge/forge/1.20.1-47.2.0/forge-1.20.1-47.2.0-universal.jar"
	)
	official := "https://maven.minecraftforge.net/" + rel
	got := forgeLibraryURLs(forgeTestLibrary("net.minecraftforge:forge:1.20.1-47.2.0:universal", official, "", rel), mirror, testForgeBases)
	if len(got) != 2 || got[0] != mirror+"/"+rel || got[1] != official {
		t.Errorf("mirrored URLs = %#v", got)
	}

	// Libraries hosted by Mojang must keep their original URL.
	mojangURL := "https://libraries.minecraft.net/com/google/guava/guava/20.0/guava-20.0.jar"
	got = forgeLibraryURLs(forgeTestLibrary("com.google.guava:guava:20.0", mojangURL, "", "com/google/guava/guava/20.0/guava-20.0.jar"), mirror, testForgeBases)
	if len(got) != 1 || got[0] != mojangURL {
		t.Errorf("Mojang library URLs = %#v", got)
	}

	// Old install profiles only have a repository base URL.
	got = forgeLibraryURLs(forgeTestLibrary("com.example:demo:1.0", "", "https://maven.minecraftforge.net/", "com/example/demo/1.0/demo-1.0.jar"), mirror, testForgeBases)
	if len(got) != 2 || got[0] != mirror+"/com/example/demo/1.0/demo-1.0.jar" {
		t.Errorf("legacy profile URLs = %#v", got)
	}

	// Without a mirror, the official Maven URL is the only candidate.
	got = forgeLibraryURLs(forgeTestLibrary("com.example:demo:1.0", "", "", "com/example/demo/1.0/demo-1.0.jar"), "", testForgeBases)
	if len(got) != 1 || got[0] != "https://maven.minecraftforge.net/com/example/demo/1.0/demo-1.0.jar" {
		t.Errorf("official URLs = %#v", got)
	}
}

func TestForgeProfileLibraries(t *testing.T) {
	body := []byte(`{
  "version": "1.20.1-forge-47.2.0",
  "minecraft": "1.20.1",
  "libraries": [{"name": "a:b:1.0"}],
  "versionInfo": {"libraries": [{"name": "c:d:2.0"}]}
}`)
	var profile forgeInstallProfile
	if err := json.Unmarshal(body, &profile); err != nil {
		t.Fatal(err)
	}
	libs := profile.allLibraries()
	if len(libs) != 2 || libs[0].Name != "a:b:1.0" || libs[1].Name != "c:d:2.0" {
		t.Errorf("libraries = %#v", libs)
	}
}

func TestParseForgeMetadata(t *testing.T) {
	body := []byte(`<metadata>
  <groupId>net.minecraftforge</groupId>
  <versioning>
    <versions>
      <version>1.19.2-43.2.0</version>
      <version>1.20.1-47.1.0</version>
      <version>1.20.1-47.2.0</version>
      <version>1.20.10-1.0.0</version>
    </versions>
  </versioning>
</metadata>`)
	got, err := parseForgeMetadata(body, "1.20.1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("versions = %d, want 2", len(got))
	}
	if got[0].Version != "47.2.0" || got[1].Version != "47.1.0" {
		t.Errorf("order = %q, %q", got[0].Version, got[1].Version)
	}
}
