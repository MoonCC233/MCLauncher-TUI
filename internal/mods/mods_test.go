package mods

import (
	"archive/zip"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func writeJar(t *testing.T, path string, files map[string]string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
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

func TestScanReadsAllLoaderFormats(t *testing.T) {
	dir := t.TempDir()
	writeJar(t, filepath.Join(dir, "fabric.jar"), map[string]string{
		"fabric.mod.json": `{"id":"lithium","name":"Lithium","version":"0.11.2","description":"Optimises the game","authors":["Jelly"]}`,
	})
	writeJar(t, filepath.Join(dir, "quilt.jar"), map[string]string{
		"quilt.mod.json": `{"quilt_loader":{"id":"qsl","version":"1.0.0","metadata":{"name":"Quilted","description":"Q","contributors":{"Owner":["A"]}}}}`,
	})
	writeJar(t, filepath.Join(dir, "forge.jar"), map[string]string{
		"META-INF/mods.toml": "modLoader=\"javafml\"\n[[mods]]\nmodId=\"jei\"\ndisplayName=\"JEI\"\nversion=\"15.2\"\ndescription='''Multi\nline'''\nauthors=\"me\"\n",
	})
	writeJar(t, filepath.Join(dir, "neoforge.jar"), map[string]string{
		"META-INF/neoforge.mods.toml": "modLoader=\"javafml\"\n[[mods]]\nmodId=\"neomap\"\ndisplayName=\"NeoMap\"\nversion=\"1.0\"\n",
	})
	writeJar(t, filepath.Join(dir, "legacy.jar"), map[string]string{
		"mcmod.info": `[{"modid":"old","name":"Old Mod","version":"1.2","description":"Legacy","authorList":["Ann"]}]`,
	})
	writeJar(t, filepath.Join(dir, "plain.jar"), nil)
	writeJar(t, filepath.Join(dir, "off.jar.disabled"), map[string]string{
		"fabric.mod.json": `{"id":"off","name":"Off","version":"2"}`,
	})
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	list, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 7 {
		t.Fatalf("scan returned %d mods, want 7", len(list))
	}
	byName := map[string]Mod{}
	for _, mod := range list {
		byName[mod.FileName] = mod
	}
	if got := byName["fabric.jar"]; got.Name != "Lithium" || got.Version != "0.11.2" || !got.Enabled || len(got.Loaders) != 1 || got.Loaders[0] != "fabric" {
		t.Errorf("fabric mod = %+v", got)
	}
	if got := byName["quilt.jar"]; got.Name != "Quilted" || got.Loaders[0] != "quilt" || got.Authors != "A" {
		t.Errorf("quilt mod = %+v", got)
	}
	if got := byName["forge.jar"]; got.Name != "JEI" || got.Description != "Multi\nline" || got.Loaders[0] != "forge" {
		t.Errorf("forge mod = %+v", got)
	}
	if got := byName["neoforge.jar"]; got.Name != "NeoMap" || got.Version != "1.0" || got.Loaders[0] != "neoforge" {
		t.Errorf("neoforge mod = %+v", got)
	}
	if got := byName["legacy.jar"]; got.Name != "Old Mod" || got.Authors != "Ann" || got.Loaders[0] != "forge" {
		t.Errorf("legacy mod = %+v", got)
	}
	if got := byName["off.jar.disabled"]; got.Enabled {
		t.Errorf("disabled mod marked enabled: %+v", got)
	}
}

func TestAddToggleRemove(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(t.TempDir(), "cool.jar")
	writeJar(t, src, map[string]string{
		"fabric.mod.json": `{"id":"cool","name":"Cool","version":"1"}`,
	})

	mod, err := Add(dir, src)
	if err != nil {
		t.Fatal(err)
	}
	if mod.FileName != "cool.jar" || mod.Name != "Cool" {
		t.Errorf("added mod = %+v", mod)
	}
	if _, err := Add(dir, src); !errors.Is(err, ErrExists) {
		t.Errorf("duplicate Add = %v, want ErrExists", err)
	}

	name, err := SetEnabled(dir, "cool.jar", false)
	if err != nil {
		t.Fatal(err)
	}
	if name != "cool.jar.disabled" {
		t.Errorf("disabled name = %q", name)
	}
	if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
		t.Fatal(err)
	}
	name, err = SetEnabled(dir, name, true)
	if err != nil {
		t.Fatal(err)
	}
	if name != "cool.jar" {
		t.Errorf("enabled name = %q", name)
	}

	if err := Remove(dir, "cool.jar"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "cool.jar")); !os.IsNotExist(err) {
		t.Error("mod file still exists")
	}
}

func TestRejectsUnsafeNames(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(filepath.Dir(dir), "evil.jar")
	writeJar(t, outside, nil)
	for _, name := range []string{"../evil.jar", `..\evil.jar`, "sub/evil.jar"} {
		if _, err := SetEnabled(dir, name, false); err == nil {
			t.Errorf("SetEnabled(%q) should fail", name)
		}
		if err := Remove(dir, name); err == nil {
			t.Errorf("Remove(%q) should fail", name)
		}
	}
}
