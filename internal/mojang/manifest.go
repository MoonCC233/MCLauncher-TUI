package mojang

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/MoonCC233/MCLauncher-TUI/internal/mirror"
	"github.com/MoonCC233/MCLauncher-TUI/internal/paths"
	"github.com/MoonCC233/MCLauncher-TUI/internal/util"
)

const ManifestURL = "https://piston-meta.mojang.com/mc/game/version_manifest_v2.json"

type VersionSummary struct {
	ID          string
	Type        string
	URL         string
	ReleaseTime string
	SHA1        string
	Latest      bool
}

type Manifest struct {
	Versions       []VersionSummary
	LatestRelease  string
	LatestSnapshot string
	Source         string // network or cache
	Warning        string
}

type rawManifest struct {
	Latest   rawLatest    `json:"latest"`
	Versions []rawVersion `json:"versions"`
}

type rawLatest struct {
	Release  string `json:"release"`
	Snapshot string `json:"snapshot"`
}

type rawVersion struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	URL         string `json:"url"`
	ReleaseTime string `json:"releaseTime"`
	SHA1        string `json:"sha1"`
}

func parseManifest(raw rawManifest, source string) *Manifest {
	m := &Manifest{
		LatestRelease:  raw.Latest.Release,
		LatestSnapshot: raw.Latest.Snapshot,
		Source:         source,
		Versions:       make([]VersionSummary, 0, len(raw.Versions)),
	}
	for _, v := range raw.Versions {
		if v.ID == "" || v.URL == "" {
			continue
		}
		m.Versions = append(m.Versions, VersionSummary{
			ID:          v.ID,
			Type:        v.Type,
			URL:         v.URL,
			ReleaseTime: v.ReleaseTime,
			SHA1:        v.SHA1,
			Latest:      v.ID == raw.Latest.Release,
		})
	}
	return m
}

func loadCachedManifest(path string) (*Manifest, error) {
	var raw rawManifest
	if err := util.ReadJSON(path, &raw); err != nil {
		return nil, err
	}
	return parseManifest(raw, "cache"), nil
}

// FetchManifest prefers the network, then falls back to a stale cache.
func FetchManifest(ctx context.Context, client *http.Client, layout paths.Layout, force bool) (*Manifest, error) {
	cachePath := layout.ManifestCache()
	if !force {
		if st, err := os.Stat(cachePath); err == nil && time.Since(st.ModTime()) < time.Hour {
			if m, err := loadCachedManifest(cachePath); err == nil && len(m.Versions) > 0 {
				return m, nil
			}
		}
	}
	var raw rawManifest
	err := fetchManifestCandidates(ctx, client, &raw)
	if err == nil && len(raw.Versions) > 0 {
		m := parseManifest(raw, "network")
		if saveErr := util.WriteJSONAtomic(cachePath, raw); saveErr != nil {
			m.Warning = fmt.Sprintf("could not cache manifest: %v", saveErr)
		}
		return m, nil
	}
	if err == nil {
		err = fmt.Errorf("version manifest was empty")
	}
	if cached, cacheErr := loadCachedManifest(cachePath); cacheErr == nil && len(cached.Versions) > 0 {
		cached.Warning = fmt.Sprintf("offline: %v", err)
		return cached, nil
	}
	return nil, fmt.Errorf("could not load version manifest: %w", err)
}

// fetchManifestCandidates reads the manifest from the mirror first when one is
// selected, then from Mojang.
func fetchManifestCandidates(ctx context.Context, client *http.Client, target any) error {
	return fetchJSONCandidatesManifest(ctx, client, mirror.Candidates(ManifestURL), target)
}

// fetchJSONCandidatesManifest reads the first candidate that answers with a
// non-empty manifest.
func fetchJSONCandidatesManifest(ctx context.Context, client *http.Client, urls []string, target any) error {
	var lastErr error
	for _, url := range urls {
		var candidate rawManifest
		if err := util.FetchJSON(ctx, client, url, &candidate); err != nil {
			lastErr = err
			continue
		}
		// A mirror that answers with an empty list must not win over the official
		// manifest, which the caller would then treat as "no versions".
		if len(candidate.Versions) == 0 {
			lastErr = fmt.Errorf("%s: empty version manifest", url)
			continue
		}
		if raw, ok := target.(*rawManifest); ok {
			*raw = candidate
			return nil
		}
		if m, ok := target.(*Manifest); ok {
			parsed := parseManifest(candidate, "network")
			*m = *parsed
			return nil
		}
		return fmt.Errorf("unsupported manifest target")
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no version manifest source is available")
	}
	return lastErr
}

func (m *Manifest) Find(id string) *VersionSummary {
	if m == nil {
		return nil
	}
	for i := range m.Versions {
		if m.Versions[i].ID == id {
			return &m.Versions[i]
		}
	}
	return nil
}

func ClientJarPath(layout paths.Layout, id string) string {
	return filepath.Join(layout.VersionDir(id), id+".jar")
}
