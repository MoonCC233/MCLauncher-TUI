package mojang

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/MoonCC233/MCLauncher-TUI/internal/paths"
	"github.com/MoonCC233/MCLauncher-TUI/internal/util"
)

func TestResolveLibrariesModern(t *testing.T) {
	artifact := func(path string) DownloadInfo {
		return DownloadInfo{Path: path, URL: "https://example.test/" + path, SHA1: "abc", Size: 42}
	}
	version := &VersionJSON{Libraries: []Library{
		{
			Name:      "com.example:alpha:1.0",
			Downloads: LibraryDownloads{Artifact: artifact("com/example/alpha/1.0/alpha-1.0.jar")},
		},
		{
			// Duplicate entries must not be downloaded or put on the
			// classpath twice.
			Name:      "com.example:alpha:1.0",
			Downloads: LibraryDownloads{Artifact: artifact("com/example/alpha/1.0/alpha-1.0.jar")},
		},
		{
			// No downloads block: the maven coordinates are enough.
			Name: "com.example:beta:2.0",
		},
		{
			Name:      "com.example:gamma:3.0",
			Rules:     []Rule{{Action: "disallow", OS: &OSRule{Name: util.OSName()}}},
			Downloads: LibraryDownloads{Artifact: artifact("com/example/gamma/3.0/gamma-3.0.jar")},
		},
		{
			Name:      "org.lwjgl:lwjgl:3.3.2:natives-windows",
			Downloads: LibraryDownloads{Artifact: artifact("org/lwjgl/lwjgl/3.3.2/lwjgl-3.3.2-natives-windows.jar")},
		},
		{
			Name:      "com.example:feature:1.0",
			Rules:     []Rule{{Action: "allow", Features: map[string]bool{"has_custom_resolution": true}}},
			Downloads: LibraryDownloads{Artifact: artifact("com/example/feature/1.0/feature-1.0.jar")},
		},
	}}

	res := ResolveLibraries(version, ArgFeatures(1280, 720))
	if err := res.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if len(res.Artifacts) != 4 {
		t.Fatalf("artifacts = %#v, want 4 entries", res.Artifacts)
	}
	if len(res.Classpath) != 4 {
		t.Fatalf("classpath = %#v, want 4 entries", res.Classpath)
	}
	if len(res.Natives) != 1 {
		t.Fatalf("natives = %#v, want 1 entry", res.Natives)
	}
	for _, rel := range res.Classpath {
		if strings.Contains(rel, "gamma") {
			t.Errorf("disallowed library on classpath: %q", rel)
		}
	}

	beta := res.Artifacts[1]
	if beta.Path != "com/example/beta/2.0/beta-2.0.jar" {
		t.Errorf("derived maven path = %q", beta.Path)
	}
	if beta.URL != "https://libraries.minecraft.net/com/example/beta/2.0/beta-2.0.jar" {
		t.Errorf("derived maven URL = %q", beta.URL)
	}
	if !strings.HasSuffix(res.Natives[0].Path, "lwjgl-3.3.2-natives-windows.jar") {
		t.Errorf("native path = %q", res.Natives[0].Path)
	}

	// Without the custom-resolution feature the conditional library is gone.
	res = ResolveLibraries(version, ArgFeatures(0, 0))
	if len(res.Artifacts) != 3 {
		t.Errorf("artifacts without feature = %#v, want 3", res.Artifacts)
	}
	for _, a := range res.Artifacts {
		if strings.Contains(a.Path, "feature") {
			t.Errorf("feature library should have been filtered: %q", a.Path)
		}
	}
}

