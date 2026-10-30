// Package mirror routes Minecraft and mod-loader downloads through a regional
// mirror. It is a small URL mapping table plus the currently selected source:
// everything that downloads a file asks this package for the URLs to try, so a
// single setting switches the whole launcher.
package mirror

import (
	"strings"
)

// Kind is the download source the player picked in Settings.
type Kind string

const (
	// Official downloads straight from Mojang and the loader projects.
	Official Kind = ""
	// BMCLAPI is the community mirror that is much faster in mainland China.
	BMCLAPI Kind = "bmclapi"
)

// Kinds lists the selectable sources, in picker order.
var Kinds = []Kind{Official, BMCLAPI}

// bmclapiBase is the root of the BMCLAPI mirror.
const bmclapiBase = "https://bmclapi2.bangbang93.com"

var current = Official

// Set switches the mirror used for the rest of the run. Unknown values fall back
// to the official servers.
func Set(kind Kind) {
	if kind != BMCLAPI {
		kind = Official
	}
	current = kind
}

// Current returns the source in use.
func Current() Kind { return current }

// Enabled reports whether downloads should go through the mirror.
func Enabled() bool { return current != Official }

// Base returns the mirror root, or "" when the official servers are in use.
func Base() string {
	if !Enabled() {
		return ""
	}
	return bmclapiBase
}

// mavenPrefixes are the repository roots that the mirror serves under /maven/.
var mavenPrefixes = []string{
	"https://libraries.minecraft.net/",
	"https://maven.fabricmc.net/",
	"https://maven.quiltmc.org/repository/release/",
	"https://maven.quiltmc.org/",
	"https://maven.minecraftforge.net/",
	"https://maven.neoforged.net/releases/",
	"https://files.minecraftforge.net/maven/",
}

// Rewrite maps one download URL onto the mirror. ok is false when the URL has no
// mirror equivalent, which leaves the caller on the official server.
func Rewrite(raw string) (string, bool) {
	base := Base()
	if base == "" || raw == "" {
		return "", false
	}
	for _, prefix := range mavenPrefixes {
		if strings.HasPrefix(raw, prefix) {
			return base + "/maven/" + strings.TrimPrefix(raw, prefix), true
		}
	}
	switch {
	case strings.HasPrefix(raw, "https://resources.download.minecraft.net/"):
		return base + "/assets/" + strings.TrimPrefix(raw, "https://resources.download.minecraft.net/"), true
	case strings.HasPrefix(raw, "https://meta.fabricmc.net/"):
		return base + "/fabric-meta/" + strings.TrimPrefix(raw, "https://meta.fabricmc.net/"), true
	case strings.HasPrefix(raw, "https://meta.quiltmc.org/"):
		return base + "/quilt-meta/" + strings.TrimPrefix(raw, "https://meta.quiltmc.org/"), true
	case strings.HasSuffix(raw, "/mc/game/version_manifest.json"):
		return base + "/mc/game/version_manifest.json", true
	case strings.HasSuffix(raw, "/mc/game/version_manifest_v2.json"):
		return base + "/mc/game/version_manifest_v2.json", true
	}
	return "", false
}

// Candidates returns the URLs to try for one download: the mirror first when it
// is enabled, then the official URL so a missing mirror file never breaks an
// install.
func Candidates(raw string) []string {
	if mirrored, ok := Rewrite(raw); ok && mirrored != raw {
		return []string{mirrored, raw}
	}
	return []string{raw}
}

// FabricMetaBase is the base the Fabric meta API is read from.
func FabricMetaBase() string {
	if !Enabled() {
		return ""
	}
	return bmclapiBase + "/fabric-meta/v2"
}

// QuiltMetaBase is the base the Quilt meta API is read from.
func QuiltMetaBase() string {
	if !Enabled() {
		return ""
	}
	return bmclapiBase + "/quilt-meta/v3"
}

// MavenBase is the mirror root that serves maven repositories, with a trailing
// slash, or "" when the official repositories should be used.
func MavenBase() string {
	if !Enabled() {
		return ""
	}
	return bmclapiBase + "/maven/"
}
