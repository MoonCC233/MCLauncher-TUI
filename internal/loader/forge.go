package loader

import (
	"archive/zip"
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/MoonCC233/MCLauncher-TUI/internal/install"
	"github.com/MoonCC233/MCLauncher-TUI/internal/mirror"
	"github.com/MoonCC233/MCLauncher-TUI/internal/mojang"
	"github.com/MoonCC233/MCLauncher-TUI/internal/paths"
	"github.com/MoonCC233/MCLauncher-TUI/internal/util"
)

type forgeArtifact struct {
	Path string `json:"path"`
	URL  string `json:"url"`
	SHA1 string `json:"sha1"`
	Size int64  `json:"size"`
}

type forgeLibrary struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	Downloads struct {
		Artifact forgeArtifact `json:"artifact"`
	} `json:"downloads"`
}

type forgeInstallProfile struct {
	Version     string         `json:"version"`
	Minecraft   string         `json:"minecraft"`
	Profile     string         `json:"profile"`
	Libraries   []forgeLibrary `json:"libraries"`
	VersionInfo *struct {
		Libraries []forgeLibrary `json:"libraries"`
	} `json:"versionInfo"`
}

func (p *forgeInstallProfile) allLibraries() []forgeLibrary {
	if p == nil {
		return nil
	}
	out := append([]forgeLibrary(nil), p.Libraries...)
	if p.VersionInfo != nil {
		out = append(out, p.VersionInfo.Libraries...)
	}
	return out
}

// forgeVariant describes the differences between the Forge and NeoForge
// installers. Both are the same installer codebase with a different Maven
// repository and version naming scheme.
type forgeVariant struct {
	kind         Kind
	label        string
	mavenBase    string
	mavenBases   []string
	mirror       string
	installerRel string
	cacheDir     string
	targetID     string
}

func (in *Installer) installForge(ctx context.Context) (string, error) {
	coord := in.Minecraft + "-" + in.LoaderVersion
	return in.installForgeLike(ctx, forgeVariant{
		kind:       Forge,
		label:      "Forge",
		mavenBase:  forgeMavenBase,
		mavenBases: []string{"https://maven.minecraftforge.net/", "https://files.minecraftforge.net/maven/"},
		mirror:     forgeMirror(),
		installerRel: fmt.Sprintf(
			"net/minecraftforge/forge/%s/forge-%s-installer.jar", coord, coord),
		cacheDir: "forge",
		targetID: ForgeVersionID(in.Minecraft, in.LoaderVersion),
	})
}

func (in *Installer) installNeoForge(ctx context.Context) (string, error) {
	var installerRel string
	if in.Minecraft == "1.20.1" {
		coord := "1.20.1-" + in.LoaderVersion
		installerRel = fmt.Sprintf("net/neoforged/forge/%s/forge-%s-installer.jar", coord, coord)
	} else {
		installerRel = fmt.Sprintf("net/neoforged/neoforge/%s/neoforge-%s-installer.jar",
			in.LoaderVersion, in.LoaderVersion)
	}
	return in.installForgeLike(ctx, forgeVariant{
		kind:         NeoForge,
		label:        "NeoForge",
		mavenBase:    neoforgeMavenBase,
		mavenBases:   []string{"https://maven.neoforged.net/releases/", "https://maven.minecraftforge.net/", "https://files.minecraftforge.net/maven/"},
		mirror:       neoforgeMirror(),
		installerRel: installerRel,
		cacheDir:     "neoforge",
		targetID:     NeoForgeVersionID(in.Minecraft, in.LoaderVersion),
	})
}

