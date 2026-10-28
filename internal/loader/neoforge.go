package loader

import (
	"archive/zip"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/MoonCC233/MCLauncher-TUI/internal/mojang"
	"github.com/MoonCC233/MCLauncher-TUI/internal/paths"
	"github.com/MoonCC233/MCLauncher-TUI/internal/util"
)

const (
	neoForgeMaven = "https://maven.neoforged.net/releases"
	maxProfile    = 10 << 20
)

type mavenMetadata struct {
	Versioning struct {
		Versions []string `xml:"versions>version"`
		Latest   string   `xml:"latest"`
		Release  string   `xml:"release"`
	} `xml:"versioning"`
}

type neoForgeArtifact struct {
	Group   string
	Version string
	URL     string
	File    string
}

type installProfile struct {
	Version   string `json:"version"`
	Minecraft string `json:"minecraft"`
	Install   struct {
		Version string `json:"version"`
	} `json:"install"`
}

func neoForgeVersions(ctx context.Context, client *http.Client, gameVersion string) ([]Version, error) {
	artifact, legacy := neoForgeBase(gameVersion)
	metadata, err := fetchMavenMetadata(ctx, client, artifact.Group)
	if err != nil {
		return nil, err
	}
	versions := filterNeoForgeVersions(metadata.Versioning.Versions, gameVersion, legacy)
	if len(versions) == 0 {
		return nil, fmt.Errorf("NeoForge has no builds for Minecraft %s", gameVersion)
	}
	return versions, nil
}

func fetchMavenMetadata(ctx context.Context, client *http.Client, group string) (*mavenMetadata, error) {
	endpoint := fmt.Sprintf("%s/%s/maven-metadata.xml", neoForgeMaven, group)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "mclauncher")
	req.Header.Set("Accept", "application/xml, text/xml")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch NeoForge versions: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch NeoForge versions: HTTP %d", resp.StatusCode)
	}
	var metadata mavenMetadata
	if err := xml.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&metadata); err != nil {
		return nil, fmt.Errorf("parse NeoForge version list: %w", err)
	}
	if len(metadata.Versioning.Versions) == 0 {
		return nil, fmt.Errorf("NeoForge version list is empty")
	}
	return &metadata, nil
}

func filterNeoForgeVersions(all []string, gameVersion string, legacy bool) []Version {
	prefix, _ := neoForgePrefix(gameVersion)
	versions := make([]Version, 0, len(all))
	for _, raw := range all {
		version := strings.TrimSpace(raw)
		if version == "" || !validProfileID(version) {
			continue
		}
		if legacy {
			if !strings.HasPrefix(version, gameVersion+"-") {
				continue
			}
		} else if !strings.HasPrefix(version, prefix) {
			continue
		}
		lower := strings.ToLower(version)
		stable := !strings.Contains(lower, "beta") && !strings.Contains(lower, "alpha")
		versions = append(versions, Version{Version: version, Stable: stable})
	}
	sort.Slice(versions, func(i, j int) bool {
		return compareVersions(versions[i].Version, versions[j].Version) > 0
	})
	return versions
}

func compareVersions(a, b string) int {
	pa := strings.FieldsFunc(a, func(r rune) bool { return r == '.' || r == '-' || r == '+' })
	pb := strings.FieldsFunc(b, func(r rune) bool { return r == '.' || r == '-' || r == '+' })
	for i := 0; i < len(pa) && i < len(pb); i++ {
		na, errA := strconv.Atoi(pa[i])
		nb, errB := strconv.Atoi(pb[i])
		if errA == nil && errB == nil {
			if na != nb {
				if na > nb {
					return 1
				}
				return -1
			}
			continue
		}
		if cmp := strings.Compare(pa[i], pb[i]); cmp != 0 {
			return cmp
		}
	}
	switch {
	case len(pa) > len(pb):
		return 1
	case len(pa) < len(pb):
		return -1
	}
	return 0
}

func neoForgeBase(gameVersion string) (neoForgeArtifact, bool) {
	prefix, legacy := neoForgePrefix(gameVersion)
	group := "net/neoforged/neoforge"
	if legacy {
		group = "net/neoforged/forge"
	}
	return neoForgeArtifact{Group: group, URL: prefix}, legacy
}

func neoForgePrefix(gameVersion string) (string, bool) {
	parts := strings.Split(strings.TrimSpace(gameVersion), ".")
	if len(parts) < 2 {
		return "", false
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return "", false
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", false
	}
	if major == 1 {
		patch := 0
		if len(parts) > 2 {
			patch, _ = strconv.Atoi(parts[2])
		}
		if minor < 20 || (minor == 20 && patch <= 1) {
			return gameVersion + "-", true
		}
		return fmt.Sprintf("%d.%d.", minor, patch), false
	}
	return fmt.Sprintf("%d.%d.", major, minor), false
}

