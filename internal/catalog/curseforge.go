package catalog

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

const (
	// DefaultCurseForgeBaseURL is a read-only CurseForge API mirror. It serves
	// the public endpoints without an API key, so the launcher does not have to
	// ship one.
	DefaultCurseForgeBaseURL = "https://mod.mcimirror.top/curseforge/v1"

	curseForgeGameID = "432"
)

// CurseForge class ids used by the three content kinds.
const (
	curseForgeClassMod          = "6"
	curseForgeClassResourcePack = "12"
	curseForgeClassShader       = "6552"
)

// CurseForge mod loader ids.
const (
	curseForgeLoaderForge    = "1"
	curseForgeLoaderFabric   = "4"
	curseForgeLoaderQuilt    = "5"
	curseForgeLoaderNeoForge = "6"
)

type cfPagination struct {
	Index       int `json:"index"`
	PageSize    int `json:"pageSize"`
	ResultCount int `json:"resultCount"`
	TotalCount  int `json:"totalCount"`
}

type cfAuthor struct {
	Name string `json:"name"`
}

type cfLogo struct {
	URL string `json:"url"`
}

type cfMod struct {
	ID            int64      `json:"id"`
	Slug          string     `json:"slug"`
	Name          string     `json:"name"`
	Summary       string     `json:"summary"`
	DownloadCount int64      `json:"downloadCount"`
	Authors       []cfAuthor `json:"authors"`
	Logo          *cfLogo    `json:"logo"`
}

type cfSearchResponse struct {
	Data       []cfMod      `json:"data"`
	Pagination cfPagination `json:"pagination"`
}

type cfHash struct {
	Value string `json:"value"`
	Algo  int    `json:"algo"`
}

type cfDependency struct {
	ModID        int64 `json:"modId"`
	RelationType int   `json:"relationType"`
}

type cfFile struct {
	ID            int64          `json:"id"`
	ModID         int64          `json:"modId"`
	DisplayName   string         `json:"displayName"`
	FileName      string         `json:"fileName"`
	DownloadURL   *string        `json:"downloadUrl"`
	FileLength    int64          `json:"fileLength"`
	FileDate      string         `json:"fileDate"`
	DownloadCount int64          `json:"downloadCount"`
	Hashes        []cfHash       `json:"hashes"`
	GameVersions  []string       `json:"gameVersions"`
	Dependencies  []cfDependency `json:"dependencies"`
}

type cfFilesResponse struct {
	Data       []cfFile     `json:"data"`
	Pagination cfPagination `json:"pagination"`
}

// searchCurseForge searches one CurseForge class. The mirror ignores the query
// unless sorting is explicit, so sortOrder is always sent.
func (c *Client) searchCurseForge(ctx context.Context, projectType, query, gameVersion, loader string, sort Sort, limit, offset int) (SearchResult, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	params := url.Values{}
	params.Set("gameId", curseForgeGameID)
	params.Set("classId", curseForgeClassID(projectType))
	params.Set("sortOrder", "desc")
	params.Set("pageSize", strconv.Itoa(limit))
	params.Set("index", strconv.Itoa(max(0, offset)))
	if text := strings.TrimSpace(query); text != "" {
		params.Set("searchFilter", text)
	}
	if version := strings.TrimSpace(gameVersion); version != "" {
		params.Set("gameVersion", version)
	}
	if projectType == ProjectTypeMod {
		if loaderID := curseForgeLoaderID(loader); loaderID != "" {
			params.Set("modLoaderType", loaderID)
		}
	}
	switch NormalizeSort(sort) {
	case SortDownloads:
		params.Set("sortField", "TotalDownloads")
	case SortNewest, SortUpdated:
		params.Set("sortField", "LastUpdated")
	}

	var response cfSearchResponse
	if err := c.getCurseForge(ctx, "/mods/search?"+params.Encode(), &response); err != nil {
		return SearchResult{}, err
	}
	hits := make([]Project, 0, len(response.Data))
	for _, item := range response.Data {
		hits = append(hits, item.project())
	}
	return SearchResult{
		Hits:      hits,
		TotalHits: response.Pagination.TotalCount,
		Offset:    response.Pagination.Index,
		Limit:     response.Pagination.PageSize,
	}, nil
}

