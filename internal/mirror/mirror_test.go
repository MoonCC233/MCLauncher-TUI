package mirror

import (
	"strings"
	"testing"
)

func TestOfficialKeepsTheOriginalURL(t *testing.T) {
	Set(Official)
	if Enabled() {
		t.Fatal("the official source must not be reported as a mirror")
	}
	candidates := Candidates("https://libraries.minecraft.net/com/google/guava/guava.jar")
	if len(candidates) != 1 || candidates[0] != "https://libraries.minecraft.net/com/google/guava/guava.jar" {
		t.Fatalf("candidates = %v", candidates)
	}
	if _, ok := Rewrite("https://libraries.minecraft.net/x.jar"); ok {
		t.Error("Rewrite should do nothing without a mirror")
	}
}

func TestBMCLAPIMapsTheKnownRepositories(t *testing.T) {
	Set(BMCLAPI)
	defer Set(Official)

	cases := map[string]string{
		"https://libraries.minecraft.net/com/google/guava/guava.jar":           bmclapiBase + "/maven/com/google/guava/guava.jar",
		"https://maven.fabricmc.net/net/fabricmc/fabric-loader/0.16.9/x.jar":   bmclapiBase + "/maven/net/fabricmc/fabric-loader/0.16.9/x.jar",
		"https://maven.minecraftforge.net/net/minecraftforge/forge/1.20/x.jar": bmclapiBase + "/maven/net/minecraftforge/forge/1.20/x.jar",
		"https://maven.neoforged.net/releases/net/neoforged/neoforge/21/x.jar": bmclapiBase + "/maven/net/neoforged/neoforge/21/x.jar",
		"https://files.minecraftforge.net/maven/net/minecraftforge/x.jar":      bmclapiBase + "/maven/net/minecraftforge/x.jar",
		"https://resources.download.minecraft.net/ab/abcdef":                   bmclapiBase + "/assets/ab/abcdef",
		"https://meta.fabricmc.net/v2/versions/loader/1.21.4":                  bmclapiBase + "/fabric-meta/v2/versions/loader/1.21.4",
		"https://meta.quiltmc.org/v3/versions/loader/1.21.4":                   bmclapiBase + "/quilt-meta/v3/versions/loader/1.21.4",
		"https://piston-meta.mojang.com/mc/game/version_manifest_v2.json":      bmclapiBase + "/mc/game/version_manifest_v2.json",
	}
	for raw, want := range cases {
		got, ok := Rewrite(raw)
		if !ok {
			t.Errorf("Rewrite(%q) reported no mirror", raw)
			continue
		}
		if got != want {
			t.Errorf("Rewrite(%q) = %q, want %q", raw, got, want)
		}
		candidates := Candidates(raw)
		if len(candidates) != 2 || candidates[0] != want || candidates[1] != raw {
			t.Errorf("Candidates(%q) = %v, want the mirror first and the original as fallback", raw, candidates)
		}
	}
}

func TestUnknownURLsStayOfficial(t *testing.T) {
	Set(BMCLAPI)
	defer Set(Official)

	// A client jar lives on piston-data with no mirror equivalent.
	raw := "https://piston-data.mojang.com/v1/objects/deadbeef/client.jar"
	if _, ok := Rewrite(raw); ok {
		t.Error("piston-data URLs have no mirror mapping")
	}
	if candidates := Candidates(raw); len(candidates) != 1 || candidates[0] != raw {
		t.Errorf("candidates = %v", candidates)
	}
}

func TestBasesAndUnknownKind(t *testing.T) {
	Set("nonsense")
	if Enabled() {
		t.Error("an unknown kind must fall back to the official source")
	}
	Set(BMCLAPI)
	defer Set(Official)
	if !strings.HasSuffix(FabricMetaBase(), "/fabric-meta/v2") {
		t.Errorf("fabric meta base = %q", FabricMetaBase())
	}
	if !strings.HasSuffix(QuiltMetaBase(), "/quilt-meta/v3") {
		t.Errorf("quilt meta base = %q", QuiltMetaBase())
	}
	if !strings.HasSuffix(MavenBase(), "/maven/") {
		t.Errorf("maven base = %q", MavenBase())
	}
}
