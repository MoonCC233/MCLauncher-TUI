// Package update checks GitHub Releases, downloads the matching archive
// through mirrors when necessary, and replaces the running launcher binary.
package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/MoonCC233/MCLauncher-TUI/internal/util"
)

const (
	repository = "MoonCC233/MCLauncher-TUI"
	latestAPI  = "https://api.github.com/repos/" + repository + "/releases/latest"

	ghProxy    = "https://gh-proxy.com/"
	ghFast     = "https://ghfast.top/"
	ghProxyNet = "https://ghproxy.net/"
)

// ErrDevelopment is returned when the running binary has no release version.
var ErrDevelopment = errors.New("development build")

type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	Size int64  `json:"size"`
}

type Release struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Body        string    `json:"body"`
	PublishedAt time.Time `json:"published_at"`
	Assets      []Asset   `json:"assets"`
}

// Progress reports one stage of the download/install operation.
type Progress struct {
	Stage  string
	Detail string
	Done   int64
	Total  int64
}

// Check returns the latest release when it is newer than current. A nil release
// and nil error mean the launcher is already up to date.
func Check(ctx context.Context, client *http.Client, current string) (*Release, error) {
	if IsDevelopment(current) {
		return nil, ErrDevelopment
	}
	if client == nil {
		client = util.NewHTTPClient()
	}
	var lastErr error
	for _, url := range apiCandidates(latestAPI) {
		release, err := fetchRelease(ctx, client, url)
		if err != nil {
			lastErr = err
			continue
		}
		if CompareVersions(release.TagName, current) > 0 {
			return release, nil
		}
		return nil, nil
	}
	if lastErr == nil {
		lastErr = errors.New("no update source is available")
	}
	return nil, lastErr
}

func fetchRelease(ctx context.Context, client *http.Client, url string) (*Release, error) {
	attemptCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(attemptCtx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "mclauncher-updater")
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: HTTP %d", url, resp.StatusCode)
	}
	var release Release
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&release); err != nil {
		return nil, fmt.Errorf("decode release: %w", err)
	}
	if strings.TrimSpace(release.TagName) == "" {
		return nil, errors.New("release has no tag name")
	}
	return &release, nil
}

// IsDevelopment reports whether version is a local/dev build.
func IsDevelopment(version string) bool {
	version = strings.TrimSpace(version)
	return version == "" || version == "unknown" || version == "dev" ||
		strings.HasPrefix(version, "dev-") || strings.HasPrefix(version, "dev ") ||
		strings.HasPrefix(version, "dev(")
}

// FindAsset selects the archive for goos/goarch. GoReleaser names assets after
// the tag without its leading "v".
func FindAsset(release *Release, goos, goarch string) (*Asset, error) {
	if release == nil {
		return nil, errors.New("missing release")
	}
	ext := "tar.gz"
	if goos == "windows" {
		ext = "zip"
	}
	version := strings.TrimPrefix(release.TagName, "v")
	for _, candidate := range []string{
		fmt.Sprintf("mclauncher_%s_%s_%s.%s", version, goos, goarch, ext),
		fmt.Sprintf("mclauncher_%s_%s_%s.%s", release.TagName, goos, goarch, ext),
	} {
		for i := range release.Assets {
			if release.Assets[i].Name == candidate {
				return &release.Assets[i], nil
			}
		}
	}
	names := make([]string, 0, len(release.Assets))
	for _, asset := range release.Assets {
		names = append(names, asset.Name)
	}
	return nil, fmt.Errorf("release %s has no asset for %s/%s; available: %s",
		release.TagName, goos, goarch, strings.Join(names, ", "))
}

func checksumAsset(release *Release) *Asset {
	for i := range release.Assets {
		if release.Assets[i].Name == "checksums.txt" {
			return &release.Assets[i]
		}
	}
	return nil
}

// CompareVersions compares two release-like version strings. It understands
// "v" prefixes, build metadata, and a single prerelease suffix.
func CompareVersions(a, b string) int {
	av, bv := parseVersion(a), parseVersion(b)
	for i := 0; i < 3; i++ {
		if av.numbers[i] != bv.numbers[i] {
			if av.numbers[i] > bv.numbers[i] {
				return 1
			}
			return -1
		}
	}
	if av.pre == bv.pre {
		return 0
	}
	if av.pre == "" {
		return 1
	}
	if bv.pre == "" {
		return -1
	}
	return comparePrerelease(av.pre, bv.pre)
}

type parsedVersion struct {
	numbers [3]int
	pre     string
}

func parseVersion(version string) parsedVersion {
	version = strings.TrimSpace(version)
	version = strings.TrimPrefix(version, "v")
	version = strings.TrimPrefix(version, "V")
	if i := strings.IndexAny(version, " (+"); i >= 0 {
		version = version[:i]
	}
	if i := strings.IndexByte(version, '+'); i >= 0 {
		version = version[:i]
	}
	out := parsedVersion{}
	if i := strings.IndexByte(version, '-'); i >= 0 {
		out.pre = version[i+1:]
		version = version[:i]
	}
	parts := strings.Split(version, ".")
	for i := 0; i < 3 && i < len(parts); i++ {
		out.numbers[i], _ = strconv.Atoi(parts[i])
	}
	return out
}

func comparePrerelease(a, b string) int {
	ap, bp := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(ap) && i < len(bp); i++ {
		if ap[i] == bp[i] {
			continue
		}
		an, aErr := strconv.Atoi(ap[i])
		bn, bErr := strconv.Atoi(bp[i])
		switch {
		case aErr == nil && bErr == nil:
			if an > bn {
				return 1
			}
			return -1
		case aErr == nil:
			return -1
		case bErr == nil:
			return 1
		case ap[i] > bp[i]:
			return 1
		default:
			return -1
		}
	}
	if len(ap) > len(bp) {
		return 1
	}
	if len(ap) < len(bp) {
		return -1
	}
	return 0
}

func apiCandidates(raw string) []string {
	var out []string
	if mirror := updateMirror(); mirror != "" {
		out = append(out, mirrorURL(mirror, raw))
	}
	out = append(out,
		mirrorURL(ghProxy, raw),
		mirrorURL(ghProxyNet, raw),
		mirrorURL(ghFast, raw),
		raw,
	)
	return dedupe(out)
}

func downloadCandidates(raw string) []string {
	var out []string
	if mirror := updateMirror(); mirror != "" {
		out = append(out, mirrorURL(mirror, raw))
	}
	out = append(out,
		mirrorURL(ghFast, raw),
		mirrorURL(ghProxy, raw),
		mirrorURL(ghProxyNet, raw),
		raw,
	)
	return dedupe(out)
}

func updateMirror() string {
	return strings.TrimSpace(os.Getenv("MCLAUNCHER_UPDATE_MIRROR"))
}

func mirrorURL(prefix, raw string) string {
	prefix = strings.TrimSpace(prefix)
	if strings.Contains(prefix, "{url}") {
		return strings.ReplaceAll(prefix, "{url}", raw)
	}
	return strings.TrimRight(prefix, "/") + "/" + raw
}

func dedupe(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
