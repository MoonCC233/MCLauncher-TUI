package packs

import (
	"context"
	"crypto/sha1"
	"crypto/sha512"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/MoonCC233/MCLauncher-TUI/internal/mirror"
)

func TestValidateURLAcceptsBothContentCDNs(t *testing.T) {
	allowed := []string{
		"https://cdn.modrinth.com/data/AANobbMI/versions/x/sodium.jar",
		"https://edge.forgecdn.net/files/9379/687/jei.jar",
		"https://mediafilez.forgecdn.net/files/1/2/pack.zip",
		"https://mod.mcimirror.top/files/9379/687/jei.jar",
		"https://mod.mcimirror.top/data/AANobbMI/versions/x/sodium.jar",
	}
	for _, raw := range allowed {
		if err := ValidateURL(raw); err != nil {
			t.Errorf("ValidateURL(%q) = %v, want it allowed", raw, err)
		}
	}
}

func TestCandidateURLsMirrorKnownContentCDNs(t *testing.T) {
	mirror.Set(mirror.Official)
	defer mirror.Set(mirror.Official)

	modrinth := "https://cdn.modrinth.com/data/AANobbMI/versions/uMOpc5uV/sodium.jar"
	got := candidateURLs(modrinth)
	if len(got) != 2 || got[0] != modrinth || got[1] != "https://mod.mcimirror.top/data/AANobbMI/versions/uMOpc5uV/sodium.jar" {
		t.Fatalf("candidates = %v, want the CDN first and the mirror as fallback", got)
	}

	curseforge := "https://edge.forgecdn.net/files/9379/687/jei.jar"
	got = candidateURLs(curseforge)
	if len(got) != 2 || got[0] != curseforge || got[1] != "https://mod.mcimirror.top/files/9379/687/jei.jar" {
		t.Fatalf("candidates = %v", got)
	}

	// An unknown host is left alone: the launcher must not rewrite arbitrary
	// links the player pasted.
	other := "https://example.com/pack.zip"
	if got := candidateURLs(other); len(got) != 1 || got[0] != other {
		t.Errorf("candidates = %v, want the original URL", got)
	}
}

func TestCandidateURLsPreferTheMirrorWhenEnabled(t *testing.T) {
	mirror.Set(mirror.BMCLAPI)
	defer mirror.Set(mirror.Official)

	raw := "https://cdn.modrinth.com/data/A/versions/V/file.jar"
	got := candidateURLs(raw)
	if len(got) != 2 || got[0] != "https://mod.mcimirror.top/data/A/versions/V/file.jar" || got[1] != raw {
		t.Fatalf("candidates = %v, want the mirror first when the mirror setting is on", got)
	}
}

func TestInstallVerifiesSHA1(t *testing.T) {
	payload := []byte("PK\x03\x04 forge mod")
	sum := sha1.Sum(payload)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer server.Close()
	allowTestHosts(t)

	dir := t.TempDir()
	record := Record{Filename: "forge.jar", SHA1: hex.EncodeToString(sum[:])}
	if _, err := Install(context.Background(), server.Client(), dir, Mod, record, server.URL, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(DirFor(dir, Mod), "forge.jar")); err != nil {
		t.Fatalf("mod file missing: %v", err)
	}

	bad := Record{Filename: "bad.jar", SHA1: strings.Repeat("0", 40)}
	if _, err := Install(context.Background(), server.Client(), dir, Mod, bad, server.URL, nil); err == nil {
		t.Fatal("expected a SHA-1 error")
	}
	if _, err := os.Stat(filepath.Join(DirFor(dir, Mod), "bad.jar")); !os.IsNotExist(err) {
		t.Error("a mod with a bad checksum must not be kept")
	}
}

func TestInstallReusesAFileThatAlreadyMatches(t *testing.T) {
	payload := []byte("PK\x03\x04 cached pack")
	sum := sha512.Sum512(payload)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("different payload"))
	}))
	defer server.Close()
	allowTestHosts(t)

	dir := t.TempDir()
	if err := os.MkdirAll(Dir(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(Dir(dir), "Cached.zip"), payload, 0o644); err != nil {
		t.Fatal(err)
	}

	record := Record{Filename: "Cached.zip", SHA512: hex.EncodeToString(sum[:]), Size: int64(len(payload))}
	installed, err := Install(context.Background(), server.Client(), dir, ResourcePack, record, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if installed.Size != int64(len(payload)) {
		t.Errorf("record = %+v", installed)
	}
	if len(List(dir, ResourcePack)) != 1 {
		t.Error("the reused file should be recorded in the manifest")
	}
}

func TestInstallDoesNotRedownloadAnUnchangedFile(t *testing.T) {
	var requests atomic.Int32
	payload := []byte("PK\x03\x04 mod")
	sum := sha512.Sum512(payload)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_, _ = w.Write(payload)
	}))
	defer server.Close()
	allowTestHosts(t)

	dir := t.TempDir()
	record := Record{Filename: "cached.jar", SHA512: hex.EncodeToString(sum[:]), Size: int64(len(payload))}
	if _, err := Install(context.Background(), server.Client(), dir, Mod, record, server.URL, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(context.Background(), server.Client(), dir, Mod, record, server.URL, nil); err != nil {
		t.Fatal(err)
	}
	if got := requests.Load(); got != 1 {
		t.Errorf("requests = %d, want the second install to reuse the file", got)
	}
}
