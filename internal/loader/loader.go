// Package loader installs mod loaders (Fabric, Quilt, Forge, NeoForge and OptiFine) as
// standalone local versions. The vanilla version stays untouched and every
// loader profile uses inheritsFrom, so the shared libraries and assets are
// reused instead of copied.
package loader

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/MoonCC233/MCLauncher-TUI/internal/install"
	"github.com/MoonCC233/MCLauncher-TUI/internal/mirror"
	"github.com/MoonCC233/MCLauncher-TUI/internal/mojang"
	"github.com/MoonCC233/MCLauncher-TUI/internal/paths"
	"github.com/MoonCC233/MCLauncher-TUI/internal/util"
)

type Kind string

const (
	Fabric   Kind = "fabric"
	Quilt    Kind = "quilt"
	Forge    Kind = "forge"
	NeoForge Kind = "neoforge"
	OptiFine Kind = "optifine"
)

var Kinds = []Kind{Fabric, Quilt, Forge, NeoForge, OptiFine}

func ValidKind(kind Kind) bool {
	for _, k := range Kinds {
		if k == kind {
			return true
		}
	}
	return false
}

// Meta is stored next to the loader version JSON. The version directory alone
// cannot tell whether a profile was created by Fabric, Forge or OptiFine.
type Meta struct {
	Kind          Kind   `json:"kind"`
	Minecraft     string `json:"minecraft"`
	LoaderVersion string `json:"loader_version,omitempty"`
	Edition       string `json:"edition,omitempty"`
	VersionID     string `json:"version_id"`
	InstalledAt   int64  `json:"installed_at"`
}

func ReadMeta(layout paths.Layout, versionID string) (*Meta, error) {
	var meta Meta
	if err := util.ReadJSON(layout.LoaderMeta(versionID), &meta); err != nil {
		return nil, err
	}
	if meta.VersionID == "" {
		meta.VersionID = versionID
	}
	if !ValidKind(meta.Kind) {
		return nil, fmt.Errorf("unknown loader kind %q", meta.Kind)
	}
	return &meta, nil
}

func WriteMeta(layout paths.Layout, meta Meta) error {
	if meta.InstalledAt == 0 {
		meta.InstalledAt = time.Now().Unix()
	}
	return util.WriteJSONAtomic(layout.LoaderMeta(meta.VersionID), &meta)
}

// Scan returns loader metadata for every version directory that has a sidecar.
func Scan(layout paths.Layout) map[string]Meta {
	out := map[string]Meta{}
	entries, err := os.ReadDir(layout.Versions)
	if err != nil {
		return out
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if meta, err := ReadMeta(layout, entry.Name()); err == nil {
			out[entry.Name()] = *meta
		}
	}
	return out
}

// RemoteVersion is one selectable loader build for a Minecraft version.
type RemoteVersion struct {
	Version string
	Stable  bool
}

const (
	fabricMetaBase    = "https://meta.fabricmc.net/v2"
	quiltMetaBase     = "https://meta.quiltmc.org/v3"
	forgeMavenBase    = "https://maven.minecraftforge.net"
	neoforgeMavenBase = "https://maven.neoforged.net/releases"
)

// fabricMetaURL resolves a path against the official Fabric meta service or
// the mirror configured with MCLAUNCHER_FABRIC_MIRROR. The mirror expects the
// full v2 base, for example https://bmclapi2.bangbang93.com/fabric-meta/v2.
func fabricMetaURL(path string) string {
	base := fabricMetaBase
	if override := strings.TrimRight(strings.TrimSpace(os.Getenv("MCLAUNCHER_FABRIC_MIRROR")), "/"); override != "" {
		base = override
	} else if mirrored := mirror.FabricMetaBase(); mirrored != "" {
		base = mirrored
	}
	return base + "/" + strings.TrimLeft(path, "/")
}

// quiltMetaURL is the Quilt equivalent of fabricMetaURL.
func quiltMetaURL(path string) string {
	base := quiltMetaBase
	if override := strings.TrimRight(strings.TrimSpace(os.Getenv("MCLAUNCHER_QUILT_MIRROR")), "/"); override != "" {
		base = override
	} else if mirrored := mirror.QuiltMetaBase(); mirrored != "" {
		base = mirrored
	}
	return base + "/" + strings.TrimLeft(path, "/")
}

