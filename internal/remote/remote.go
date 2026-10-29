// Package remote reads the project's release notes, contributors and issues from
// GitHub. Everything is cached on disk so the launcher still has something to
// show when the network or the API rate limit says no.
package remote

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MoonCC233/MCLauncher-TUI/internal/update"
	"github.com/MoonCC233/MCLauncher-TUI/internal/util"
)

// Repository is the upstream project these feeds come from.
const Repository = "MoonCC233/MCLauncher-TUI"

const (
	apiBase     = "https://api.github.com/repos/" + Repository
	releasesURL = apiBase + "/releases?per_page=20"

	// cacheTTL keeps the launcher from hammering the API: unauthenticated
	// requests are limited to sixty an hour.
	cacheTTL = 30 * time.Minute
)

// Announcement is one thing worth telling the player about, built from a
// release: the popup and the release list share this shape.
type Announcement struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Level     string    `json:"level,omitempty"`
	Published time.Time `json:"published"`
}

// Contributor is one entry of the GitHub contributors API.
type Contributor struct {
	Login         string `json:"login"`
	Contributions int    `json:"contributions"`
	HTMLURL       string `json:"html_url"`
}

// Issue is one entry of the GitHub issues API. Pull requests are reported by the
// same endpoint, so Issue keeps the marker to filter them out.
type Issue struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	State     string    `json:"state"`
	Comments  int       `json:"comments"`
	HTMLURL   string    `json:"html_url"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	User      struct {
		Login string `json:"login"`
	} `json:"user"`
	PullRequest *struct{} `json:"pull_request,omitempty"`
}

// IssueURL is the page a player can read or reply on.
func (i Issue) IssueURL() string { return i.HTMLURL }

// NewIssueURL opens the bug report form.
func NewIssueURL() string {
	return "https://github.com/" + Repository + "/issues/new?template=bug_report.yml"
}

// RepoURL is the project home page.
func RepoURL() string { return "https://github.com/" + Repository }

// ReleaseNotes returns the release the running build belongs to, as a single
// announcement. When the version is a development build, or the release is not
// published yet, the newest release stands in for it.
//
// The popup and the release list share one cache: the notes come from the same
// request the About screen makes.
func ReleaseNotes(ctx context.Context, client *http.Client, cachePath, currentVersion string, force bool) ([]Announcement, error) {
	// The popup is shown at startup, so waiting for four dead mirrors in a row
	// would push it past the point where anyone still cares.
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	return fetchReleaseNotesFrom(ctx, client, cachePath, force, currentVersion, update.Candidates(releasesURL))
}

func fetchReleaseNotesFrom(ctx context.Context, client *http.Client, cachePath string, force bool, currentVersion string, urls []string) ([]Announcement, error) {
	releases, err := fetchReleases(ctx, client, cachePath, force, urls)
	if err != nil {
		return nil, err
	}
	if len(releases) == 0 {
		return nil, nil
	}
	picked := releases[0]
	if wanted := strings.TrimSpace(currentVersion); wanted != "" {
		for _, release := range releases {
			if sameVersion(release.TagName, wanted) {
				picked = release
				break
			}
		}
	}
	notes := Announcement{
		ID:        firstNonEmpty(picked.TagName, picked.Name),
		Title:     firstNonEmpty(picked.Name, picked.TagName),
		Body:      picked.Body,
		Published: picked.PublishedAt,
	}
	if notes.ID == "" {
		return nil, nil
	}
	return []Announcement{notes}, nil
}

// sameVersion compares a release tag with the running version, ignoring the
// leading v and the " (commit)" suffix.
func sameVersion(tag, current string) bool {
	tag = strings.TrimSpace(tag)
	current = strings.TrimSpace(current)
	if i := strings.IndexAny(current, " ("); i > 0 {
		current = current[:i]
	}
	if tag == "" || current == "" || current == "dev" {
		return false
	}
	return update.CompareVersions(tag, current) == 0
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// Contributors lists the people who wrote the project.
func Contributors(ctx context.Context, client *http.Client, cachePath string, force bool) ([]Contributor, error) {
	var list []Contributor
	url := apiBase + "/contributors?per_page=50"
	if err := fetchCached(ctx, client, cachePath, force, &list, update.Candidates(url)); err != nil {
		return nil, err
	}
	return list, nil
}

// Releases lists the published releases, newest first.
func Releases(ctx context.Context, client *http.Client, cachePath string, force bool) ([]update.Release, error) {
	return fetchReleases(ctx, client, cachePath, force, update.Candidates(releasesURL))
}

func fetchReleases(ctx context.Context, client *http.Client, cachePath string, force bool, urls []string) ([]update.Release, error) {
	var list []update.Release
	if err := fetchCached(ctx, client, cachePath, force, &list, urls); err != nil {
		return nil, err
	}
	return list, nil
}

// Issues lists the reported issues and feature requests, most recently updated
// first, without the pull requests the same endpoint returns.
func Issues(ctx context.Context, client *http.Client, cachePath string, force bool) ([]Issue, error) {
	var list []Issue
	url := apiBase + "/issues?state=all&sort=updated&per_page=30"
	if err := fetchCached(ctx, client, cachePath, force, &list, update.Candidates(url)); err != nil {
		return nil, err
	}
	return filterIssues(list), nil
}

// filterIssues drops the pull requests GitHub returns from the issues endpoint.
func filterIssues(list []Issue) []Issue {
	out := make([]Issue, 0, len(list))
	for _, issue := range list {
		if issue.PullRequest != nil {
			continue
		}
		out = append(out, issue)
	}
	return out
}

// fetchCached serves a fresh cache when it can, otherwise tries every candidate
// URL and falls back to the stale cache on failure.
func fetchCached(ctx context.Context, client *http.Client, cachePath string, force bool, target any, urls []string) error {
	if client == nil {
		client = util.NewHTTPClient()
	}
	if !force {
		if data, err := os.ReadFile(cachePath); err == nil {
			if info, statErr := os.Stat(cachePath); statErr == nil && time.Since(info.ModTime()) < cacheTTL {
				if json.Unmarshal(data, target) == nil {
					return nil
				}
			}
		}
	}

	var lastErr error
	for _, url := range urls {
		data, err := getJSON(ctx, client, url)
		if err != nil {
			lastErr = err
			continue
		}
		if err := json.Unmarshal(data, target); err != nil {
			lastErr = fmt.Errorf("decode %s: %w", url, err)
			continue
		}
		writeCache(cachePath, data)
		return nil
	}

	// Offline or rate limited: a stale answer beats an empty screen.
	if data, err := os.ReadFile(cachePath); err == nil {
		if json.Unmarshal(data, target) == nil {
			return nil
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no source is available")
	}
	return lastErr
}

func getJSON(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	attemptCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(attemptCtx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", "mclauncher")
	request.Header.Set("Accept", "application/vnd.github+json")
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", url, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: HTTP %d", url, response.StatusCode)
	}
	return io.ReadAll(io.LimitReader(response.Body, 4<<20))
}

func writeCache(path string, data []byte) {
	if strings.TrimSpace(path) == "" {
		return
	}
	if err := util.EnsureDir(filepath.Dir(path)); err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o644)
}

func sortNewestFirst(list []Announcement) {
	for i := 1; i < len(list); i++ {
		for j := i; j > 0 && list[j].Published.After(list[j-1].Published); j-- {
			list[j], list[j-1] = list[j-1], list[j]
		}
	}
}