func (in *Installer) installForgeLike(ctx context.Context, v forgeVariant) (string, error) {
	url := v.mavenBase + "/" + v.installerRel
	if v.mirror != "" {
		url = v.mirror + "/" + v.installerRel
	}
	installerPath := filepath.Join(in.Layout.Cache, v.cacheDir, filepath.Base(v.installerRel))

	in.emit(install.Event{
		Type:   install.EventStage,
		Stage:  "Downloading " + v.label + " installer",
		Detail: filepath.Base(v.installerRel),
	})
	sha1 := fetchOptionalSHA1(ctx, in.client(), url+".sha1")
	needDownload := !util.VerifyFile(installerPath, sha1, 0)
	if !needDownload {
		// Without a checksum an interrupted download can look valid by size, so
		// make sure the zip really contains an install profile.
		if _, err := readForgeProfile(installerPath); err != nil {
			_ = os.Remove(installerPath)
			needDownload = true
		}
	}
	if needDownload {
		if err := util.DownloadFile(ctx, in.client(), util.DownloadSpec{
			URL:  url,
			Dest: installerPath,
			SHA1: sha1,
		}, nil); err != nil {
			return "", fmt.Errorf("download %s installer: %w", v.label, err)
		}
		if _, err := readForgeProfile(installerPath); err != nil {
			_ = os.Remove(installerPath)
			return "", fmt.Errorf("%s installer is corrupt: %w", v.label, err)
		}
	}

	profile, profileErr := readForgeProfile(installerPath)
	if profileErr == nil && profile.Minecraft != "" && profile.Minecraft != in.Minecraft {
		return "", fmt.Errorf("%s installer is for Minecraft %s, not %s", v.label, profile.Minecraft, in.Minecraft)
	}

	if err := ensureLauncherProfiles(in.Layout); err != nil {
		return "", err
	}

	// The installers probe the Forge site before doing anything useful and
	// their URLConnection has no timeout, so a filtered host can hang the whole
	// installation forever. They also give up after one try on a slow mirror.
	// Fetch the libraries ourselves with retries, then let the installer run
	// offline against the validated local files.
	offline := false
	if profileErr == nil {
		libs := profile.allLibraries()
		// NeoForge 1.20.1 keeps the FancyModLoader runtime libraries in the
		// embedded version.json rather than install_profile.json.
		if extra, err := readForgeVersionLibraries(installerPath); err == nil {
			libs = append(libs, extra...)
		}
		if len(libs) > 0 {
			if err := in.prepareForgeLibraries(ctx, libs, v.mirror, v.mavenBases, v.label); err != nil {
				return "", err
			}
			offline = true
		}
	}

	in.emit(install.Event{
		Type:   install.EventStage,
		Stage:  "Running " + v.label + " installer",
		Detail: v.targetID,
	})
	javaPath := in.JavaPath
	if javaPath == "" {
		javaPath = "java"
	}
	args := make([]string, 0, 8)
	if !offline {
		// Keep the online fallback from hanging on the certificate probe.
		args = append(args,
			"-Dsun.net.client.defaultConnectTimeout=10000",
			"-Dsun.net.client.defaultReadTimeout=60000",
		)
	}
	args = append(args, "-jar", installerPath, "--installClient", in.Layout.Root)
	if offline {
		args = append(args, "--offline")
	} else if v.mirror != "" {
		// The installer concatenates the mirror URL with the maven path
		// directly, so the trailing slash is required or it requests
		// /mavenorg/ow2/asm/....
		args = append(args, "--mirror", v.mirror+"/")
	}
	if err := runJava(ctx, javaPath, args, in.Layout.Root, func(line string) {
		in.emit(install.Event{Type: install.EventLog, Stage: "Running " + v.label + " installer", Detail: line})
	}); err != nil {
		return "", fmt.Errorf("%s installer: %w", v.label, err)
	}

	generatedID := ""
	if profileErr == nil {
		generatedID = profile.Version
	}
	if generatedID == "" {
		generatedID = in.findGeneratedVersion(v)
	} else if _, err := os.Stat(in.Layout.VersionJSON(generatedID)); err != nil {
		generatedID = in.findGeneratedVersion(v)
	}
	if generatedID == "" {
		return "", fmt.Errorf("%s installer finished but no version file was created", v.label)
	}
	if generatedID != v.targetID {
		if err := adoptVersion(in.Layout, generatedID, v.targetID); err != nil {
			return "", fmt.Errorf("adopt %s version: %w", v.label, err)
		}
	}
	if err := normalizeVersionID(in.Layout, v.targetID); err != nil {
		return "", err
	}
	if err := rewriteForgeVersionMirror(in.Layout, v.targetID, v.mirror, v.mavenBases); err != nil {
		return "", err
	}
	meta := Meta{
		Kind:          v.kind,
		Minecraft:     in.Minecraft,
		LoaderVersion: in.LoaderVersion,
		VersionID:     v.targetID,
		InstalledAt:   time.Now().Unix(),
	}
	if err := WriteMeta(in.Layout, meta); err != nil {
		return "", err
	}
	return v.targetID, nil
}

