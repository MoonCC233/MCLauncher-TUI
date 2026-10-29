package packs

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizeFilename(t *testing.T) {
	cases := map[string]string{
		"pack.zip":          "pack.zip",
		"sub/dir/pack.zip":  "pack.zip",
		"weird:name?.zip":   "weird-name-.zip",
		"no-extension":      "no-extension",
		"":                  "pack",
		"..":                "pack",
		"  Fresh Textures ": "Fresh Textures",
	}
	for in, want := range cases {
		if got := SanitizeFilename(in); got != want {
			t.Errorf("SanitizeFilename(%q) = %q, want %q", in, got, want)
		}
	}
}

// allowTestHosts lets the installer talk to an httptest server.
func allowTestHosts(t *testing.T) {
	t.Helper()
	previous := validatePackURL
	validatePackURL = func(string) error { return nil }
	t.Cleanup(func() { validatePackURL = previous })
}

func TestValidateURL(t *testing.T) {
	if err := ValidateURL("https://cdn.modrinth.com/data/x/y/pack.zip"); err != nil {
		t.Errorf("cdn url rejected: %v", err)
	}
	for _, bad := range []string{
		"http://cdn.modrinth.com/data/x/y/pack.zip",
		"https://example.com/pack.zip",
		"https://cdn.modrinth.com.evil.test/pack.zip",
		"not a url",
		"",
	} {
		if err := ValidateURL(bad); err == nil {
			t.Errorf("ValidateURL(%q) should fail", bad)
		}
	}
}

func TestInstallVerifiesChecksumAndTracksManifest(t *testing.T) {
	allowTestHosts(t)
	payload := []byte("PK\x03\x04 fake pack")
	sum := sha512.Sum512(payload)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	dir := t.TempDir()
	record := Record{
		ProjectID: "proj", VersionID: "ver", Title: "Fresh",
		Filename: "Fresh.zip", SHA512: hex.EncodeToString(sum[:]),
	}
	var progress int64
	installed, err := Install(context.Background(), server.Client(), dir, ResourcePack, record, server.URL, func(done int64) {
		progress = done
	})
	if err != nil {
		t.Fatal(err)
	}
	if progress != int64(len(payload)) {
		t.Errorf("progress = %d, want %d", progress, len(payload))
	}
	if installed.Size != int64(len(payload)) || installed.InstalledAt == 0 {
		t.Errorf("record = %+v", installed)
	}

	data, err := os.ReadFile(filepath.Join(Dir(dir), "Fresh.zip"))
	if err != nil || string(data) != string(payload) {
		t.Fatalf("pack file = %v, err = %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(Dir(dir), "Fresh.zip.part")); !os.IsNotExist(err) {
		t.Error("temporary download file was left behind")
	}

	list := List(dir, ResourcePack)
	if len(list) != 1 || list[0].Title != "Fresh" || list[0].ProjectID != "proj" {
		t.Fatalf("manifest = %+v", list)
	}
}

func TestInstallRejectsBadChecksum(t *testing.T) {
	allowTestHosts(t)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("payload"))
	}))
	defer server.Close()

	dir := t.TempDir()
	record := Record{Filename: "bad.zip", SHA512: strings.Repeat("0", 128)}
	if _, err := Install(context.Background(), server.Client(), dir, ResourcePack, record, server.URL, nil); err == nil {
		t.Fatal("expected a checksum error")
	}
	if _, err := os.Stat(filepath.Join(Dir(dir), "bad.zip")); !os.IsNotExist(err) {
		t.Error("a pack with a bad checksum must not be kept")
	}
	if len(List(dir, ResourcePack)) != 0 {
		t.Error("manifest should stay empty")
	}
}

