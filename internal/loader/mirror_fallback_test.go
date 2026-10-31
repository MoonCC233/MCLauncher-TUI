package loader

import (
	"strings"
	"testing"

	"github.com/MoonCC233/MCLauncher-TUI/internal/mirror"
)

func TestFabricMetaURLsTryTheDomesticMirrorFirst(t *testing.T) {
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
}

func TestLoaderMetadataURLsIncludeTheDomesticMirror(t *testing.T) {
	forge := forgeMetadataURLs("net/minecraftforge/forge/maven-metadata.xml")
	if !containsURL(forge, mirror.BMCLAPIBase) || !containsURL(forge, mirror.SJTUBase) || !containsURL(forge, forgeMavenBase) {
		t.Errorf("forge urls = %v", forge)
	}
	neo := neoforgeMetadataURLs("net/neoforged/neoforge/maven-metadata.xml")
	if !containsURL(neo, mirror.BMCLAPIBase) || !containsURL(neo, mirror.SJTUBase) || !containsURL(neo, neoforgeMavenBase) {
		t.Errorf("neoforge urls = %v", neo)
	}
	quilt := quiltMetaURLs("versions/loader/1.21.4")
	if len(quilt) < 2 || !strings.Contains(quilt[0], "bangbang93") || quilt[1] != quiltMetaBase+"/versions/loader/1.21.4" {
		t.Errorf("quilt urls = %v", quilt)
	}
}

func containsURL(urls []string, prefix string) bool {
	for _, raw := range urls {
		if strings.HasPrefix(raw, prefix) {
			return true
		}
	}
	return false
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
