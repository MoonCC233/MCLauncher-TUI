package mojang

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

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
	err := util.FetchJSON(ctx, client, ManifestURL, &raw)
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

func LoadVersionJSON(layout paths.Layout, id string) (*VersionJSON, error) {
	v, err := loadVersionJSONFile(layout, id)
	if err != nil {
		return nil, err
	}
	if v.MainClass == "" || len(v.Libraries) == 0 {
		return nil, fmt.Errorf("version metadata for %s is incomplete", id)
	}
	return v, nil
}

func loadVersionJSONFile(layout paths.Layout, id string) (*VersionJSON, error) {
	var v VersionJSON
	if err := util.ReadJSON(layout.VersionJSON(id), &v); err != nil {
		return nil, err
	}
	if v.ID == "" {
		v.ID = id
	}
	return &v, nil
}

// FetchVersionJSON loads local metadata when possible, otherwise downloads it
// from the URL advertised by the manifest.
func FetchVersionJSON(ctx context.Context, client *http.Client, layout paths.Layout, manifest *Manifest, id string, force bool) (*VersionJSON, error) {
	local, _ := LoadVersionJSON(layout, id)
	if local != nil && !force {
		return local, nil
	}
	if manifest == nil {
		var err error
		manifest, err = FetchManifest(ctx, client, layout, false)
		if err != nil {
			if local != nil {
				return local, nil
			}
			return nil, err
		}
	}
	summary := manifest.Find(id)
	if summary == nil {
		if local != nil {
			return local, nil
		}
		return nil, fmt.Errorf("version %q is not in the Mojang manifest", id)
	}
	var v VersionJSON
	if err := util.FetchJSON(ctx, client, summary.URL, &v); err != nil {
		if local != nil {
			return local, nil
		}
		return nil, fmt.Errorf("fetch version %s: %w", id, err)
	}
	if v.MainClass == "" {
		if local != nil {
			return local, nil
		}
		return nil, fmt.Errorf("version %s metadata is invalid", id)
	}
	if v.ID == "" {
		v.ID = id
	}
	if err := util.WriteJSONAtomic(layout.VersionJSON(id), &v); err != nil {
		return nil, err
	}
	return &v, nil
}

func ClientJarPath(layout paths.Layout, id string) string {
	return filepath.Join(layout.VersionDir(id), id+".jar")
}