// curseForgeVersions lists the files of one project, newest first, filtered to
// the requested game version and loader.
func (c *Client) curseForgeVersions(ctx context.Context, projectID, gameVersion, loader string) ([]Version, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, fmt.Errorf("project id is empty")
	}
	params := url.Values{}
	params.Set("pageSize", "50")
	if version := strings.TrimSpace(gameVersion); version != "" {
		params.Set("gameVersion", version)
	}
	if loaderID := curseForgeLoaderID(loader); loaderID != "" {
		params.Set("modLoaderType", loaderID)
	}

	var response cfFilesResponse
	path := "/mods/" + url.PathEscape(projectID) + "/files?" + params.Encode()
	if err := c.getCurseForge(ctx, path, &response); err != nil {
		return nil, err
	}
	out := make([]Version, 0, len(response.Data))
	for _, file := range response.Data {
		version, ok := file.version()
		if ok {
			out = append(out, version)
		}
	}
	return out, nil
}

func (m cfMod) project() Project {
	author := ""
	if len(m.Authors) > 0 {
		author = m.Authors[0].Name
	}
	icon := ""
	if m.Logo != nil {
		icon = m.Logo.URL
	}
	return Project{
		ID:          strconv.FormatInt(m.ID, 10),
		Slug:        m.Slug,
		Title:       m.Name,
		Description: m.Summary,
		Author:      author,
		Downloads:   m.DownloadCount,
		IconURL:     icon,
		Source:      SourceCurseForge,
	}
}

// version converts one CurseForge file into the shared version shape. Files
// without a download URL are skipped: the author disabled third-party
// distribution, and the launcher must not work around that.
func (f cfFile) version() (Version, bool) {
	if strings.TrimSpace(f.FileName) == "" {
		return Version{}, false
	}
	downloadURL := ""
	if f.DownloadURL != nil {
		downloadURL = strings.TrimSpace(*f.DownloadURL)
	}
	if downloadURL == "" {
		return Version{}, false
	}

	hashes := make(map[string]string, len(f.Hashes))
	for _, hash := range f.Hashes {
		if hash.Algo == 1 && hash.Value != "" {
			hashes["sha1"] = hash.Value
		}
	}
	name := strings.TrimSpace(f.DisplayName)
	if name == "" {
		name = f.FileName
	}
	return Version{
		ID:            strconv.FormatInt(f.ID, 10),
		ProjectID:     strconv.FormatInt(f.ModID, 10),
		Name:          name,
		VersionNumber: name,
		GameVersions:  curseForgeGameVersions(f.GameVersions),
		Loaders:       curseForgeLoaders(f.GameVersions),
		Published:     f.FileDate,
		Downloads:     f.DownloadCount,
		Files: []File{{
			Filename: f.FileName,
			URL:      downloadURL,
			Size:     f.FileLength,
			Primary:  true,
			Hashes:   hashes,
		}},
		Dependencies: curseForgeDependencies(f.Dependencies),
	}, true
}

// curseForgeDependencies keeps the required relations (relationType 3).
func curseForgeDependencies(raw []cfDependency) []Dependency {
	var out []Dependency
	for _, dependency := range raw {
		if dependency.RelationType != 3 || dependency.ModID <= 0 {
			continue
		}
		out = append(out, Dependency{
			ProjectID: strconv.FormatInt(dependency.ModID, 10),
			Type:      "required",
		})
	}
	return out
}

var minecraftVersionRE = regexp.MustCompile(`^(\d+(\.\d+)*|[0-9]{2}w[0-9]{2}[a-z])$`)

// curseForgeGameVersions removes the Client/Server markers so the version list
// only shows actual Minecraft versions.
func curseForgeGameVersions(raw []string) []string {
	out := make([]string, 0, len(raw))
	for _, version := range raw {
		if minecraftVersionRE.MatchString(version) {
			out = append(out, version)
		}
	}
	return out
}

// curseForgeLoaders extracts the loader tags from a CurseForge gameVersions
// array.
func curseForgeLoaders(raw []string) []string {
	var out []string
	for _, version := range raw {
		switch strings.ToLower(version) {
		case "fabric", "quilt", "forge", "neoforge":
			out = append(out, strings.ToLower(version))
		}
	}
	return out
}

func curseForgeClassID(projectType string) string {
	switch projectType {
	case ProjectTypeShader:
		return curseForgeClassShader
	case ProjectTypeMod:
		return curseForgeClassMod
	default:
		return curseForgeClassResourcePack
	}
}

// curseForgeLoaderID maps a Modrinth-style loader tag onto the numeric id the
// CurseForge API expects. OptiFine has no CurseForge loader id.
func curseForgeLoaderID(loader string) string {
	switch strings.ToLower(strings.TrimSpace(loader)) {
	case "forge":
		return curseForgeLoaderForge
	case "fabric":
		return curseForgeLoaderFabric
	case "quilt":
		return curseForgeLoaderQuilt
	case "neoforge":
		return curseForgeLoaderNeoForge
	default:
		return ""
	}
}
