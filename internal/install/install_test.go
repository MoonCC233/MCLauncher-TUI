package install

import (
	"context"
	"crypto/sha1"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MoonCC233/MCLauncher-TUI/internal/mojang"
	"github.com/MoonCC233/MCLauncher-TUI/internal/paths"
)

func touch(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestIsInstalled(t *testing.T) {
	layout := paths.New(t.TempDir())
	const id = "1.21.4"

	if IsInstalled(layout, "") {
		t.Error("empty version ID should not be installed")
	}
	if IsInstalled(layout, id) {
		t.Error("a missing version should not be installed")
	}

	touch(t, layout.Marker(id), 1)
	if IsInstalled(layout, id) {
		t.Error("marker alone should not count as installed")
	}

	touch(t, layout.ClientJar(id), 0)
	if IsInstalled(layout, id) {
		t.Error("an empty client jar should not count as installed")
	}
	touch(t, layout.ClientJar(id), 10)
	if IsInstalled(layout, id) {
		t.Error("missing version JSON should not count as installed")
	}

	touch(t, layout.VersionJSON(id), 10)
	if !IsInstalled(layout, id) {
		t.Error("marker + client jar + version JSON should count as installed")
	}
}

func TestInstalledSet(t *testing.T) {
	layout := paths.New(t.TempDir())
	if err := layout.Ensure(); err != nil {
		t.Fatal(err)
	}
	complete := func(id string) {
		touch(t, layout.Marker(id), 1)
		touch(t, layout.ClientJar(id), 10)
		touch(t, layout.VersionJSON(id), 10)
	}
	complete("1.21.4")
	complete("1.20.1")
	// Incomplete variants must be ignored.
	touch(t, layout.Marker("empty-jar"), 1)
	touch(t, layout.ClientJar("empty-jar"), 0)
	touch(t, layout.VersionJSON("empty-jar"), 10)
	touch(t, layout.Marker("no-json"), 1)
	touch(t, layout.ClientJar("no-json"), 10)
	if err := os.MkdirAll(filepath.Join(layout.Versions, "not-a-dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(layout.Versions, "plain-file"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := InstalledSet(layout)
	if len(got) != 2 || !got["1.21.4"] || !got["1.20.1"] {
		t.Errorf("InstalledSet = %#v", got)
	}

	// A missing versions directory yields an empty map, not a panic.
	missing := paths.New(filepath.Join(t.TempDir(), "does-not-exist"))
	if got := InstalledSet(missing); len(got) != 0 {
		t.Errorf("InstalledSet(missing) = %#v", got)
	}
}

func TestLogConfigPath(t *testing.T) {
	layout := paths.New(t.TempDir())

	v := &mojang.VersionJSON{ID: "1.20.1"}
	if got, want := logConfigPath(layout, v), filepath.Join(layout.LogConfigs, "1.20.1-log4j.xml"); got != want {
		t.Errorf("logConfigPath = %q, want %q", got, want)
	}

	v.Logging = &mojang.Logging{Client: &mojang.LoggingClient{
		File: mojang.DownloadInfo{Path: "log4j/client-1.20.1.xml"},
	}}
	if got, want := logConfigPath(layout, v), filepath.Join(layout.LogConfigs, "client-1.20.1.xml"); got != want {
		t.Errorf("logConfigPath(path) = %q, want %q", got, want)
	}

	v.Logging.Client.File = mojang.DownloadInfo{URL: "https://example.test/log4j/from-url.xml?token=abc"}
	if got, want := LogConfigPath(layout, v), filepath.Join(layout.LogConfigs, "from-url.xml"); got != want {
		t.Errorf("LogConfigPath(url) = %q, want %q", got, want)
	}

	empty := &mojang.VersionJSON{}
	if got, want := logConfigPath(layout, empty), filepath.Join(layout.LogConfigs, "client-log4j.xml"); got != want {
		t.Errorf("logConfigPath(empty) = %q, want %q", got, want)
	}
	if !strings.HasSuffix(logConfigPath(layout, empty), ".xml") {
		t.Error("log config should always be an XML file")
	}
}

func TestConsoleLogConfigReplacesXMLLayout(t *testing.T) {
	layout := paths.New(t.TempDir())
	if err := layout.Ensure(); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(layout.LogConfigs, "client-1.12.xml")
	raw := `<?xml version="1.0" encoding="UTF-8"?>
<Configuration>
  <Appenders>
    <Console name="SysOut" target="SYSTEM_OUT">
      <LegacyXMLLayout />
    </Console>
  </Appenders>
</Configuration>`
	if err := os.WriteFile(source, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	dest, err := ConsoleLogConfig(layout, source)
	if err != nil {
		t.Fatal(err)
	}
	if dest == source {
		t.Fatal("XML console config should be rewritten to a separate file")
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "LegacyXMLLayout") {
		t.Error("rewritten config still uses LegacyXMLLayout")
	}
	if !strings.Contains(string(data), "PatternLayout") {
		t.Error("rewritten console appender should use PatternLayout")
	}
	if sourceData, err := os.ReadFile(source); err != nil || string(sourceData) != raw {
		t.Error("source log config should not be modified")
	}
	again, err := ConsoleLogConfig(layout, source)
	if err != nil || again != dest {
		t.Fatalf("second rewrite = %q, %v; want %q", again, err, dest)
	}
}

func TestConsoleLogConfigKeepsPatternLayout(t *testing.T) {
	layout := paths.New(t.TempDir())
	if err := layout.Ensure(); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(layout.LogConfigs, "client.xml")
	raw := `<Configuration><Appenders><Console><PatternLayout pattern="%msg%n" /></Console></Appenders></Configuration>`
	if err := os.WriteFile(source, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	dest, err := ConsoleLogConfig(layout, source)
	if err != nil {
		t.Fatal(err)
	}
	if dest != source {
		t.Fatalf("PatternLayout config = %q, want original %q", dest, source)
	}
}

func TestThreads(t *testing.T) {
	cases := []struct {
		in   int
		want int
	}{
		{0, 8},
		{-5, 8},
		{4, 4},
		{100, 32},
	}
	for _, tc := range cases {
		in := &Installer{Threads: tc.in}
		if got := in.threads(); got != tc.want {
			t.Errorf("threads(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}
func TestIsInstalledInheritedLoader(t *testing.T) {
	layout := paths.New(t.TempDir())
	const parent = "1.21.4"
	const loader = "fabric-loader-0.16.10-1.21.4"
	touch(t, layout.ClientJar(parent), 10)
	if err := os.MkdirAll(filepath.Dir(layout.VersionJSON(parent)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(layout.VersionJSON(parent), []byte(`{"id":"1.21.4","mainClass":"Main","libraries":[{"name":"a:b:1"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	touch(t, layout.Marker(loader), 1)
	if err := os.MkdirAll(filepath.Dir(layout.VersionJSON(loader)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(layout.VersionJSON(loader), []byte(`{"id":"fabric-loader-0.16.10-1.21.4","inheritsFrom":"1.21.4"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if !IsInstalled(layout, loader) {
		t.Error("loader profile should use the inherited vanilla client jar")
	}
}

func TestInstallLoaderInstallsMissingParent(t *testing.T) {
	layout := paths.New(t.TempDir())
	if err := layout.Ensure(); err != nil {
		t.Fatal(err)
	}
	const parent = "1.21.4"
	const loader = "fabric-loader-0.16.10-1.21.4"
	clientJar := []byte("client jar")
	parentLib := []byte("parent library")
	loaderLib := []byte("loader library")
	sha := func(data []byte) string {
		return fmt.Sprintf("%x", sha1.Sum(data))
	}

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/versions/1.21.4.json":
			_, _ = fmt.Fprintf(w, `{
				"id": "1.21.4",
				"type": "release",
				"mainClass": "net.minecraft.client.main.Main",
				"downloads": {"client": {"sha1": %q, "size": %d, "url": %q}},
				"libraries": [{"name": "test:parent:1", "downloads": {"artifact": {
					"path": "test/parent/1/parent-1.jar", "sha1": %q, "size": %d, "url": %q
				}}}]
			}`, sha(clientJar), len(clientJar), server.URL+"/client.jar", sha(parentLib), len(parentLib), server.URL+"/parent.jar")
		case "/client.jar":
			_, _ = w.Write(clientJar)
		case "/parent.jar":
			_, _ = w.Write(parentLib)
		case "/loader.jar":
			_, _ = w.Write(loaderLib)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	loaderJSON := fmt.Sprintf(`{
		"id": %q,
		"inheritsFrom": %q,
		"type": "release",
		"mainClass": "net.fabricmc.loader.impl.launch.knot.KnotClient",
		"libraries": [{"name": "net.fabricmc:fabric-loader:0.16.10", "downloads": {"artifact": {
			"path": "net/fabricmc/fabric-loader/0.16.10/fabric-loader-0.16.10.jar",
			"sha1": %q, "size": %d, "url": %q
		}}}]
	}`, loader, parent, sha(loaderLib), len(loaderLib), server.URL+"/loader.jar")
	if err := os.MkdirAll(filepath.Dir(layout.VersionJSON(loader)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(layout.VersionJSON(loader), []byte(loaderJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	manifest := &mojang.Manifest{Versions: []mojang.VersionSummary{{
		ID:  parent,
		URL: server.URL + "/versions/1.21.4.json",
	}}}
	installer := &Installer{
		Layout:    layout,
		VersionID: loader,
		Manifest:  manifest,
		Client:    server.Client(),
		Threads:   1,
	}
	if err := installer.Install(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !IsInstalled(layout, parent) {
		t.Error("missing parent version was not installed")
	}
	if !IsInstalled(layout, loader) {
		t.Error("loader profile was not marked as installed")
	}
}
