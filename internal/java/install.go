package java

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/MoonCC233/MCLauncher-TUI/internal/util"
)

// Progress reports one stage of a Java download/install operation.
type Progress struct {
	Stage  string
	Detail string
	Done   int64
	Total  int64
}

type adoptiumPackage struct {
	Link     string `json:"link"`
	Name     string `json:"name"`
	Checksum string `json:"checksum"`
	Size     int64  `json:"size"`
}

type adoptiumAsset struct {
	Binary struct {
		Package adoptiumPackage `json:"package"`
	} `json:"binary"`
	Version struct {
		Semver string `json:"semver"`
		Major  int    `json:"major"`
	} `json:"version"`
}

const adoptiumAPI = "https://api.adoptium.net/v3/assets/latest/%d/hotspot?architecture=%s&image_type=jdk&os=%s&vendor=eclipse"

// InstallLatest downloads the latest Adoptium Temurin JDK for major into
// root/temurin-<major> and returns the detected runtime.
func InstallLatest(
	ctx context.Context,
	client *http.Client,
	root string,
	major int,
	progress func(Progress),
) (*Info, error) {
	if root == "" {
		return nil, fmt.Errorf("missing Java install directory")
	}
	if major < 8 {
		return nil, fmt.Errorf("unsupported Java version %d", major)
	}
	if client == nil {
		client = util.NewHTTPClient()
	}
	osName := map[string]string{"windows": "windows", "linux": "linux", "darwin": "mac"}[runtime.GOOS]
	arch := map[string]string{"amd64": "x64", "arm64": "aarch64", "386": "x86"}[runtime.GOARCH]
	if osName == "" || arch == "" {
		return nil, fmt.Errorf("Adoptium does not support %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	reportJava(progress, Progress{Stage: "Resolving Java runtime", Detail: "Adoptium Temurin JDK"})
	api := fmt.Sprintf(adoptiumAPI, major, arch, osName)
	assets, err := fetchAdoptiumAssets(ctx, client, api)
	if err != nil {
		return nil, err
	}
	if len(assets) == 0 {
		return nil, fmt.Errorf("no Adoptium JDK %d found for %s/%s", major, runtime.GOOS, runtime.GOARCH)
	}
	asset := assets[0]
	pkg := asset.Binary.Package
	if pkg.Link == "" || pkg.Name == "" {
		return nil, fmt.Errorf("Adoptium returned an incomplete package")
	}

	tempDir, err := os.MkdirTemp("", "mclauncher-java-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempDir)
	archivePath := filepath.Join(tempDir, pkg.Name)
	reportJava(progress, Progress{Stage: "Downloading Java", Detail: pkg.Name, Total: pkg.Size})
	if err := downloadJava(ctx, client, pkg.Link, archivePath, pkg.Size, pkg.Checksum, func(done int64) {
		reportJava(progress, Progress{Stage: "Downloading Java", Detail: pkg.Name, Done: done, Total: pkg.Size})
	}); err != nil {
		return nil, fmt.Errorf("download Java: %w", err)
	}

	reportJava(progress, Progress{Stage: "Extracting Java", Detail: pkg.Name})
	dest := filepath.Join(root, fmt.Sprintf("temurin-%d", major))
	if err := os.RemoveAll(dest); err != nil {
		return nil, err
	}
	success := false
	defer func() {
		if !success {
			_ = os.RemoveAll(dest)
		}
	}()
	if err := extractJavaArchive(archivePath, dest); err != nil {
		return nil, fmt.Errorf("extract Java: %w", err)
	}
	javaPath := findJavaExecutable(dest)
	if javaPath == "" {
		return nil, fmt.Errorf("archive does not contain a java executable")
	}
	info := Detect(javaPath)
	if info == nil {
		return nil, fmt.Errorf("downloaded Java cannot be started: %s", javaPath)
	}
	success = true
	reportJava(progress, Progress{Stage: "Java installed", Detail: info.Short(), Done: 1, Total: 1})
	return info, nil
}

func reportJava(progress func(Progress), value Progress) {
	if progress != nil {
		progress(value)
	}
}

func fetchAdoptiumAssets(ctx context.Context, client *http.Client, url string) ([]adoptiumAsset, error) {
	attemptCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(attemptCtx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "mclauncher-java")
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: HTTP %d", url, resp.StatusCode)
	}
	var assets []adoptiumAsset
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&assets); err != nil {
		return nil, fmt.Errorf("decode Adoptium response: %w", err)
	}
	return assets, nil
}

func downloadJava(
	ctx context.Context,
	client *http.Client,
	rawURL, dest string,
	size int64,
	wantSHA256 string,
	onProgress func(int64),
) error {
	part := dest + ".part"
	var lastErr error
	for _, url := range javaDownloadURLs(rawURL) {
		for attempt := 0; attempt < 8; attempt++ {
			if err := ctx.Err(); err != nil {
				return err
			}
			offset := fileSize(part)
			if size > 0 && offset >= size {
				break
			}
			attemptCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
			err := downloadJavaPart(attemptCtx, client, url, part, offset, size, onProgress)
			cancel()
			if err == nil {
				if size == 0 || fileSize(part) >= size {
					break
				}
				lastErr = fmt.Errorf("incomplete download for %s", filepath.Base(dest))
			} else {
				lastErr = err
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if strings.Contains(lastErr.Error(), "HTTP 404") || strings.Contains(lastErr.Error(), "HTTP 403") {
				break
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Second):
			}
		}
		if size > 0 && fileSize(part) != size {
			continue
		}
		if wantSHA256 != "" {
			got, err := sha256File(part)
			if err != nil {
				lastErr = err
				_ = os.Remove(part)
				continue
			}
			if !strings.EqualFold(got, wantSHA256) {
				lastErr = fmt.Errorf("checksum mismatch for %s", filepath.Base(dest))
				_ = os.Remove(part)
				continue
			}
		}
		if err := os.Rename(part, dest); err != nil {
			return err
		}
		return nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no download source is available")
	}
	return lastErr
}

func downloadJavaPart(
	ctx context.Context,
	client *http.Client,
	url, part string,
	offset, size int64,
	onProgress func(int64),
) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "mclauncher-java")
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		// The server ignored the range request; restart from zero.
		offset = 0
	case http.StatusPartialContent:
		if rangeHeader := resp.Header.Get("Content-Range"); offset > 0 && rangeHeader != "" &&
			!strings.HasPrefix(rangeHeader, fmt.Sprintf("bytes %d-", offset)) {
			return fmt.Errorf("unexpected Content-Range %q", rangeHeader)
		}
	case http.StatusRequestedRangeNotSatisfiable:
		return nil
	default:
		return fmt.Errorf("GET %s: HTTP %d", url, resp.StatusCode)
	}
	flags := os.O_CREATE | os.O_WRONLY
	if offset == 0 {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_APPEND
	}
	f, err := os.OpenFile(part, flags, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	written := offset
	buf := make([]byte, 128*1024)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, err := f.Write(buf[:n]); err != nil {
				return err
			}
			written += int64(n)
			if onProgress != nil {
				onProgress(written)
			}
			if size > 0 && written > size {
				return fmt.Errorf("received more than the expected %d bytes", size)
			}
		}
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
}

