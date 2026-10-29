package mojang

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/MoonCC233/MCLauncher-TUI/internal/paths"
	"github.com/MoonCC233/MCLauncher-TUI/internal/util"
)

const maxInheritanceDepth = 16

// LoadVersionJSON loads a version and merges every parent listed through
// inheritsFrom. Loader profiles such as Fabric, Quilt, Forge and OptiFine only
// describe their additions; the game libraries and arguments come from the
// vanilla parent.
func LoadVersionJSON(layout paths.Layout, id string) (*VersionJSON, error) {
	v, err := loadVersionTree(layout, id, nil, 0)
	if err != nil {
		return nil, err
	}
	if v.MainClass == "" || len(v.Libraries) == 0 {
		return nil, fmt.Errorf("version metadata for %s is incomplete", id)
	}
	return v, nil
}

// FetchVersionJSON loads local metadata when possible, otherwise downloads the
// version and any missing parents from the URL advertised by the manifest.
// Loader versions are local-only, so their profiles are never replaced.
func FetchVersionJSON(ctx context.Context, client *http.Client, layout paths.Layout, manifest *Manifest, id string, force bool) (*VersionJSON, error) {
	if !force {
		if local, err := LoadVersionJSON(layout, id); err == nil {
			return local, nil
		}
	}
	if manifest == nil {
		var err error
		manifest, err = FetchManifest(ctx, client, layout, false)
		if err != nil {
			if local, loadErr := LoadVersionJSON(layout, id); loadErr == nil {
				return local, nil
			}
			return nil, err
		}
	}
	if err := fetchVersionTree(ctx, client, layout, manifest, id, force, nil); err != nil {
		if !force {
			if local, loadErr := LoadVersionJSON(layout, id); loadErr == nil {
				return local, nil
			}
		}
		return nil, err
	}
	return LoadVersionJSON(layout, id)
}

func loadVersionTree(layout paths.Layout, id string, seen map[string]bool, depth int) (*VersionJSON, error) {
	if depth > maxInheritanceDepth {
		return nil, fmt.Errorf("version inheritance for %s is too deep", id)
	}
	if seen == nil {
		seen = map[string]bool{}
	}
	key := strings.ToLower(id)
	if seen[key] {
		return nil, fmt.Errorf("version inheritance cycle at %s", id)
	}
	seen[key] = true

	local, err := loadRawVersion(layout, id)
	if err != nil {
		return nil, err
	}
	if local.InheritsFrom == "" {
		return local, nil
	}
	parent, err := loadVersionTree(layout, local.InheritsFrom, seen, depth+1)
	if err != nil {
		return nil, err
	}
	merged := MergeVersions(parent, local)
	if merged.ID == "" {
		merged.ID = id
	}
	// Keep the direct parent on the result so installers can repair the base
	// version before touching the loader files.
	merged.InheritsFrom = local.InheritsFrom
	if merged.JarID == "" {
		merged.JarID = parent.JarID
	}
	return merged, nil
}

func loadRawVersion(layout paths.Layout, id string) (*VersionJSON, error) {
	var v VersionJSON
	if err := util.ReadJSON(layout.VersionJSON(id), &v); err != nil {
		return nil, err
	}
	if v.ID == "" {
		v.ID = id
	}
	if v.JarID == "" && v.Downloads.Client.URL != "" {
		v.JarID = id
	}
	return &v, nil
}

