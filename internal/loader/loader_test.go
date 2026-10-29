package loader

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

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

func TestRewriteMavenURL(t *testing.T) {
	const mirror = "https://mirror.test/maven"
	if got, want := rewriteMavenURL("https://maven.fabricmc.net/a/b.jar", "https://maven.fabricmc.net/", mirror), mirror+"/a/b.jar"; got != want {
		t.Errorf("rewriteMavenURL = %q, want %q", got, want)
	}
	unchanged := "https://libraries.minecraft.net/a/b.jar"
	if got := rewriteMavenURL(unchanged, "https://maven.fabricmc.net/", mirror); got != unchanged {
		t.Errorf("rewriteMavenURL changed an unrelated host: %q", got)
	}
	if got := rewriteMavenURL(unchanged, "https://maven.fabricmc.net/", ""); got != unchanged {
		t.Errorf("rewriteMavenURL changed a URL without a mirror: %q", got)
	}
}

func TestRewriteForgeVersionMirror(t *testing.T) {
	layout := paths.New(t.TempDir())
	if err := os.MkdirAll(layout.Versions, 0o755); err != nil {
		t.Fatal(err)
	}
	body := map[string]any{
		"id": "1.20.1-forge-47.2.0",
		"libraries": []any{
			map[string]any{
				"name": "net.minecraftforge:demo:1.0",
				"url":  "https://maven.minecraftforge.net/",
				"downloads": map[string]any{
					"artifact": map[string]any{
						"url": "https://maven.minecraftforge.net/net/minecraftforge/demo/1.0/demo-1.0.jar",
					},
					"classifiers": map[string]any{
						"natives": map[string]any{
							"url": "https://maven.minecraftforge.net/net/minecraftforge/demo/1.0/demo-1.0-natives.jar",
						},
					},
				},
			},
		},
	}
	path := layout.VersionJSON("1.20.1-forge-47.2.0")
	if err := util.WriteJSONAtomic(path, body); err != nil {
		t.Fatal(err)
	}
	if err := rewriteForgeVersionMirror(layout, "1.20.1-forge-47.2.0", "https://mirror.test/maven"); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Libraries []struct {
			URL       string `json:"url"`
			Downloads struct {
				Artifact struct {
					URL string `json:"url"`
				} `json:"artifact"`
				Classifiers map[string]struct {
					URL string `json:"url"`
				} `json:"classifiers"`
			} `json:"downloads"`
		} `json:"libraries"`
	}
	if err := util.ReadJSON(path, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Libraries) != 1 {
		t.Fatalf("libraries = %#v", got.Libraries)
	}
	lib := got.Libraries[0]
	if lib.URL != "https://mirror.test/maven/" {
		t.Errorf("url = %q", lib.URL)
	}
	if lib.Downloads.Artifact.URL != "https://mirror.test/maven/net/minecraftforge/demo/1.0/demo-1.0.jar" {
		t.Errorf("artifact url = %q", lib.Downloads.Artifact.URL)
	}
	if got := lib.Downloads.Classifiers["natives"].URL; got != "https://mirror.test/maven/net/minecraftforge/demo/1.0/demo-1.0-natives.jar" {
		t.Errorf("classifier url = %q", got)
	}
}

func TestMetaMirrors(t *testing.T) {
	t.Setenv("MCLAUNCHER_FABRIC_MIRROR", "https://mirror.test/fabric/v2/")
	if got, want := fabricMetaURL("versions/loader/1.20.1"), "https://mirror.test/fabric/v2/versions/loader/1.20.1"; got != want {
		t.Errorf("fabricMetaURL = %q, want %q", got, want)
	}
	t.Setenv("MCLAUNCHER_QUILT_MIRROR", "")
	if got, want := quiltMetaURL("versions/loader/1.20.1"), quiltMetaBase+"/versions/loader/1.20.1"; got != want {
		t.Errorf("quiltMetaURL = %q, want %q", got, want)
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
	got := forgeLibraryURLs(forgeTestLibrary("net.minecraftforge:forge:1.20.1-47.2.0:universal", official, "", rel), mirror)
	if len(got) != 2 || got[0] != mirror+"/"+rel || got[1] != official {
		t.Errorf("mirrored URLs = %#v", got)
	}

	// Libraries hosted by Mojang must keep their original URL.
	mojangURL := "https://libraries.minecraft.net/com/google/guava/guava/20.0/guava-20.0.jar"
	got = forgeLibraryURLs(forgeTestLibrary("com.google.guava:guava:20.0", mojangURL, "", "com/google/guava/guava/20.0/guava-20.0.jar"), mirror)
	if len(got) != 1 || got[0] != mojangURL {
		t.Errorf("Mojang library URLs = %#v", got)
	}

	// Old install profiles only have a repository base URL.
	got = forgeLibraryURLs(forgeTestLibrary("com.example:demo:1.0", "", "https://maven.minecraftforge.net/", "com/example/demo/1.0/demo-1.0.jar"), mirror)
	if len(got) != 2 || got[0] != mirror+"/com/example/demo/1.0/demo-1.0.jar" {
		t.Errorf("legacy profile URLs = %#v", got)
	}

	// Without a mirror, the official Maven URL is the only candidate.
	got = forgeLibraryURLs(forgeTestLibrary("com.example:demo:1.0", "", "", "com/example/demo/1.0/demo-1.0.jar"), "")
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
