package skin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MoonCC233/MCLauncher-TUI/internal/util"
)

// InjectorVersion is the authlib-injector release bundled by the launcher on
// first use. The artifact is AGPL-3.0, so it is downloaded instead of shipped.
const (
	InjectorVersion = "1.2.8"
	injectorSHA256  = "9c7f4343e6c82034958ffb48c14a2cb0c85928be7283103ce17da00c6d5a7b10"
	injectorSize    = 349681
	injectorURL     = "https://github.com/yushijinhun/authlib-injector/releases/download/v1.2.8/authlib-injector-1.2.8.jar"
)

// injectorURLForTest lets tests point the downloader at a local server.
var injectorURLForTest = injectorURL

// InjectorPath is the cached path inside the launcher's cache directory.
func InjectorPath(dir string) string {
	return filepath.Join(dir, "authlib-injector-"+InjectorVersion+".jar")
}

// EnsureInjector returns a verified authlib-injector jar, downloading it into
// dir when it is missing. MCLAUNCHER_AUTHLIB_INJECTOR can point at an existing
// jar (for example a newer build); that file is used as-is.
func EnsureInjector(ctx context.Context, client *http.Client, dir string) (string, error) {
	if override := strings.TrimSpace(os.Getenv("MCLAUNCHER_AUTHLIB_INJECTOR")); override != "" {
		st, err := os.Stat(override)
		if err != nil {
			return "", fmt.Errorf("MCLAUNCHER_AUTHLIB_INJECTOR: %w", err)
		}
		if st.IsDir() || st.Size() == 0 {
			return "", fmt.Errorf("MCLAUNCHER_AUTHLIB_INJECTOR: %s is not a jar file", override)
		}
		return override, nil
	}
	path := InjectorPath(dir)
	if verifyInjector(path) == nil {
		return path, nil
	}
	if client == nil {
		client = util.NewHTTPClient()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	// The jar is small; do not let an unreachable mirror hold up a launch.
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	var lastErr error
	for _, url := range injectorURLs(injectorURLForTest) {
		if err := downloadInjector(ctx, client, url, path); err != nil {
			lastErr = err
			continue
		}
		if err := verifyInjector(path); err != nil {
			lastErr = err
			_ = os.Remove(path)
			continue
		}
		return path, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no download source configured")
	}
	return "", fmt.Errorf("download authlib-injector: %w", lastErr)
}

func verifyInjector(path string) error {
	st, err := os.Stat(path)
	if err != nil {
		return err
	}
	if st.IsDir() {
		return fmt.Errorf("%s is a directory", path)
	}
	if st.Size() != injectorSize {
		return fmt.Errorf("%s has size %d, want %d", path, st.Size(), injectorSize)
	}
	got, err := sha256File(path)
	if err != nil {
		return err
	}
	if !strings.EqualFold(got, injectorSHA256) {
		return fmt.Errorf("%s has SHA-256 %s, want %s", path, got, injectorSHA256)
	}
	return nil
}

func downloadInjector(ctx context.Context, client *http.Client, url, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: HTTP %s", url, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	return util.WriteFileAtomic(path, data, 0o644)
}

func injectorURLs(raw string) []string {
	var out []string
	if mirror := strings.TrimSpace(os.Getenv("MCLAUNCHER_AUTHLIB_MIRROR")); mirror != "" {
		if strings.Contains(mirror, "{url}") {
			out = append(out, strings.ReplaceAll(mirror, "{url}", raw))
		} else {
			out = append(out, strings.TrimRight(mirror, "/")+"/"+raw)
		}
	}
	if strings.HasPrefix(raw, "https://github.com/") {
		out = append(out,
			"https://ghfast.top/"+raw,
			"https://gh-proxy.com/"+raw,
			"https://ghproxy.net/"+raw,
		)
	}
	out = append(out, raw)
	seen := map[string]bool{}
	unique := out[:0]
	for _, url := range out {
		if url == "" || seen[url] {
			continue
		}
		seen[url] = true
		unique = append(unique, url)
	}
	return unique
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