func TestSetEnabledKeepsOtherOptions(t *testing.T) {
	dir := t.TempDir()
	options := strings.Join([]string{
		"version:3465",
		"lang:zh_CN",
		`resourcePacks:["vanilla"]`,
		"fov:0.0",
	}, "\n") + "\n"
	if err := os.WriteFile(OptionsPath(dir), []byte(options), 0o644); err != nil {
		t.Fatal(err)
	}

	changed, err := SetEnabled(dir, ResourcePack, "Fresh.zip", true)
	if err != nil || !changed {
		t.Fatalf("changed = %v, err = %v", changed, err)
	}
	if !Enabled(dir, ResourcePack, "Fresh.zip") {
		t.Fatal("pack should be enabled")
	}
	data, err := os.ReadFile(OptionsPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{"version:3465", "lang:zh_CN", "fov:0.0", `"vanilla"`, `"file/Fresh.zip"`} {
		if !strings.Contains(text, want) {
			t.Errorf("options.txt lost %q:\n%s", want, text)
		}
	}

	// Enabling twice is a no-op.
	if changed, err := SetEnabled(dir, ResourcePack, "Fresh.zip", true); err != nil || changed {
		t.Errorf("second enable changed = %v, err = %v", changed, err)
	}

	if changed, err := SetEnabled(dir, ResourcePack, "Fresh.zip", false); err != nil || !changed {
		t.Fatalf("disable changed = %v, err = %v", changed, err)
	}
	if Enabled(dir, ResourcePack, "Fresh.zip") {
		t.Fatal("pack should be disabled")
	}
	data, _ = os.ReadFile(OptionsPath(dir))
	if strings.Contains(string(data), "file/Fresh.zip") {
		t.Errorf("options.txt still lists the pack:\n%s", data)
	}
}

func TestSetEnabledCreatesOptionsWhenMissing(t *testing.T) {
	dir := t.TempDir()
	if changed, err := SetEnabled(dir, ResourcePack, "Fresh.zip", true); err != nil || !changed {
		t.Fatalf("changed = %v, err = %v", changed, err)
	}
	data, err := os.ReadFile(OptionsPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `resourcePacks:["file/Fresh.zip"]`) {
		t.Errorf("options.txt = %s", data)
	}
	if !Enabled(dir, ResourcePack, "Fresh.zip") {
		t.Fatal("pack should be enabled")
	}
}

func TestRemoveDropsFileAndEntry(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(Dir(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(Dir(dir), "Fresh.zip"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := upsertRecord(dir, ResourcePack, Record{Filename: "Fresh.zip", Title: "Fresh"}); err != nil {
		t.Fatal(err)
	}
	if _, err := SetEnabled(dir, ResourcePack, "Fresh.zip", true); err != nil {
		t.Fatal(err)
	}

	if err := Remove(dir, ResourcePack, "Fresh.zip"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(Dir(dir), "Fresh.zip")); !os.IsNotExist(err) {
		t.Error("pack file still exists")
	}
	if len(List(dir, ResourcePack)) != 0 {
		t.Error("manifest entry still exists")
	}
	if Enabled(dir, ResourcePack, "Fresh.zip") {
		t.Error("options.txt still enables the pack")
	}
}

func TestInstallURLAllowsOtherHosts(t *testing.T) {
	payload := []byte("PK\x03\x04 other site")
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	dir := t.TempDir()
	// No validatePackURL override: the catalog host check must not apply to links
	// the player pasted, only https matters.
	record, err := InstallURL(context.Background(), server.Client(), dir, Mod, Record{
		Filename: "Cool Mod.jar", Title: "Cool Mod",
	}, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if record.Filename != "Cool Mod.jar" {
		t.Errorf("filename = %q", record.Filename)
	}
	if _, err := os.Stat(filepath.Join(DirFor(dir, Mod), "Cool Mod.jar")); err != nil {
		t.Fatalf("mod file missing: %v", err)
	}
	if len(List(dir, Mod)) != 1 {
		t.Error("manifest should list the mod")
	}
}

func TestImportFileCopiesIntoTheRightFolder(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(t.TempDir(), "Fancy Shader.zip")
	if err := os.WriteFile(source, []byte("PK\x03\x04 shader"), 0o644); err != nil {
		t.Fatal(err)
	}

	record, err := ImportFile(dir, ShaderPack, source)
	if err != nil {
		t.Fatal(err)
	}
	if record.Filename != "Fancy Shader.zip" || record.Source != source {
		t.Errorf("record = %+v", record)
	}
	if _, err := os.Stat(filepath.Join(DirFor(dir, ShaderPack), "Fancy Shader.zip")); err != nil {
		t.Fatalf("shader not copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(DirFor(dir, ResourcePack), "Fancy Shader.zip")); !os.IsNotExist(err) {
		t.Error("the file must not land in resourcepacks")
	}
	if len(List(dir, ShaderPack)) != 1 {
		t.Error("manifest should list the shader")
	}

	// Wrong extension for the kind is rejected.
	jar := filepath.Join(t.TempDir(), "mod.jar")
	if err := os.WriteFile(jar, []byte("jar"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ImportFile(dir, ShaderPack, jar); err == nil {
		t.Error("a .jar must not be imported as a shader")
	}
}

func TestModEnableDisableUsesDisabledSuffix(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(DirFor(dir, Mod), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(DirFor(dir, Mod), "Cool.jar"), []byte("jar"), 0o644); err != nil {
		t.Fatal(err)
	}

	if !Enabled(dir, Mod, "Cool.jar") {
		t.Fatal("a jar without the suffix is enabled")
	}
	changed, err := SetEnabled(dir, Mod, "Cool.jar", false)
	if err != nil || !changed {
		t.Fatalf("disable changed = %v err = %v", changed, err)
	}
	if _, err := os.Stat(filepath.Join(DirFor(dir, Mod), "Cool.jar.disabled")); err != nil {
		t.Fatalf("mod was not renamed: %v", err)
	}
	if Enabled(dir, Mod, "Cool.jar.disabled") {
		t.Error("the disabled file should report disabled")
	}
	if _, err := SetEnabled(dir, Mod, "Cool.jar", true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(DirFor(dir, Mod), "Cool.jar")); err != nil {
		t.Fatalf("mod was not restored: %v", err)
	}
}

func TestShaderToggleIsANoOp(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(DirFor(dir, ShaderPack), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(DirFor(dir, ShaderPack), "S.zip"), []byte("zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := SetEnabled(dir, ShaderPack, "S.zip", false)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Error("shaders are selected in game, nothing to write")
	}
	if !Enabled(dir, ShaderPack, "S.zip") {
		t.Error("an installed shader counts as available")
	}
}

func TestListIgnoresFilesOfTheWrongKind(t *testing.T) {
	dir := t.TempDir()
	for _, kind := range Kinds {
		if err := os.MkdirAll(DirFor(dir, kind), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// A jar in the resource pack folder and a zip in the mods folder are not
	// something Minecraft loads, so they must not be listed.
	if err := os.WriteFile(filepath.Join(DirFor(dir, ResourcePack), "wrong.jar"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(DirFor(dir, ResourcePack), "right.zip"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(DirFor(dir, Mod), "wrong.zip"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(DirFor(dir, Mod), "right.jar"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(DirFor(dir, Mod), "off.jar.disabled"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(DirFor(dir, ShaderPack), "right.zip"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	packs := List(dir, ResourcePack)
	if len(packs) != 1 || packs[0].Filename != "right.zip" {
		t.Errorf("resource packs = %+v, want only right.zip", packs)
	}
	mods := List(dir, Mod)
	if len(mods) != 2 {
		t.Fatalf("mods = %+v, want the jar and the disabled jar", mods)
	}
	shaders := List(dir, ShaderPack)
	if len(shaders) != 1 || shaders[0].Filename != "right.zip" {
		t.Errorf("shaders = %+v, want only right.zip", shaders)
	}
}