func fabricMavenMirror() string {
	if override := strings.TrimRight(strings.TrimSpace(os.Getenv("MCLAUNCHER_FABRIC_MAVEN_MIRROR")), "/"); override != "" {
		return override
	}
	return strings.TrimRight(mirror.MavenBase(), "/")
}

func quiltMavenMirror() string {
	if override := strings.TrimRight(strings.TrimSpace(os.Getenv("MCLAUNCHER_QUILT_MAVEN_MIRROR")), "/"); override != "" {
		return override
	}
	return strings.TrimRight(mirror.MavenBase(), "/")
}

func rewriteMavenURL(url, base, mirror string) string {
	if url == "" || mirror == "" || base == "" || !strings.HasPrefix(url, base) {
		return url
	}
	return mirror + "/" + strings.TrimPrefix(url, base)
}

func rewriteProfileMaven(v *mojang.VersionJSON, base, mirror string) {
	if v == nil || mirror == "" {
		return
	}
	for i := range v.Libraries {
		lib := &v.Libraries[i]
		lib.URL = rewriteMavenURL(lib.URL, base, mirror)
		lib.Downloads.Artifact.URL = rewriteMavenURL(lib.Downloads.Artifact.URL, base, mirror)
		for key, info := range lib.Downloads.Classifiers {
			info.URL = rewriteMavenURL(info.URL, base, mirror)
			lib.Downloads.Classifiers[key] = info
		}
	}
}

// FetchVersions downloads the available loader builds for a Minecraft version.
// OptiFine has no public API; it is imported from a local jar instead.
func FetchVersions(ctx context.Context, client *http.Client, kind Kind, minecraft string) ([]RemoteVersion, error) {
	if strings.TrimSpace(minecraft) == "" {
		return nil, fmt.Errorf("missing Minecraft version")
	}
	if client == nil {
		client = util.NewHTTPClient()
	}
	switch kind {
	case Fabric:
		return fetchFabricVersions(ctx, client, minecraft)
	case Quilt:
		return fetchQuiltVersions(ctx, client, minecraft)
	case Forge:
		return fetchForgeVersions(ctx, client, minecraft)
	case NeoForge:
		return fetchNeoForgeVersions(ctx, client, minecraft)
	case OptiFine:
		return nil, fmt.Errorf("OptiFine is installed from a local jar")
	default:
		return nil, fmt.Errorf("unknown loader %q", kind)
	}
}

type loaderVersionEntry struct {
	Loader struct {
		Version string `json:"version"`
		Stable  *bool  `json:"stable"`
	} `json:"loader"`
}

// parseMetaVersions reads the Fabric v2 and Quilt v3 game-specific replies.
// Both nest the loader version under "loader"; Quilt omits "stable", so the
// version string is used as the fallback.
func parseMetaVersions(raw []loaderVersionEntry) []RemoteVersion {
	out := make([]RemoteVersion, 0, len(raw))
	for _, v := range raw {
		if v.Loader.Version == "" {
			continue
		}
		stable := !strings.Contains(v.Loader.Version, "-")
		if v.Loader.Stable != nil {
			stable = *v.Loader.Stable
		}
		out = append(out, RemoteVersion{Version: v.Loader.Version, Stable: stable})
	}
	return out
}

func fetchFabricVersions(ctx context.Context, client *http.Client, minecraft string) ([]RemoteVersion, error) {
	var raw []loaderVersionEntry
	url := fabricMetaURL("versions/loader/" + minecraft)
	if err := util.FetchJSON(ctx, client, url, &raw); err != nil {
		return nil, err
	}
	return parseMetaVersions(raw), nil
}

func fetchQuiltVersions(ctx context.Context, client *http.Client, minecraft string) ([]RemoteVersion, error) {
	var raw []loaderVersionEntry
	url := quiltMetaURL("versions/loader/" + minecraft)
	if err := util.FetchJSON(ctx, client, url, &raw); err != nil {
		return nil, err
	}
	return parseMetaVersions(raw), nil
}

type mavenMetadata struct {
	Versions []string `xml:"versioning>versions>version"`
}

func fetchForgeVersions(ctx context.Context, client *http.Client, minecraft string) ([]RemoteVersion, error) {
	url := forgeMavenURL("net/minecraftforge/forge/maven-metadata.xml")
	body, err := fetchText(ctx, client, url)
	if err != nil {
		return nil, err
	}
	return parseForgeMetadata([]byte(body), minecraft)
}

