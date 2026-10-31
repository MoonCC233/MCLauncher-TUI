package catalog

import (
	"context"
	"net/url"
	"strings"
)

// maxDependencyDepth bounds recursive dependency resolution. Real mods rarely
// nest more than two levels; the limit keeps a malformed or cyclic catalog
// answer from downloading forever.
const maxDependencyDepth = 4

// ResolveRequired returns the versions that the root version needs at install
// time. Both catalogs are supported: Modrinth dependencies carry a project and
// sometimes a version id, while CurseForge files only point at a project, so
// the newest file matching the game version and loader is used.
//
// A missing optional dependency must never fail an install, so individual
// resolution errors are skipped and whatever could be resolved is returned.
func (c *Client) ResolveRequired(ctx context.Context, source Source, root Version, gameVersion, loader string) ([]Version, error) {
	source = NormalizeSource(source)
	seen := make(map[string]bool)
	if root.ProjectID != "" {
		seen[root.ProjectID] = true
	}
	queue := []Version{root}
	var resolved []Version

	for depth := 0; depth < maxDependencyDepth && len(queue) > 0; depth++ {
		var next []Version
		for _, version := range queue {
			for _, dependency := range version.Dependencies {
				if dependency.Type != "" && dependency.Type != "required" {
					continue
				}
				projectID := strings.TrimSpace(dependency.ProjectID)
				if projectID != "" && seen[projectID] {
					continue
				}

				dependencyVersion, err := c.resolveDependency(ctx, source, dependency, gameVersion, loader)
				if err != nil {
					continue
				}
				if dependencyVersion.ProjectID == "" {
					dependencyVersion.ProjectID = projectID
				}
				if dependencyVersion.ProjectID != "" {
					if seen[dependencyVersion.ProjectID] {
						continue
					}
					seen[dependencyVersion.ProjectID] = true
				}
				if _, ok := dependencyVersion.PrimaryFile(); !ok {
					continue
				}
				resolved = append(resolved, dependencyVersion)
				next = append(next, dependencyVersion)
			}
		}
		queue = next
	}
	return resolved, nil
}

// resolveDependency picks one concrete version for a dependency. A pinned
// Modrinth version id wins; everything else falls back to the newest version
// compatible with the instance.
func (c *Client) resolveDependency(ctx context.Context, source Source, dependency Dependency, gameVersion, loader string) (Version, error) {
	if source == SourceModrinth && strings.TrimSpace(dependency.VersionID) != "" {
		var version Version
		path := "/version/" + url.PathEscape(strings.TrimSpace(dependency.VersionID))
		if err := c.get(ctx, path, &version); err == nil {
			if _, ok := version.PrimaryFile(); ok {
				return version, nil
			}
		}
	}
	versions, err := c.VersionsForSource(ctx, source, dependency.ProjectID, gameVersion, loader)
	if err != nil {
		return Version{}, err
	}
	for _, version := range versions {
		if _, ok := version.PrimaryFile(); ok {
			return version, nil
		}
	}
	return Version{}, nil
}
