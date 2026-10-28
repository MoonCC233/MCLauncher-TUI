package mojang

import (
	"fmt"

	"github.com/MoonCC233/MCLauncher-TUI/internal/paths"
)

// LoadResolvedVersionJSON loads a version and merges every inherited parent.
// The returned jarID points at the version that owns the client jar (usually
// the vanilla parent of a mod-loader profile).
func LoadResolvedVersionJSON(layout paths.Layout, id string) (*VersionJSON, string, error) {
	chain := make([]*VersionJSON, 0, 3)
	seen := make(map[string]bool)
	current := id
	jarID := id
	for {
		if current == "" {
			break
		}
		if seen[current] {
			return nil, "", fmt.Errorf("version inheritance cycle at %s", current)
		}
		seen[current] = true
		v, err := loadVersionJSONFile(layout, current)
		if err != nil {
			return nil, "", err
		}
		chain = append(chain, v)
		jarID = v.EffectiveID(current)
		if v.InheritsFrom == "" {
			break
		}
		current = v.InheritsFrom
	}

	merged := chain[len(chain)-1]
	for i := len(chain) - 2; i >= 0; i-- {
		merged = MergeVersionJSON(merged, chain[i])
	}
	if merged.MainClass == "" {
		return nil, "", fmt.Errorf("version metadata for %s is incomplete", id)
	}
	if len(merged.Libraries) == 0 {
		return nil, "", fmt.Errorf("version metadata for %s has no libraries", id)
	}
	return merged, jarID, nil
}

// ClientJarID walks the inheritance chain to find the version that owns the
// client jar.
func ClientJarID(layout paths.Layout, id string) (string, error) {
	seen := make(map[string]bool)
	current := id
	for current != "" {
		if seen[current] {
			return "", fmt.Errorf("version inheritance cycle at %s", current)
		}
		seen[current] = true
		v, err := loadVersionJSONFile(layout, current)
		if err != nil {
			return "", err
		}
		if v.InheritsFrom == "" {
			return v.EffectiveID(current), nil
		}
		current = v.InheritsFrom
	}
	return id, nil
}

// ClientJarFor resolves the client jar path and falls back to the version's own
// directory when metadata cannot be read.
func ClientJarFor(layout paths.Layout, id string) string {
	jarID, err := ClientJarID(layout, id)
	if err != nil || jarID == "" {
		jarID = id
	}
	return ClientJarPath(layout, jarID)
}

// MergeVersionJSON overlays a child profile on top of its parent. Scalar fields
// fall back to the parent and libraries are deduplicated by Maven coordinate.
func MergeVersionJSON(parent, child *VersionJSON) *VersionJSON {
	if parent == nil {
		return child
	}
	if child == nil {
		return parent
	}
	out := *parent
	if child.ID != "" {
		out.ID = child.ID
	}
	out.InheritsFrom = ""
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
	if child.Downloads.Client.URL != "" || child.Downloads.Client.SHA1 != "" || child.Downloads.Client.Size > 0 {
		out.Downloads.Client = child.Downloads.Client
	}
	if child.Downloads.Server.URL != "" || child.Downloads.Server.SHA1 != "" || child.Downloads.Server.Size > 0 {
		out.Downloads.Server = child.Downloads.Server
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
	out.Libraries = mergeLibraries(parent.Libraries, child.Libraries)
	out.Arguments = mergeArguments(parent.Arguments, child.Arguments)
	return &out
}

func mergeLibraries(parent, child []Library) []Library {
	merged := make([]Library, 0, len(parent)+len(child))
	index := make(map[string]int, len(parent)+len(child))
	for _, lib := range parent {
		if lib.Name != "" {
			index[lib.Name] = len(merged)
		}
		merged = append(merged, lib)
	}
	for _, lib := range child {
		if lib.Name != "" {
			if at, ok := index[lib.Name]; ok {
				merged[at] = lib
				continue
			}
			index[lib.Name] = len(merged)
		}
		merged = append(merged, lib)
	}
	return merged
}

func mergeArguments(parent, child *Arguments) *Arguments {
	if parent == nil {
		return child
	}
	if child == nil {
		return parent
	}
	return &Arguments{
		Game: append(append([]Argument{}, parent.Game...), child.Game...),
		JVM:  append(append([]Argument{}, parent.JVM...), child.JVM...),
	}
}