func TestResolveLibrariesLegacyNatives(t *testing.T) {
	classifier := "natives-" + util.OSName() + "-" + archBits()
	nativePath := "com/example/legacy/1.0/legacy-1.0-" + classifier + ".jar"
	version := &VersionJSON{Libraries: []Library{
		{
			Name:    "com.example:legacy:1.0",
			URL:     "https://libraries.example/",
			Natives: map[string]string{util.OSName(): "natives-" + util.OSName() + "-${arch}"},
			Extract: &ExtractRule{Exclude: []string{"META-INF/"}},
			Downloads: LibraryDownloads{
				Artifact: DownloadInfo{Path: "com/example/legacy/1.0/legacy-1.0.jar"},
				Classifiers: map[string]DownloadInfo{
					classifier: {Path: nativePath, SHA1: "def"},
				},
			},
		},
	}}
	res := ResolveLibraries(version, nil)
	if len(res.Natives) != 1 {
		t.Fatalf("natives = %#v, want one", res.Natives)
	}
	native := res.Natives[0]
	if native.Path != nativePath {
		t.Errorf("native path = %q, want %q", native.Path, nativePath)
	}
	if native.URL != "https://libraries.example/"+nativePath {
		t.Errorf("native URL = %q", native.URL)
	}
	if len(native.Exclude) != 1 || native.Exclude[0] != "META-INF/" {
		t.Errorf("native exclude = %#v", native.Exclude)
	}
	if len(res.Classpath) != 1 || res.Classpath[0] != "com/example/legacy/1.0/legacy-1.0.jar" {
		t.Errorf("classpath = %#v", res.Classpath)
	}
}

func TestResolveLibrariesNativeFallback(t *testing.T) {
	version := &VersionJSON{Libraries: []Library{
		{
			Name:      "com.example:foo-natives:1.0",
			Natives:   map[string]string{util.OSName(): "natives-${arch}"},
			Downloads: LibraryDownloads{Artifact: DownloadInfo{Path: "com/example/foo-natives/1.0/foo-natives-1.0.jar"}},
		},
	}}
	res := ResolveLibraries(version, nil)
	if len(res.Natives) != 1 {
		t.Fatalf("natives = %#v, want the regular artifact as fallback", res.Natives)
	}
	if res.Natives[0].Path != "com/example/foo-natives/1.0/foo-natives-1.0.jar" {
		t.Errorf("fallback native path = %q", res.Natives[0].Path)
	}
}

func TestPathsAndClasspath(t *testing.T) {
	layout := paths.New(t.TempDir())
	res := &LibraryResolution{
		Classpath: []string{
			"org/lwjgl/lwjgl/3.3.2/lwjgl-3.3.2.jar",
			"com/example/alpha/1.0/alpha-1.0.jar",
		},
	}
	clientJar := layout.ClientJar("1.20.1")
	cp := Classpath(layout, res, clientJar)
	parts := strings.Split(cp, util.ClasspathSeparator())
	if len(parts) != 3 {
		t.Fatalf("classpath has %d entries: %q", len(parts), cp)
	}
	if want := filepath.Join(layout.Libraries, "org", "lwjgl", "lwjgl", "3.3.2", "lwjgl-3.3.2.jar"); parts[0] != want {
		t.Errorf("classpath[0] = %q, want %q", parts[0], want)
	}
	if parts[2] != clientJar {
		t.Errorf("classpath should end with the client jar, got %q", parts[2])
	}

	artifact := Artifact{Path: "com/example/alpha/1.0/alpha-1.0.jar"}
	if got, want := ArtifactPath(layout, artifact), filepath.Join(layout.Libraries, "com", "example", "alpha", "1.0", "alpha-1.0.jar"); got != want {
		t.Errorf("ArtifactPath = %q, want %q", got, want)
	}
	native := NativeJar{Path: "org/lwjgl/lwjgl/3.3.2/lwjgl-3.3.2-natives-windows.jar"}
	if got, want := NativePath(layout, native), filepath.Join(layout.Libraries, "org", "lwjgl", "lwjgl", "3.3.2", "lwjgl-3.3.2-natives-windows.jar"); got != want {
		t.Errorf("NativePath = %q, want %q", got, want)
	}

	if err := (&LibraryResolution{Classpath: []string{"not-a-jar.txt"}}).Validate(); err == nil {
		t.Error("Validate should reject non-jar classpath entries")
	}
}