func (in *Installer) findGeneratedVersion(v forgeVariant) string {
	if v.kind == NeoForge {
		return findNeoForgeVersion(in.Layout, in.Minecraft, in.LoaderVersion, v.targetID)
	}
	return findForgeVersion(in.Layout, in.Minecraft)
}

func readForgeProfile(path string) (*forgeInstallProfile, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	var profile forgeInstallProfile
	for _, file := range reader.File {
		if file.Name != "install_profile.json" {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return nil, err
		}
		err = json.NewDecoder(rc).Decode(&profile)
		rc.Close()
		if err != nil {
			return nil, err
		}
		return &profile, nil
	}
	return nil, fmt.Errorf("install_profile.json not found")
}

func readForgeVersionLibraries(path string) ([]forgeLibrary, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	for _, file := range reader.File {
		if file.Name != "version.json" {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return nil, err
		}
		var payload struct {
			Libraries []forgeLibrary `json:"libraries"`
		}
		err = json.NewDecoder(rc).Decode(&payload)
		rc.Close()
		if err != nil {
			return nil, err
		}
		return payload.Libraries, nil
	}
	return nil, nil
}

// prepareForgeLibraries downloads every artifact listed in install_profile.json
// before the installer starts. The installer only retries a failed mirror URL
// once and then falls back to the (often unreachable) official host, so doing
// the transfers here gives each file several retries and a proper timeout. The
// installer is then run with --offline and only validates the local files.
func (in *Installer) prepareForgeLibraries(
	ctx context.Context,
	libs []forgeLibrary,
	mirror string,
	bases []string,
	label string,
) error {
	unique := make([]forgeLibrary, 0, len(libs))
	seen := make(map[string]bool, len(libs))
	for _, lib := range libs {
		rel := lib.Downloads.Artifact.Path
		if rel == "" {
			rel = mojang.MavenPath(lib.Name)
		}
		if rel == "" {
			continue
		}
		rel = filepath.ToSlash(rel)
		if seen[rel] {
			continue
		}
		seen[rel] = true
		lib.Downloads.Artifact.Path = rel
		unique = append(unique, lib)
	}
	if len(unique) == 0 {
		return nil
	}

	for i, lib := range unique {
		artifact := lib.Downloads.Artifact
		dest, err := util.SafeJoin(in.Layout.Libraries, artifact.Path)
		if err != nil {
			return fmt.Errorf("%s library %s: %w", label, lib.Name, err)
		}
		if util.VerifyFile(dest, artifact.SHA1, artifact.Size) {
			continue
		}
		in.emit(install.Event{
			Type:   install.EventStage,
			Stage:  "Preparing " + label + " libraries",
			Detail: fmt.Sprintf("%d/%d %s", i+1, len(unique), filepath.Base(artifact.Path)),
		})

		var lastErr error
		for _, url := range forgeLibraryURLs(lib, mirror, bases) {
			for attempt := 0; attempt < 2; attempt++ {
				fileCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
				lastErr = util.DownloadFile(fileCtx, in.client(), util.DownloadSpec{
					URL:  url,
					Dest: dest,
					SHA1: artifact.SHA1,
					Size: artifact.Size,
				}, nil)
				cancel()
				if lastErr == nil {
					break
				}
				if ctx.Err() != nil {
					return ctx.Err()
				}
			}
			if lastErr == nil {
				break
			}
		}
		if lastErr != nil {
			return fmt.Errorf("prepare %s library %s: %w", label, lib.Name, lastErr)
		}
	}
	return nil
}

