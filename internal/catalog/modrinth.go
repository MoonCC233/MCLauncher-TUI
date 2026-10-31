// Package catalog queries the Modrinth and CurseForge content catalogs.
package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultBaseURL is Modrinth's public API root.
	DefaultBaseURL = "https://api.modrinth.com/v2"
	// DefaultFallbackURL is a read-only Modrinth mirror used when the official
	// API cannot be reached.
	DefaultFallbackURL = "https://mod.mcimirror.top/modrinth/v2"

	// ProjectTypeResourcePack is the facet used for resource packs in search.
	ProjectTypeResourcePack = "resourcepack"
	// ProjectTypeShader is the facet used for shader packs in search.
	ProjectTypeShader = "shader"
	// ProjectTypeMod is the facet used for mods in search.
	ProjectTypeMod = "mod"

	// MinecraftLoader is the loader tag Modrinth uses for vanilla-compatible files.
	MinecraftLoader = "minecraft"
)

// Project is one search hit.
type Project struct {
	ID          string   `json:"project_id"`
	Slug        string   `json:"slug"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Author      string   `json:"author"`
	Downloads   int64    `json:"downloads"`
	IconURL     string   `json:"icon_url"`
	Versions    []string `json:"versions"`
	// Source records which catalog produced the hit. It is not part of the
	// remote payload, so it is filled in by the client.
	Source Source `json:"-"`
}

// SearchResult is one page of search hits.
type SearchResult struct {
	Hits      []Project `json:"hits"`
	TotalHits int       `json:"total_hits"`
	Offset    int       `json:"offset"`
	Limit     int       `json:"limit"`
}

// File is one downloadable file of a version.
type File struct {
	Filename string            `json:"filename"`
	URL      string            `json:"url"`
	Size     int64             `json:"size"`
	Primary  bool              `json:"primary"`
	Hashes   map[string]string `json:"hashes"`
}

// Version is one released version of a project.
type Version struct {
	ID            string       `json:"id"`
	ProjectID     string       `json:"project_id"`
	Name          string       `json:"name"`
	VersionNumber string       `json:"version_number"`
	GameVersions  []string     `json:"game_versions"`
	Loaders       []string     `json:"loaders"`
	Published     string       `json:"date_published"`
	Downloads     int64        `json:"downloads"`
	Files         []File       `json:"files"`
	Dependencies  []Dependency `json:"dependencies,omitempty"`
}

// Dependency is one required or optional relation of a version. Both catalogs
// are normalised onto this shape: Type is "required", "optional", "incompatible"
// or "embedded".
type Dependency struct {
	ProjectID string `json:"project_id,omitempty"`
	VersionID string `json:"version_id,omitempty"`
	FileName  string `json:"file_name,omitempty"`
	Type      string `json:"dependency_type,omitempty"`
}

// PrimaryFile returns the file the author marked as primary, falling back to the
// first one so a malformed entry cannot break the installer.
func (v Version) PrimaryFile() (File, bool) {
	for _, file := range v.Files {
		if file.Primary {
			return file, true
		}
	}
	if len(v.Files) > 0 {
		return v.Files[0], true
	}
	return File{}, false
}

// Client talks to the Modrinth and CurseForge catalogs.
type Client struct {
	HTTP    *http.Client
	BaseURL string
	// FallbackBaseURL is tried when BaseURL is the official Modrinth API and
	// that request fails. Tests leave it empty.
	FallbackBaseURL string
	// CurseForgeBaseURL is the CurseForge API root. The default points at a
	// keyless read-only mirror, so no private API key ships with the launcher.
	CurseForgeBaseURL string
	UserAgent         string
}

// NewClient builds a client with a sensible timeout and the identifying user
// agent Modrinth asks every application to send.
func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 20 * time.Second}
	}
	return &Client{
		HTTP:              httpClient,
		BaseURL:           DefaultBaseURL,
		FallbackBaseURL:   DefaultFallbackURL,
		CurseForgeBaseURL: DefaultCurseForgeBaseURL,
		UserAgent:         "mclauncher (github.com/MoonCC233/MCLauncher-TUI)",
	}
}

// SearchPacks searches resource packs, optionally restricted to a game version.
func (c *Client) SearchPacks(ctx context.Context, query, gameVersion string, limit, offset int) (SearchResult, error) {
	return c.Search(ctx, ProjectTypeResourcePack, query, gameVersion, "", limit, offset)
}

// Search looks for one project type, optionally restricted to a game version and
// a mod loader (loaders are exposed as categories by Modrinth).
func (c *Client) Search(ctx context.Context, projectType, query, gameVersion, loader string, limit, offset int) (SearchResult, error) {
	return c.SearchWithSort(ctx, projectType, query, gameVersion, loader, SortRelevance, limit, offset)
}

// SearchWithSort is Search with an explicit result ordering.
func (c *Client) SearchWithSort(ctx context.Context, projectType, query, gameVersion, loader string, sort Sort, limit, offset int) (SearchResult, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	if strings.TrimSpace(projectType) == "" {
		projectType = ProjectTypeResourcePack
	}
	facets := [][]string{{"project_type:" + strings.TrimSpace(projectType)}}
	if strings.TrimSpace(gameVersion) != "" {
		facets = append(facets, []string{"versions:" + strings.TrimSpace(gameVersion)})
	}
	if strings.TrimSpace(loader) != "" {
		facets = append(facets, []string{"categories:" + strings.TrimSpace(loader)})
	}
	encoded, err := json.Marshal(facets)
	if err != nil {
		return SearchResult{}, err
	}
	params := url.Values{}
	params.Set("query", strings.TrimSpace(query))
	params.Set("facets", string(encoded))
	params.Set("limit", strconv.Itoa(limit))
	params.Set("offset", strconv.Itoa(max(0, offset)))
	params.Set("index", string(NormalizeSort(sort)))

	var out SearchResult
	if err := c.get(ctx, "/search?"+params.Encode(), &out); err != nil {
		return SearchResult{}, err
	}
	return out, nil
}

// Versions lists the versions of a project, newest first, filtered to the game
// version when one is given and to vanilla-compatible files.
func (c *Client) Versions(ctx context.Context, projectID, gameVersion string) ([]Version, error) {
	return c.VersionsFor(ctx, projectID, gameVersion, "")
}

// VersionsFor is Versions with an optional loader filter.
func (c *Client) VersionsFor(ctx context.Context, projectID, gameVersion, loader string) ([]Version, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, fmt.Errorf("project id is empty")
	}
	params := url.Values{}
	if strings.TrimSpace(gameVersion) != "" {
		encoded, err := json.Marshal([]string{strings.TrimSpace(gameVersion)})
		if err != nil {
			return nil, err
		}
		params.Set("game_versions", string(encoded))
	}
	if strings.TrimSpace(loader) != "" {
		encoded, err := json.Marshal([]string{strings.TrimSpace(loader)})
		if err != nil {
			return nil, err
		}
		params.Set("loaders", string(encoded))
	}
	path := "/project/" + url.PathEscape(projectID) + "/version"
	if encoded := params.Encode(); encoded != "" {
		path += "?" + encoded
	}

	var versions []Version
	if err := c.get(ctx, path, &versions); err != nil {
		return nil, err
	}
	compatible := versions[:0]
	for _, version := range versions {
		if _, ok := version.PrimaryFile(); ok {
			compatible = append(compatible, version)
		}
	}
	return compatible, nil
}

func (c *Client) get(ctx context.Context, path string, out any) error {
	base := strings.TrimRight(c.BaseURL, "/")
	if base == "" {
		base = DefaultBaseURL
	}
	err := c.getJSON(ctx, base+path, out, "modrinth")
	if err == nil {
		return err
	}
	// The official API is unreachable or returned a temporary error. A
	// read-only mirror keeps browsing usable without hiding the original
	// failure when both are down.
	if base == DefaultBaseURL {
		if fallback := strings.TrimRight(c.FallbackBaseURL, "/"); fallback != "" && fallback != base {
			if mirrorErr := c.getJSON(ctx, fallback+path, out, "modrinth mirror"); mirrorErr == nil {
				return nil
			}
		}
	}
	return err
}

// getCurseForge reads one CurseForge endpoint.
func (c *Client) getCurseForge(ctx context.Context, path string, out any) error {
	base := strings.TrimRight(c.CurseForgeBaseURL, "/")
	if base == "" {
		base = DefaultCurseForgeBaseURL
	}
	return c.getJSON(ctx, base+path, out, "curseforge")
}

// getJSON decodes one JSON response. Server errors and transport failures are
// retried once because the public read-only mirrors occasionally answer 502.
func (c *Client) getJSON(ctx context.Context, rawURL string, out any, label string) error {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := c.getJSONOnce(ctx, rawURL, out, label)
		if err == nil {
			return nil
		}
		lastErr = err
		if !retryableCatalogError(err) {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 400 * time.Millisecond):
		}
	}
	return lastErr
}

// statusError is a non-200 catalog response. It keeps the status code so the
// retry decision does not depend on message matching.
type statusError struct {
	label  string
	status string
	code   int
}

func (e statusError) Error() string { return e.label + ": " + e.status }

func (c *Client) getJSONOnce(ctx context.Context, rawURL string, out any, label string) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")
	if c.UserAgent != "" {
		request.Header.Set("User-Agent", c.UserAgent)
	}
	client := c.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return statusError{label: label, status: response.Status, code: response.StatusCode}
	}
	return json.NewDecoder(response.Body).Decode(out)
}

// retryableCatalogError reports whether a failed request is worth repeating on
// the same host. Missing projects and refused requests are not.
func retryableCatalogError(err error) bool {
	var failure statusError
	if errors.As(err, &failure) {
		return failure.code == http.StatusTooManyRequests || failure.code >= 500
	}
	// Transport failures have no status code and are usually transient.
	return true
}
