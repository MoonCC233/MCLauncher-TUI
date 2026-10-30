package loader

import (
	"strings"
	"testing"

	"github.com/MoonCC233/MCLauncher-TUI/internal/mirror"
)

func TestFabricMetaURLsTryTheMirrorFirst(t *testing.T) {
	mirror.Set(mirror.BMCLAPI)
	defer mirror.Set(mirror.Official)

	urls := fabricMetaURLs("versions/loader/1.21.4")
	if len(urls) != 2 {
		t.Fatalf("urls = %v, want the mirror and the official API", urls)
	}
	if !strings.Contains(urls[0], "bangbang93") {
		t.Errorf("first url = %q, want the mirror", urls[0])
	}
	if urls[1] != fabricMetaBase+"/versions/loader/1.21.4" {
		t.Errorf("second url = %q, want the official API", urls[1])
	}

	// Without a mirror only the official API is left.
	mirror.Set(mirror.Official)
	urls = fabricMetaURLs("versions/loader/1.21.4")
	if len(urls) != 1 || urls[0] != fabricMetaBase+"/versions/loader/1.21.4" {
		t.Errorf("urls = %v, want only the official API", urls)
	}
}

func TestForgeInstallerAlternatesKeepTheRepositoryPath(t *testing.T) {
	primary := "https://bmclapi2.bangbang93.com/maven/net/minecraftforge/forge/1.20.1-47.2.0/forge-1.20.1-47.2.0-installer.jar"
	alts := forgeInstallerAlternates(primary, []string{"https://maven.minecraftforge.net/", "https://files.minecraftforge.net/maven/"})
	if len(alts) != 2 {
		t.Fatalf("alternates = %v", alts)
	}
	want := "https://maven.minecraftforge.net/net/minecraftforge/forge/1.20.1-47.2.0/forge-1.20.1-47.2.0-installer.jar"
	if alts[0] != want {
		t.Errorf("first alternate = %q, want %q", alts[0], want)
	}
}