func forgeLibraryURLs(lib forgeLibrary, mirror string, bases []string) []string {
	artifact := lib.Downloads.Artifact
	url := strings.TrimSpace(artifact.URL)
	if url == "" {
		base := strings.TrimSpace(lib.URL)
		if base == "" && len(bases) > 0 {
			base = bases[0]
		}
		if base == "" {
			base = forgeMavenBase + "/"
		}
		url = strings.TrimRight(base, "/") + "/" + strings.TrimLeft(artifact.Path, "/")
	}
	urls := []string{url}
	if mirror != "" {
		for _, base := range bases {
			if strings.HasPrefix(url, base) {
				urls = []string{mirror + "/" + strings.TrimPrefix(url, base), url}
				break
			}
		}
	}
	out := make([]string, 0, len(urls))
	seen := map[string]bool{}
	for _, u := range urls {
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		out = append(out, u)
	}
	return out
}

func ensureLauncherProfiles(layout paths.Layout) error {
	path := filepath.Join(layout.Root, "launcher_profiles.json")
	data, err := os.ReadFile(path)
	if err == nil && len(strings.TrimSpace(string(data))) > 0 {
		var probe map[string]any
		if json.Unmarshal(data, &probe) == nil {
			return nil
		}
		// Keep a corrupt file around instead of silently dropping profiles.
		_ = os.Rename(path, path+".bak")
	}
	return util.WriteJSONAtomic(path, map[string]any{
		"profiles": map[string]any{},
		"settings": map[string]any{},
		"version":  3,
	})
}

var forgeDirRE = regexp.MustCompile(`^(.+)-forge-(.+)$`)

func findForgeVersion(layout paths.Layout, minecraft string) string {
	entries, err := os.ReadDir(layout.Versions)
	if err != nil {
		return ""
	}
	best := ""
	var bestTime time.Time
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		m := forgeDirRE.FindStringSubmatch(entry.Name())
		if m == nil || m[1] != minecraft {
			continue
		}
		if _, err := os.Stat(layout.VersionJSON(entry.Name())); err != nil {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if best == "" || info.ModTime().After(bestTime) {
			best = entry.Name()
			bestTime = info.ModTime()
		}
	}
	return best
}

func findNeoForgeVersion(layout paths.Layout, minecraft, loaderVersion, targetID string) string {
	for _, id := range []string{
		targetID,
		"neoforge-" + loaderVersion,
		minecraft + "-forge-" + loaderVersion,
	} {
		if _, err := os.Stat(layout.VersionJSON(id)); err == nil {
			return id
		}
	}
	entries, err := os.ReadDir(layout.Versions)
	if err != nil {
		return ""
	}
	best := ""
	var bestTime time.Time
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "neoforge-") && !strings.HasPrefix(name, minecraft+"-neoforge-") {
			continue
		}
		if _, err := os.Stat(layout.VersionJSON(name)); err != nil {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if best == "" || info.ModTime().After(bestTime) {
			best = name
			bestTime = info.ModTime()
		}
	}
	return best
}

// adoptVersion renames the installer's version directory to the launcher's
// stable loader ID. NeoForge 1.20.1 reuses Forge's "1.20.1-forge-<version>"
// name, which would otherwise collide with a real Forge installation.
func adoptVersion(layout paths.Layout, oldID, newID string) error {
	if oldID == newID {
		return nil
	}
	oldDir := layout.VersionDir(oldID)
	newDir := layout.VersionDir(newID)
	if _, err := os.Stat(newDir); err == nil {
		if err := os.RemoveAll(newDir); err != nil {
			return err
		}
	}
	if err := os.Rename(oldDir, newDir); err != nil {
		return err
	}
	oldJSON := filepath.Join(newDir, oldID+".json")
	newJSON := layout.VersionJSON(newID)
	if oldJSON != newJSON {
		if err := os.Rename(oldJSON, newJSON); err != nil {
			return err
		}
	}
	var raw map[string]any
	if err := util.ReadJSON(newJSON, &raw); err != nil {
		return err
	}
	raw["id"] = newID
	return util.WriteJSONAtomic(newJSON, raw)
}

func normalizeVersionID(layout paths.Layout, versionID string) error {
	var version map[string]any
	path := layout.VersionJSON(versionID)
	if err := util.ReadJSON(path, &version); err != nil {
		return err
	}
	changed := false
	if id, _ := version["id"].(string); id != versionID {
		version["id"] = versionID
		changed = true
	}
	if _, ok := version["type"]; !ok {
		version["type"] = "release"
		changed = true
	}
	if !changed {
		return nil
	}
	return util.WriteJSONAtomic(path, version)
}

