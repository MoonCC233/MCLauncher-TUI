package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLayoutPaths(t *testing.T) {
	root := t.TempDir()
	l := New(root)

	if l.Root != filepath.Clean(root) {
		t.Errorf("Root = %q, want %q", l.Root, filepath.Clean(root))
	}
	if got, want := l.ConfigFile, filepath.Join(root, "config.json"); got != want {
		t.Errorf("ConfigFile = %q, want %q", got, want)
	}
	if got, want := l.VersionJSON("1.20.1"), filepath.Join(root, "versions", "1.20.1", "1.20.1.json"); got != want {
		t.Errorf("VersionJSON = %q, want %q", got, want)
	}
	if got, want := l.ClientJar("1.20.1"), filepath.Join(root, "versions", "1.20.1", "1.20.1.jar"); got != want {
		t.Errorf("ClientJar = %q, want %q", got, want)
	}
	if got, want := l.Marker("1.20.1"), filepath.Join(root, "versions", "1.20.1", ".mclauncher-installed.json"); got != want {
		t.Errorf("Marker = %q, want %q", got, want)
	}
	if got, want := l.InstanceDir("abc"), filepath.Join(root, "instances", "abc"); got != want {
		t.Errorf("InstanceDir = %q, want %q", got, want)
	}
	if got, want := l.AssetIndex("12"), filepath.Join(root, "assets", "indexes", "12.json"); got != want {
		t.Errorf("AssetIndex = %q, want %q", got, want)
	}
	if got, want := l.AssetObject("abcdef"), filepath.Join(root, "assets", "objects", "ab", "abcdef"); got != want {
		t.Errorf("AssetObject = %q, want %q", got, want)
	}
	if got, want := l.AssetObject("a"), filepath.Join(root, "assets", "objects", "a"); got != want {
		t.Errorf("AssetObject(short) = %q, want %q", got, want)
	}
	if got, want := l.VirtualAssets("legacy"), filepath.Join(root, "assets", "virtual", "legacy"); got != want {
		t.Errorf("VirtualAssets = %q, want %q", got, want)
	}
	if got, want := l.ManifestCache(), filepath.Join(root, "cache", "version_manifest_v2.json"); got != want {
		t.Errorf("ManifestCache = %q, want %q", got, want)
	}
}

func TestEnsureCreatesLayout(t *testing.T) {
	l := New(t.TempDir())
	if err := l.Ensure(); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	for _, dir := range []string{
		l.Root, l.Versions, l.Libraries, l.Assets, l.Indexes, l.Objects,
		l.Virtual, l.LogConfigs, l.Instances, l.Logs, l.Cache, l.Java,
	} {
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			t.Errorf("expected directory %q (err = %v)", dir, err)
		}
	}
}

func TestDefaultRootOverride(t *testing.T) {
	root := t.TempDir()
	t.Setenv("MCLAUNCHER_DATA_DIR", root)
	got, err := DefaultRoot()
	if err != nil {
		t.Fatalf("DefaultRoot: %v", err)
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	if got != abs {
		t.Errorf("DefaultRoot = %q, want %q", got, abs)
	}

	t.Setenv("MCLAUNCHER_DATA_DIR", "")
	got, err = DefaultRoot()
	if err != nil {
		t.Fatalf("DefaultRoot(default): %v", err)
	}
	if filepath.Base(got) != ".minecraft" {
		t.Errorf("DefaultRoot(default) = %q, want a .minecraft directory", got)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("DefaultRoot should be absolute, got %q", got)
	}
}

func TestResolveDataDir(t *testing.T) {
	root := t.TempDir()
	t.Setenv("MCLAUNCHER_DATA_DIR", "")
	got, err := ResolveDataDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if got != root {
		t.Errorf("ResolveDataDir(%q) = %q", root, got)
	}
	got, err = ResolveDataDir("relative-data")
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(got) || filepath.Base(got) != "relative-data" {
		t.Errorf("ResolveDataDir(relative) = %q", got)
	}
}