func fileSize(name string) int64 {
	st, err := os.Stat(name)
	if err != nil {
		return 0
	}
	return st.Size()
}

func javaDownloadURLs(raw string) []string {
	var out []string
	if mirror := strings.TrimSpace(os.Getenv("MCLAUNCHER_JAVA_MIRROR")); mirror != "" {
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

func sha256File(name string) (string, error) {
	f, err := os.Open(name)
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

func extractJavaArchive(archivePath, dest string) error {
	lower := strings.ToLower(archivePath)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return extractJavaZip(archivePath, dest)
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return extractJavaTarGz(archivePath, dest)
	default:
		return fmt.Errorf("unsupported Java archive %s", filepath.Base(archivePath))
	}
}

func extractJavaZip(archivePath, dest string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()
	root := ""
	for _, file := range reader.File {
		if name := firstPathComponent(file.Name); name != "" {
			root = name
			break
		}
	}
	for _, file := range reader.File {
		rel, ok := archiveEntryPath(file.Name, root)
		if !ok {
			continue
		}
		target, err := util.SafeJoin(dest, rel)
		if err != nil {
			return err
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if file.Mode()&os.ModeSymlink != 0 {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := file.Open()
		if err != nil {
			return err
		}
		err = writeJavaFile(target, rc, javaFileMode(file.Mode().Perm(), rel))
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func extractJavaTarGz(archivePath, dest string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	reader := tar.NewReader(gz)
	root := ""
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if root == "" {
			root = firstPathComponent(header.Name)
		}
		rel, ok := archiveEntryPath(header.Name, root)
		if !ok {
			continue
		}
		target, err := util.SafeJoin(dest, rel)
		if err != nil {
			return err
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := writeJavaFile(target, reader, javaFileMode(header.FileInfo().Mode().Perm(), rel)); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeJavaFile(dest string, src io.Reader, mode os.FileMode) error {
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(f, io.LimitReader(src, 512<<20))
	closeErr := f.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func firstPathComponent(name string) string {
	clean := path.Clean(strings.ReplaceAll(name, "\\", "/"))
	clean = strings.TrimPrefix(clean, "./")
	if clean == "" || clean == "." || strings.HasPrefix(clean, "../") {
		return ""
	}
	return strings.Split(clean, "/")[0]
}

func archiveEntryPath(name, root string) (string, bool) {
	clean := path.Clean(strings.ReplaceAll(name, "\\", "/"))
	clean = strings.TrimPrefix(clean, "./")
	if clean == "" || clean == "." || strings.HasPrefix(clean, "../") {
		return "", false
	}
	if root != "" {
		if clean == root {
			return "", false
		}
		if strings.HasPrefix(clean, root+"/") {
			clean = strings.TrimPrefix(clean, root+"/")
		}
	}
	if clean == "" {
		return "", false
	}
	return clean, true
}

func javaFileMode(mode os.FileMode, rel string) os.FileMode {
	if mode == 0 {
		mode = 0o644
	}
	if runtime.GOOS != "windows" && strings.HasPrefix(rel, "bin/") {
		mode |= 0o755
	}
	return mode
}

func findJavaExecutable(root string) string {
	direct := filepath.Join(root, "bin", javaExe())
	if st, err := os.Stat(direct); err == nil && !st.IsDir() {
		return direct
	}
	var found string
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || found != "" {
			return nil
		}
		if d.IsDir() {
			rel, _ := filepath.Rel(root, p)
			if rel != "." && strings.Count(rel, string(filepath.Separator)) >= 4 {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() == javaExe() && strings.EqualFold(filepath.Base(filepath.Dir(p)), "bin") {
			found = p
			return filepath.SkipAll
		}
		return nil
	})
	return found
}
