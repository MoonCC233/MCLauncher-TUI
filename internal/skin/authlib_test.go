package skin

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInjectorPathUsesVersion(t *testing.T) {
	got := InjectorPath(filepath.Join("cache", "minecraft"))
	if !strings.Contains(got, "authlib-injector-"+InjectorVersion+".jar") {
		t.Errorf("path = %q", got)
	}
}

func TestInjectorURLsPreferMirror(t *testing.T) {
	t.Setenv("MCLAUNCHER_AUTHLIB_MIRROR", "https://mirror.test/{url}")
	urls := injectorURLs(injectorURL)
	if len(urls) < 2 {
		t.Fatalf("urls = %v", urls)
	}
	if urls[0] != strings.ReplaceAll("https://mirror.test/{url}", "{url}", injectorURL) {
		t.Errorf("first url = %q", urls[0])
	}
	if urls[len(urls)-1] != injectorURL {
		t.Errorf("last url = %q", urls[len(urls)-1])
	}
}

func TestEnsureInjectorHonoursOverride(t *testing.T) {
	dir := t.TempDir()
	jar := filepath.Join(dir, "custom.jar")
	if err := os.WriteFile(jar, []byte("jar"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MCLAUNCHER_AUTHLIB_INJECTOR", jar)
	got, err := EnsureInjector(context.Background(), nil, dir)
	if err != nil {
		t.Fatalf("EnsureInjector: %v", err)
	}
	if got != jar {
		t.Errorf("path = %q, want %q", got, jar)
	}
}

func TestEnsureInjectorRejectsMissingOverride(t *testing.T) {
	t.Setenv("MCLAUNCHER_AUTHLIB_INJECTOR", filepath.Join(t.TempDir(), "missing.jar"))
	if _, err := EnsureInjector(context.Background(), nil, t.TempDir()); err == nil {
		t.Error("missing override was accepted")
	}
}

func TestVerifyInjectorRejectsWrongSize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.jar")
	if err := os.WriteFile(path, []byte("too small"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifyInjector(path); err == nil {
		t.Error("wrong-size jar passed verification")
	}
}