// MergeVersions overlays child on top of parent. Libraries with the same
// coordinate are replaced in place, while all other child entries are appended
// after the parent libraries. Arguments are concatenated, parent first.
func MergeVersions(parent, child *VersionJSON) *VersionJSON {
	if parent == nil {
		if child == nil {
			return &VersionJSON{}
		}
		out := *child
		return &out
	}
	out := *parent
	if child == nil {
		return &out
	}

	if child.ID != "" {
		out.ID = child.ID
	}
	out.InheritsFrom = child.InheritsFrom
	if child.Type != "" {
		out.Type = child.Type
	}
	if child.MainClass != "" {
		out.MainClass = child.MainClass
	}
	if child.Assets != "" {
		out.Assets = child.Assets
	}
	if child.AssetIndex != nil {
		out.AssetIndex = child.AssetIndex
	}
	if child.JavaVersion != nil {
		out.JavaVersion = child.JavaVersion
	}
	if child.Logging != nil {
		out.Logging = child.Logging
	}
	if child.MinecraftArguments != "" {
		out.MinecraftArguments = child.MinecraftArguments
	}
	if child.Downloads.Client.URL != "" || child.Downloads.Client.Path != "" ||
		child.Downloads.Server.URL != "" || child.Downloads.Server.Path != "" {
		out.Downloads = child.Downloads
		if child.ID != "" {
			out.JarID = child.ID
		}
	}
	if child.JarID != "" {
		// OptiFine stores its patched client jar in its own version directory.
		out.JarID = child.JarID
	}

	out.Libraries = mergeLibraries(parent.Libraries, child.Libraries)
	out.Arguments = mergeArguments(parent.Arguments, child.Arguments)
	return &out
}

func mergeLibraries(parent, child []Library) []Library {
	out := make([]Library, 0, len(parent)+len(child))
	index := make(map[string]int, len(parent)+len(child))
	for _, lib := range parent {
		index[lib.Name] = len(out)
		out = append(out, lib)
	}
	for _, lib := range child {
		if i, ok := index[lib.Name]; ok {
			out[i] = lib
			continue
		}
		index[lib.Name] = len(out)
		out = append(out, lib)
	}
	return out
}

func mergeArguments(parent, child *Arguments) *Arguments {
	if parent == nil && child == nil {
		return nil
	}
	out := &Arguments{}
	if parent != nil {
		out.Game = append(out.Game, parent.Game...)
		out.JVM = append(out.JVM, parent.JVM...)
	}
	if child != nil {
		out.Game = append(out.Game, child.Game...)
		out.JVM = append(out.JVM, child.JVM...)
	}
	return out
}

// ClientJarID returns the version directory that owns the client jar. Loader
// profiles normally point at their vanilla parent.
func (v *VersionJSON) ClientJarID(fallback string) string {
	if v != nil && v.JarID != "" {
		return v.JarID
	}
	return fallback
}

// fetchVersionTree downloads a version and any missing parents from the Mojang
// manifest. Loader profiles are local-only, so a missing loader id with an
// existing local JSON is left untouched.
func fetchVersionTree(ctx context.Context, client *http.Client, layout paths.Layout, manifest *Manifest, id string, force bool, seen map[string]bool) error {
	if seen == nil {
		seen = map[string]bool{}
	}
	key := strings.ToLower(id)
	if seen[key] {
		return fmt.Errorf("version inheritance cycle at %s", id)
	}
	seen[key] = true

	rawPath := layout.VersionJSON(id)
	var local VersionJSON
	localErr := util.ReadJSON(rawPath, &local)
	if localErr == nil && !force {
		if local.InheritsFrom == "" {
			return nil
		}
		// Repair a missing parent even when the child JSON already exists.
		return fetchVersionTree(ctx, client, layout, manifest, local.InheritsFrom, false, seen)
	}

	summary := manifest.Find(id)
	if summary == nil {
		if localErr == nil {
			return nil
		}
		if manifest != nil {
			return fmt.Errorf("version %q is not in the Mojang manifest", id)
		}
		return fmt.Errorf("version %q is not installed", id)
	}
	var v VersionJSON
	if err := util.FetchJSON(ctx, client, summary.URL, &v); err != nil {
		if localErr == nil {
			return nil
		}
		return fmt.Errorf("fetch version %s: %w", id, err)
	}
	if v.MainClass == "" {
		if localErr == nil {
			return nil
		}
		return fmt.Errorf("version %s metadata is invalid", id)
	}
	if v.ID == "" {
		v.ID = id
	}
	if err := util.WriteJSONAtomic(rawPath, &v); err != nil {
		return err
	}
	if v.InheritsFrom != "" {
		return fetchVersionTree(ctx, client, layout, manifest, v.InheritsFrom, force, seen)
	}
	return nil
}