func parseForgeMetadata(body []byte, minecraft string) ([]RemoteVersion, error) {
	var meta mavenMetadata
	if err := xml.Unmarshal(body, &meta); err != nil {
		return nil, fmt.Errorf("parse Forge metadata: %w", err)
	}
	prefix := minecraft + "-"
	out := make([]RemoteVersion, 0, len(meta.Versions))
	for _, full := range meta.Versions {
		if !strings.HasPrefix(full, prefix) {
			continue
		}
		version := strings.TrimPrefix(full, prefix)
		if version == "" || strings.Contains(version, "-installer") {
			continue
		}
		out = append(out, RemoteVersion{Version: version, Stable: true})
	}
	// Maven metadata is ordered oldest first; the picker expects newest first.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

func neoforgeGroupAndPrefix(minecraft string) (group, prefix string, err error) {
	if minecraft == "1.20.1" {
		return "net/neoforged/forge", "1.20.1-", nil
	}
	parts := strings.Split(minecraft, ".")
	if len(parts) < 2 || parts[0] != "1" {
		return "", "", fmt.Errorf("NeoForge does not support Minecraft %s", minecraft)
	}
	minor, parseErr := strconv.Atoi(parts[1])
	if parseErr != nil {
		return "", "", fmt.Errorf("NeoForge does not support Minecraft %s", minecraft)
	}
	patch := 0
	if len(parts) > 2 {
		patch, parseErr = strconv.Atoi(parts[2])
		if parseErr != nil {
			return "", "", fmt.Errorf("NeoForge does not support Minecraft %s", minecraft)
		}
	}
	if minor < 20 || (minor == 20 && patch < 2) {
		return "", "", fmt.Errorf("NeoForge does not support Minecraft %s", minecraft)
	}
	return "net/neoforged/neoforge", fmt.Sprintf("%d.%d.", minor, patch), nil
}

func fetchNeoForgeVersions(ctx context.Context, client *http.Client, minecraft string) ([]RemoteVersion, error) {
	group, prefix, err := neoforgeGroupAndPrefix(minecraft)
	if err != nil {
		return nil, err
	}
	url := neoforgeMavenURL(group + "/maven-metadata.xml")
	body, err := fetchText(ctx, client, url)
	if err != nil {
		return nil, err
	}
	return parseNeoForgeMetadata([]byte(body), prefix)
}

func parseNeoForgeMetadata(body []byte, prefix string) ([]RemoteVersion, error) {
	var meta mavenMetadata
	if err := xml.Unmarshal(body, &meta); err != nil {
		return nil, fmt.Errorf("parse NeoForge metadata: %w", err)
	}
	out := make([]RemoteVersion, 0, len(meta.Versions))
	for _, full := range meta.Versions {
		if !strings.HasPrefix(full, prefix) {
			continue
		}
		version := full
		// 1.20.1 NeoForge versions are "1.20.1-47.1.106"; the launcher stores
		// them as 47.1.106. Modern versions ("21.1.265") have no MC prefix.
		if strings.HasSuffix(prefix, "-") {
			version = strings.TrimPrefix(full, prefix)
		}
		if version == "" || strings.Contains(version, "-installer") {
			continue
		}
		out = append(out, RemoteVersion{Version: version, Stable: !strings.Contains(version, "-")})
	}
	// Maven metadata is ordered oldest first; the picker expects newest first.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

// Installer creates a loader version. The vanilla base version is installed
// first when needed, so a loader always has a complete parent.
type Installer struct {
	Layout        paths.Layout
	Kind          Kind
	Minecraft     string
	LoaderVersion string
	OptiFineJar   string
	JavaPath      string
	Client        *http.Client
	Manifest      *mojang.Manifest
	Threads       int
	OnEvent       func(install.Event)

	// SkipBaseInstall is used by tests and by callers that already verified the
	// vanilla version.
	SkipBaseInstall bool
}

func (in *Installer) emit(ev install.Event) {
	if in.OnEvent != nil {
		in.OnEvent(ev)
	}
}

func (in *Installer) client() *http.Client {
	if in.Client != nil {
		return in.Client
	}
	return util.NewHTTPClient()
}

func (in *Installer) VersionID() string {
	switch in.Kind {
	case Fabric:
		return fmt.Sprintf("fabric-loader-%s-%s", in.LoaderVersion, in.Minecraft)
	case Quilt:
		return fmt.Sprintf("quilt-loader-%s-%s", in.LoaderVersion, in.Minecraft)
	case Forge:
		return ForgeVersionID(in.Minecraft, in.LoaderVersion)
	case NeoForge:
		return NeoForgeVersionID(in.Minecraft, in.LoaderVersion)
	case OptiFine:
		_, edition, _ := ParseOptiFineJarName(filepath.Base(in.OptiFineJar))
		return OptiFineVersionID(in.Minecraft, edition)
	}
	return ""
}

func ForgeVersionID(minecraft, forge string) string {
	return fmt.Sprintf("%s-forge-%s", minecraft, forge)
}

func NeoForgeVersionID(minecraft, neoforge string) string {
	return fmt.Sprintf("%s-neoforge-%s", minecraft, neoforge)
}

func OptiFineVersionID(minecraft, edition string) string {
	return fmt.Sprintf("%s-OptiFine-%s", minecraft, edition)
}

func (in *Installer) Install(ctx context.Context) error {
	if !ValidKind(in.Kind) {
		return fmt.Errorf("unknown loader %q", in.Kind)
	}
	if strings.TrimSpace(in.Minecraft) == "" {
		return fmt.Errorf("missing Minecraft version")
	}
	if in.Kind != OptiFine && strings.TrimSpace(in.LoaderVersion) == "" {
		return fmt.Errorf("missing loader version")
	}
	if in.Kind == OptiFine && strings.TrimSpace(in.OptiFineJar) == "" {
		return fmt.Errorf("missing OptiFine jar")
	}
	if err := in.Layout.Ensure(); err != nil {
		return err
	}
	if !in.SkipBaseInstall && !install.IsInstalled(in.Layout, in.Minecraft) {
		in.emit(install.Event{Type: install.EventStage, Stage: "Installing Minecraft", Detail: in.Minecraft})
		base := &install.Installer{
			Layout:    in.Layout,
			VersionID: in.Minecraft,
			Manifest:  in.Manifest,
			Threads:   in.Threads,
			Client:    in.client(),
			OnEvent:   in.OnEvent,
		}
		if err := base.Install(ctx); err != nil {
			return fmt.Errorf("install Minecraft %s: %w", in.Minecraft, err)
		}
	}

	var (
		versionID string
		err       error
	)
	switch in.Kind {
	case Fabric:
		versionID, err = in.installProfile(ctx, fabricMetaURL("versions/loader/"+in.Minecraft+"/"+in.LoaderVersion+"/profile/json"), Fabric)
	case Quilt:
		versionID, err = in.installProfile(ctx, quiltMetaURL("versions/loader/"+in.Minecraft+"/"+in.LoaderVersion+"/profile/json"), Quilt)
	case Forge:
		versionID, err = in.installForge(ctx)
	case NeoForge:
		versionID, err = in.installNeoForge(ctx)
	case OptiFine:
		versionID, err = in.installOptiFine(ctx)
	}
	if err != nil {
		return err
	}
	if versionID == "" {
		return fmt.Errorf("loader installer did not produce a version")
	}
	if err := in.finish(ctx, versionID); err != nil {
		return err
	}
	in.emit(install.Event{Type: install.EventDone, Stage: "Ready"})
	return nil
}

// finish installs the libraries and assets referenced by the merged profile and
// writes the marker that makes the version launchable.
func (in *Installer) finish(ctx context.Context, versionID string) error {
	in.emit(install.Event{Type: install.EventStage, Stage: "Installing loader libraries", Detail: versionID})
	skipClient := in.Kind == OptiFine
	installer := &install.Installer{
		Layout:     in.Layout,
		VersionID:  versionID,
		Manifest:   in.Manifest,
		Threads:    in.Threads,
		Client:     in.client(),
		SkipClient: skipClient,
		OnEvent:    in.OnEvent,
	}
	if err := installer.Install(ctx); err != nil {
		return fmt.Errorf("install %s files: %w", versionID, err)
	}
	return nil
}

func (in *Installer) installProfile(ctx context.Context, url string, kind Kind) (string, error) {
	in.emit(install.Event{Type: install.EventStage, Stage: "Downloading loader profile", Detail: string(kind) + " " + in.LoaderVersion})
	var profile mojang.VersionJSON
	if err := util.FetchJSON(ctx, in.client(), url, &profile); err != nil {
		return "", err
	}
	switch kind {
	case Fabric:
		rewriteProfileMaven(&profile, "https://maven.fabricmc.net/", fabricMavenMirror())
	case Quilt:
		rewriteProfileMaven(&profile, "https://maven.quiltmc.org/", quiltMavenMirror())
		// Quilt profiles use Fabric's Maven for ASM, intermediary and mixin.
		rewriteProfileMaven(&profile, "https://maven.fabricmc.net/", fabricMavenMirror())
	}
	versionID := in.VersionID()
	profile.ID = versionID
	profile.InheritsFrom = in.Minecraft
	profile.JarID = ""
	if profile.MainClass == "" {
		return "", fmt.Errorf("%s profile has no main class", kind)
	}
	if profile.Type == "" {
		profile.Type = "release"
	}
	if err := util.WriteJSONAtomic(in.Layout.VersionJSON(versionID), &profile); err != nil {
		return "", err
	}
	meta := Meta{
		Kind:          kind,
		Minecraft:     in.Minecraft,
		LoaderVersion: in.LoaderVersion,
		VersionID:     versionID,
		InstalledAt:   time.Now().Unix(),
	}
	if err := WriteMeta(in.Layout, meta); err != nil {
		return "", err
	}
	return versionID, nil
}

var optiFineNameRE = regexp.MustCompile(`(?i)^OptiFine[_-]([^_]+)[_-](.+)$`)

// ParseOptiFineJarName extracts the Minecraft version and OptiFine edition from
// a filename such as OptiFine_1.20.1_HD_U_I6.jar.
func ParseOptiFineJarName(name string) (minecraft, edition string, ok bool) {
	name = strings.TrimSpace(name)
	name = strings.TrimSuffix(name, filepath.Ext(name))
	m := optiFineNameRE.FindStringSubmatch(name)
	if m == nil {
		return "", "", false
	}
	minecraft = strings.TrimSpace(m[1])
	edition = strings.TrimSpace(m[2])
	if minecraft == "" || edition == "" || strings.ContainsAny(edition, `/\`) {
		return "", "", false
	}
	return minecraft, edition, true
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := util.EnsureDir(filepath.Dir(dst)); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func writeLibrary(layout paths.Layout, rel string, data []byte) (sha1 string, size int64, err error) {
	dest, err := util.SafeJoin(layout.Libraries, rel)
	if err != nil {
		return "", 0, err
	}
	if err := util.EnsureDir(filepath.Dir(dest)); err != nil {
		return "", 0, err
	}
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		return "", 0, err
	}
	sum, err := util.SHA1File(dest)
	if err != nil {
		return "", 0, err
	}
	return sum, int64(len(data)), nil
}

// artifactRef builds a local-only library entry. Installer skips entries with
// an empty URL, but launch still puts them on the classpath.
func artifactRef(layout paths.Layout, name, rel string) mojang.Library {
	lib := mojang.Library{
		Name:  name,
		Local: true,
		Downloads: mojang.LibraryDownloads{
			Artifact: mojang.DownloadInfo{Path: filepath.ToSlash(rel)},
		},
	}
	full := filepath.Join(layout.Libraries, filepath.FromSlash(rel))
	if st, err := os.Stat(full); err == nil {
		lib.Downloads.Artifact.Size = st.Size()
		lib.Size = st.Size()
		if sum, err := util.SHA1File(full); err == nil {
			lib.Downloads.Artifact.SHA1 = sum
			lib.SHA1 = sum
		}
	}
	return lib
}

func fetchText(ctx context.Context, client *http.Client, url string) (string, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("User-Agent", "mclauncher/0.5 (+https://github.com/MoonCC233/MCLauncher-TUI)")
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
		} else {
			data, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK && readErr == nil {
				return string(data), nil
			}
			if readErr != nil {
				lastErr = readErr
			} else {
				lastErr = fmt.Errorf("GET %s: HTTP %d", url, resp.StatusCode)
			}
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 600 * time.Millisecond):
		}
	}
	return "", lastErr
}