// rewriteForgeVersionMirror points the generated version JSON at the configured
// Maven mirror. The installer only knows about install_profile libraries;
// finish() also installs the generated version's libraries and would otherwise
// fall back to the official host.
func rewriteForgeVersionMirror(layout paths.Layout, versionID, mirror string, bases []string) error {
	if mirror == "" {
		return nil
	}
	path := layout.VersionJSON(versionID)
	var raw map[string]any
	if err := util.ReadJSON(path, &raw); err != nil {
		return err
	}
	libraries, ok := raw["libraries"].([]any)
	if !ok {
		return nil
	}
	changed := false
	rewrite := func(value any) any {
		url, ok := value.(string)
		if !ok {
			return value
		}
		out := url
		for _, base := range bases {
			out = rewriteMavenURL(out, base, mirror)
		}
		if out != url {
			changed = true
		}
		return out
	}
	for _, item := range libraries {
		lib, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if url, ok := lib["url"]; ok {
			lib["url"] = rewrite(url)
		}
		downloads, ok := lib["downloads"].(map[string]any)
		if !ok {
			continue
		}
		if artifact, ok := downloads["artifact"].(map[string]any); ok {
			if url, ok := artifact["url"]; ok {
				artifact["url"] = rewrite(url)
			}
		}
		if classifiers, ok := downloads["classifiers"].(map[string]any); ok {
			for _, value := range classifiers {
				if info, ok := value.(map[string]any); ok {
					if url, ok := info["url"]; ok {
						info["url"] = rewrite(url)
					}
				}
			}
		}
	}
	if !changed {
		return nil
	}
	return util.WriteJSONAtomic(path, raw)
}

var sha1RE = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)

// forgeMirror lets users route Forge Maven traffic through a mirror when the
// official host is slow or blocked. The same URL is handed to the Forge
// installer so its library and processor downloads use the mirror too.
func forgeMirror() string {
	if override := strings.TrimRight(strings.TrimSpace(os.Getenv("MCLAUNCHER_FORGE_MIRROR")), "/"); override != "" {
		return override
	}
	return strings.TrimRight(mirror.MavenBase(), "/")
}

// forgeMavenURL resolves a repository-relative path against the configured
// mirror, or the official Forge Maven when no mirror is set.
func forgeMavenURL(rel string) string {
	if mirror := forgeMirror(); mirror != "" {
		return mirror + "/" + strings.TrimLeft(rel, "/")
	}
	return forgeMavenBase + "/" + strings.TrimLeft(rel, "/")
}

func neoforgeMirror() string {
	if override := strings.TrimRight(strings.TrimSpace(os.Getenv("MCLAUNCHER_NEOFORGE_MIRROR")), "/"); override != "" {
		return override
	}
	return strings.TrimRight(mirror.MavenBase(), "/")
}

func neoforgeMavenURL(rel string) string {
	if mirror := neoforgeMirror(); mirror != "" {
		return mirror + "/" + strings.TrimLeft(rel, "/")
	}
	return neoforgeMavenBase + "/" + strings.TrimLeft(rel, "/")
}

func fetchOptionalSHA1(ctx context.Context, client *http.Client, url string) string {
	// The checksum is a best-effort integrity hint; never let a slow mirror
	// block the installation before the installer has even started.
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return ""
	}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 256))
	if err != nil {
		return ""
	}
	sum := strings.TrimSpace(string(data))
	if sha1RE.MatchString(sum) {
		return sum
	}
	return ""
}

func runJava(ctx context.Context, javaPath string, args []string, dir string, onLine func(string)) error {
	cmd := exec.CommandContext(ctx, javaPath, args...)
	cmd.Dir = dir
	hideWindow(cmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	var wg sync.WaitGroup
	pumpLines := func(r io.Reader) {
		defer wg.Done()
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 16*1024), 256*1024)
		for scanner.Scan() {
			if onLine != nil {
				onLine(strings.TrimRight(scanner.Text(), "\r"))
			}
		}
	}
	wg.Add(2)
	go pumpLines(stdout)
	go pumpLines(stderr)
	wg.Wait()
	err = cmd.Wait()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}
