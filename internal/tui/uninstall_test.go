package tui

import (
	"os"
	"testing"
)

func TestUninstallVersionRemovesFiles(t *testing.T) {
	m := newTestModel(t)
	const id = "1.21.4"
	dir := m.layout.VersionDir(id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	m.installed[id] = true

	if err := m.uninstallVersion(id); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("version directory still exists: %v", err)
	}
	entries, err := os.ReadDir(m.layout.Versions)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if len(entry.Name()) > len(id) && entry.Name()[:len(id)] == id+".uninstall-" {
			t.Fatalf("leftover staging directory %q", entry.Name())
		}
	}
	if m.installed[id] {
		t.Fatal("version still marked as installed")
	}
	if err := m.uninstallVersion(id); err == nil {
		t.Fatal("uninstalling an absent version should fail")
	}
}

func TestSwitchVersionsRefreshesInstalledState(t *testing.T) {
	m := newTestModel(t)
	const id = "1.21.4"
	dir := m.layout.VersionDir(id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for path, data := range map[string]string{
		m.layout.VersionJSON(id): `{"id":"1.21.4","mainClass":"x","libraries":[{"name":"a:b:1"}]}`,
		m.layout.ClientJar(id):   "jar",
		m.layout.Marker(id):      "{}",
	} {
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m.switchScreen(ScreenVersions)
	if !m.installed[id] {
		t.Fatal("installed version was not detected on screen switch")
	}
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	m.switchScreen(ScreenHome)
	m.switchScreen(ScreenVersions)
	if m.installed[id] {
		t.Fatal("removed version is still marked as installed")
	}
}
