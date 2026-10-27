package mojang

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"github.com/MoonCC233/MCLauncher-TUI/internal/paths"
)

func TestSafeZipName(t *testing.T) {
	valid := []string{
		"libtest.so",
		"a/b/c.dll",
		"a/./b.txt",
	}
	for _, name := range valid {
		if !safeZipName(name) {
			t.Errorf("safeZipName(%q) = false, want true", name)
		}
	}
	invalid := []string{
		"",
		".",
		"../evil.txt",
		"a/../../evil.txt",
		"/absolute.txt",
		"C:/drive.txt",
	}
	for _, name := range invalid {
		if safeZipName(name) {
			t.Errorf("safeZipName(%q) = true, want false", name)
		}
	}
}

func TestExcluded(t *testing.T) {
	if !excluded("META-INF/MANIFEST.MF", nil) {
		t.Error("META-INF should always be excluded")
	}
	if !excluded("meta-inf/MANIFEST.MF", nil) {
		t.Error("META-INF matching should be case-insensitive")
	}
	if !excluded("excluded/secret.txt", []string{"excluded/"}) {
		t.Error("exclude patterns should match directories")
	}
	if !excluded("excluded/secret.txt", []string{`excluded\`}) {
		t.Error("backslash patterns should match slash entries")
	}
	if !excluded("excluded", []string{"excluded"}) {
		t.Error("exact matches should be excluded")
	}
	if excluded("not-excluded/file.txt", []string{"excluded"}) {
		t.Error("prefix matching should require a path boundary")
	}
	if excluded("libtest.so", []string{"", "/"}) {
		t.Error("empty patterns should be ignored")
	}
}

func writeNativeZip(t *testing.T, jarPath string, entries map[string]string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(jarPath), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(jarPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for name, body := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestExtractNatives(t *testing.T) {
	layout := paths.New(t.TempDir())
	res := &LibraryResolution{Natives: []NativeJar{{
		Path:    "org/example/natives/test-natives.jar",
		Exclude: []string{"excluded"},
	}}}
	jarPath := NativePath(layout, res.Natives[0])
	writeNativeZip(t, jarPath, map[string]string{
		"libtest.so":           "native-binary",
		"sub/dir/readme.txt":   "hello",
		"META-INF/MANIFEST.MF": "metadata",
		"excluded/secret.txt":  "secret",
		"../evil.txt":          "evil",
		"/absolute.txt":        "evil",
		"C:/drive.txt":         "evil",
	})

	nativesDir := layout.NativesDir("1.20.1")
	if err := os.MkdirAll(nativesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(nativesDir, "stale.txt")
	if err := os.WriteFile(stale, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := ExtractNatives(layout, "1.20.1", res); err != nil {
		t.Fatalf("ExtractNatives: %v", err)
	}

	assertFile := func(rel, want string) {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(nativesDir, filepath.FromSlash(rel)))
		if err != nil {
			t.Errorf("missing %s: %v", rel, err)
			return
		}
		if string(data) != want {
			t.Errorf("%s = %q, want %q", rel, data, want)
		}
	}
	assertMissing := func(path string) {
		t.Helper()
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("%s should not exist", path)
		}
	}

	assertFile("libtest.so", "native-binary")
	assertFile("sub/dir/readme.txt", "hello")
	assertMissing(stale)
	assertMissing(filepath.Join(nativesDir, "META-INF", "MANIFEST.MF"))
	assertMissing(filepath.Join(nativesDir, "excluded", "secret.txt"))
	assertMissing(filepath.Join(layout.Versions, "1.20.1", "evil.txt"))
	assertMissing(filepath.Join(nativesDir, "C:", "drive.txt"))

	// A missing jar surfaces a useful error.
	missing := &LibraryResolution{Natives: []NativeJar{{Path: "missing.jar"}}}
	if err := ExtractNatives(layout, "1.19.4", missing); err == nil {
		t.Error("ExtractNatives(missing jar) = nil error")
	}
}

func TestLegacyAssetPath(t *testing.T) {
	layout := paths.New(t.TempDir())
	if got := LegacyAssetPath(layout, "legacy"); got != layout.Assets {
		t.Errorf("LegacyAssetPath(no virtual dir) = %q, want %q", got, layout.Assets)
	}
	virtual := layout.VirtualAssets("legacy")
	if err := os.MkdirAll(virtual, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := LegacyAssetPath(layout, "legacy"); got != virtual {
		t.Errorf("LegacyAssetPath(virtual dir) = %q, want %q", got, virtual)
	}
	if err := os.WriteFile(filepath.Join(virtual, ".mclauncher-ok"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := LegacyAssetPath(layout, "legacy"); got != virtual {
		t.Errorf("LegacyAssetPath(marker) = %q, want %q", got, virtual)
	}
}