func neoForgeArtifactFor(gameVersion, loaderVersion string) (neoForgeArtifact, error) {
	artifact, legacy := neoForgeBase(gameVersion)
	version := strings.TrimSpace(loaderVersion)
	if version == "" {
		return neoForgeArtifact{}, fmt.Errorf("loader version is required")
	}
	if legacy && !strings.HasPrefix(version, gameVersion+"-") {
		version = gameVersion + "-" + version
	}
	if !validProfileID(version) {
		return neoForgeArtifact{}, fmt.Errorf("invalid NeoForge version %q", version)
	}
	artifact.Version = version
	artifact.URL = fmt.Sprintf("%s/%s/%s/%s-%s-installer.jar",
		neoForgeMaven, artifact.Group, version, filepath.Base(artifact.Group), version)
	artifact.File = fmt.Sprintf("%s-%s-installer.jar", filepath.Base(artifact.Group), version)
	return artifact, nil
}

func installNeoForge(ctx context.Context, opts Options) (string, error) {
	artifact, err := neoForgeArtifactFor(opts.GameVersion, opts.LoaderVersion)
	if err != nil {
		return "", err
	}
	if opts.JavaPath == "" {
		return "", fmt.Errorf("no Java runtime selected for the NeoForge installer")
	}
	if err := util.EnsureDir(opts.Layout.LoaderCache()); err != nil {
		return "", err
	}
	installerPath := filepath.Join(opts.Layout.LoaderCache(), artifact.File)
	if info, err := os.Stat(installerPath); err != nil || info.Size() == 0 {
		opts.stage("Downloading NeoForge installer", artifact.Version)
		if err := util.DownloadFile(ctx, opts.client(), util.DownloadSpec{
			URL:  artifact.URL,
			Dest: installerPath,
		}, nil); err != nil {
			return "", fmt.Errorf("download NeoForge installer: %w", err)
		}
	}
	profile, _ := readInstallProfile(installerPath)
	opts.stage("Running NeoForge installer", artifact.Version)
	if err := prepareLauncherProfiles(opts.Layout); err != nil {
		return "", err
	}
	if err := runInstaller(ctx, opts, installerPath); err != nil {
		return "", err
	}
	versionID := ""
	for _, candidate := range []string{profile.Version, profile.Install.Version} {
		candidate = strings.TrimSpace(candidate)
		if validProfileID(candidate) && profileInheritsFrom(opts.Layout, candidate, opts.GameVersion) {
			versionID = candidate
			break
		}
	}
	if versionID == "" {
		versionID = findInstalledProfile(opts.Layout, opts.GameVersion, opts.LoaderVersion)
	}
	if versionID == "" {
		return "", fmt.Errorf("NeoForge installer finished but no version profile was found")
	}
	if _, _, err := mojang.LoadResolvedVersionJSON(opts.Layout, versionID); err != nil {
		return "", fmt.Errorf("load NeoForge profile %s: %w", versionID, err)
	}
	opts.log("installed NeoForge profile " + versionID)
	return versionID, nil
}

func prepareLauncherProfiles(layout paths.Layout) error {
	path := filepath.Join(layout.Root, "launcher_profiles.json")
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	profile := map[string]any{
		"profiles": map[string]any{},
		"settings": map[string]any{},
		"version":  3,
	}
	return util.WriteJSONAtomic(path, profile)
}

func runInstaller(ctx context.Context, opts Options, installerPath string) error {
	return runJavaInstaller(ctx, opts.JavaPath, installerPath, opts.Layout.Root, opts.log)
}

func readInstallProfile(path string) (installProfile, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return installProfile{}, err
	}
	defer reader.Close()
	for _, file := range reader.File {
		if filepath.Base(file.Name) != "install_profile.json" {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return installProfile{}, err
		}
		data, err := io.ReadAll(io.LimitReader(rc, maxProfile))
		_ = rc.Close()
		if err != nil {
			return installProfile{}, err
		}
		var profile installProfile
		if err := json.Unmarshal(data, &profile); err != nil {
			return installProfile{}, fmt.Errorf("parse install_profile.json: %w", err)
		}
		return profile, nil
	}
	return installProfile{}, fmt.Errorf("installer has no install_profile.json")
}

func profileInheritsFrom(layout paths.Layout, id, gameVersion string) bool {
	var profile struct {
		InheritsFrom string `json:"inheritsFrom"`
	}
	if err := util.ReadJSON(layout.VersionJSON(id), &profile); err != nil {
		return false
	}
	return strings.EqualFold(profile.InheritsFrom, gameVersion)
}

func findInstalledProfile(layout paths.Layout, gameVersion, loaderVersion string) string {
	entries, err := os.ReadDir(layout.Versions)
	if err != nil {
		return ""
	}
	suffix := loaderVersion
	if at := strings.Index(suffix, "-"); at >= 0 {
		suffix = suffix[at+1:]
	}
	type candidate struct {
		id      string
		modTime int64
	}
	var found []candidate
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		id := entry.Name()
		if !strings.Contains(id, loaderVersion) && !strings.Contains(id, suffix) {
			continue
		}
		var profile struct {
			InheritsFrom string `json:"inheritsFrom"`
		}
		if err := util.ReadJSON(layout.VersionJSON(id), &profile); err != nil {
			continue
		}
		if !strings.EqualFold(profile.InheritsFrom, gameVersion) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		found = append(found, candidate{id: id, modTime: info.ModTime().UnixNano()})
	}
	sort.Slice(found, func(i, j int) bool { return found[i].modTime > found[j].modTime })
	if len(found) > 0 {
		return found[0].id
	}
	return ""
}
