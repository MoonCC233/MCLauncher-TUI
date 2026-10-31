package catalog

import "context"

// Source identifies a content catalog the browser can search.
type Source string

const (
	// SourceModrinth is the official Modrinth catalog.
	SourceModrinth Source = "modrinth"
	// SourceCurseForge is the CurseForge catalog, read through a keyless
	// read-only mirror.
	SourceCurseForge Source = "curseforge"
)

// Sources lists the selectable catalogs, in picker order.
var Sources = []Source{SourceModrinth, SourceCurseForge}

// Sort is the result ordering offered by the download center.
type Sort string

const (
	// SortRelevance keeps the catalog's own relevance ordering.
	SortRelevance Sort = "relevance"
	// SortDownloads orders by total downloads.
	SortDownloads Sort = "downloads"
	// SortNewest orders by the newest release.
	SortNewest Sort = "newest"
	// SortUpdated orders by the last update.
	SortUpdated Sort = "updated"
)

// Sorts lists the selectable orderings, in picker order.
var Sorts = []Sort{SortRelevance, SortDownloads, SortNewest, SortUpdated}

// NormalizeSort falls back to relevance for empty or unknown values.
func NormalizeSort(sort Sort) Sort {
	for _, candidate := range Sorts {
		if candidate == sort {
			return sort
		}
	}
	return SortRelevance
}

// SearchOptions is one catalog query.
type SearchOptions struct {
	Source      Source
	ProjectType string
	Query       string
	GameVersion string
	Loader      string
	Sort        Sort
	Limit       int
	Offset      int
}

// Valid reports whether the source is one this package understands.
func (s Source) Valid() bool {
	for _, candidate := range Sources {
		if candidate == s {
			return true
		}
	}
	return false
}

// Label is the human-readable catalog name. Both names are proper nouns and are
// kept identical in every interface language.
func (s Source) Label() string {
	if s == SourceCurseForge {
		return "CurseForge"
	}
	return "Modrinth"
}

// NormalizeSource falls back to Modrinth for empty or unknown values.
func NormalizeSource(source Source) Source {
	if source.Valid() {
		return source
	}
	return SourceModrinth
}

// SearchSource searches one catalog with the catalog's default ordering.
func (c *Client) SearchSource(ctx context.Context, source Source, projectType, query, gameVersion, loader string, limit, offset int) (SearchResult, error) {
	return c.SearchCatalog(ctx, SearchOptions{
		Source:      source,
		ProjectType: projectType,
		Query:       query,
		GameVersion: gameVersion,
		Loader:      loader,
		Limit:       limit,
		Offset:      offset,
	})
}

// SearchCatalog runs one catalog query, including its ordering.
func (c *Client) SearchCatalog(ctx context.Context, options SearchOptions) (SearchResult, error) {
	options.Sort = NormalizeSort(options.Sort)
	if NormalizeSource(options.Source) == SourceCurseForge {
		return c.searchCurseForge(ctx, options.ProjectType, options.Query, options.GameVersion, options.Loader, options.Sort, options.Limit, options.Offset)
	}
	return c.SearchWithSort(ctx, options.ProjectType, options.Query, options.GameVersion, options.Loader, options.Sort, options.Limit, options.Offset)
}

// VersionsForSource lists the installable versions of one project in a catalog.
func (c *Client) VersionsForSource(ctx context.Context, source Source, projectID, gameVersion, loader string) ([]Version, error) {
	if NormalizeSource(source) == SourceCurseForge {
		return c.curseForgeVersions(ctx, projectID, gameVersion, loader)
	}
	return c.VersionsFor(ctx, projectID, gameVersion, loader)
}

// SourceForProject returns the catalog recorded on a search hit.
func SourceForProject(project Project) Source {
	if project.Source.Valid() {
		return project.Source
	}
	return SourceModrinth
}
