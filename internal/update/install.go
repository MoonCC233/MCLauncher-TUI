package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
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

// DownloadAndInstall downloads the release asset, verifies its checksum,
// extracts the launcher, and replaces the running executable. The caller must
// restart the launcher afterwards.
func DownloadAndInstall(
	ctx context.Context,
	client *http.Client,
	release *Release,
	asset *Asset,
	progress func(Progress),
) error {
	if release == nil || asset == nil {
		return fmt.Errorf("missing update release or asset")
	}
	if client == nil {
		client = util.NewHTTPClient()
	}
	checksum := checksumAsset(release)
	if checksum == nil {
		return fmt.Errorf("release %s has no checksums.txt", release.TagName)
	}
	sumText, err := fetchText(ctx, client, checksum.URL, 1<<20)
	if err != nil {
		return fmt.Errorf("download checksums: %w", err)
	}
	want, err := checksumFor(sumText, asset.Name)
	if err != nil {
		return err
	}

	tempDir, err := os.MkdirTemp("", "mclauncher-update-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)
	archivePath := filepath.Join(tempDir, asset.Name)

	report(progress, Progress{Stage: "Downloading update", Detail: asset.Name, Total: asset.Size})
	if err := downloadFile(ctx, client, asset.URL, archivePath, asset.Size, want, func(done int64) {
		report(progress, Progress{Stage: "Downloading update", Detail: asset.Name, Done: done, Total: asset.Size})
	}); err != nil {
		return fmt.Errorf("download update: %w", err)
	}

	report(progress, Progress{Stage: "Installing update", Detail: filepath.Base(asset.Name)})
	binary, err := extractBinary(archivePath, tempDir, runtime.GOOS)
	if err != nil {
		return fmt.Errorf("extract update: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := installBinary(binary); err != nil {
		return fmt.Errorf("install update: %w", err)
	}
	report(progress, Progress{Stage: "Update installed", Detail: release.TagName})
	return nil
}

func report(progress func(Progress), value Progress) {
	if progress != nil {
		progress(value)
	}
}

func downloadFile(
	ctx context.Context,
	client *http.Client,
	rawURL, dest string,
	size int64,
	wantSHA256 string,
	onProgress func(int64),
) error {
	if client == nil {
		client = util.NewHTTPClient()
	}
	var lastErr error
	for _, url := range downloadCandidates(rawURL) {
		attemptCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		err := util.DownloadFile(attemptCtx, client, util.DownloadSpec{
			URL:  url,
			Dest: dest,
			Size: size,
		}, onProgress)
		cancel()
		if err != nil {
			lastErr = err
			continue
		}
		if wantSHA256 != "" {
			got, err := sha256File(dest)
			if err != nil {
				lastErr = err
				_ = os.Remove(dest)
				continue
			}
			if !strings.EqualFold(got, wantSHA256) {
				lastErr = fmt.Errorf("checksum mismatch for %s", filepath.Base(dest))
				_ = os.Remove(dest)
				continue
			}
		}
		return nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no download source is available")
	}
	return lastErr
}

func fetchText(ctx context.Context, client *http.Client, rawURL string, limit int64) (string, error) {
	if client == nil {
		client = util.NewHTTPClient()
	}
	var lastErr error
	for _, url := range downloadCandidates(rawURL) {
		attemptCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		req, err := http.NewRequestWithContext(attemptCtx, http.MethodGet, url, nil)
		if err != nil {
			cancel()
			lastErr = err
			continue
		}
		req.Header.Set("User-Agent", "mclauncher-updater")
		resp, err := client.Do(req)
		if err != nil {
			cancel()
			lastErr = err
			continue
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			cancel()
			lastErr = fmt.Errorf("GET %s: HTTP %d", url, resp.StatusCode)
			continue
		}
		data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
		resp.Body.Close()
		cancel()
		if err != nil {
			lastErr = err
			continue
		}
		if int64(len(data)) > limit {
			lastErr = fmt.Errorf("%s is too large", filepath.Base(rawURL))
			continue
		}
		return string(data), nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no download source is available")
	}
	return "", lastErr
}

func checksumFor(text, name string) (string, error) {
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		file := strings.TrimPrefix(fields[1], "*")
		if file != name {
			continue
		}
		sum := strings.ToLower(fields[0])
		if len(sum) == 64 {
			return sum, nil
		}
	}
	return "", fmt.Errorf("checksums.txt has no SHA-256 for %s", name)
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

func extractBinary(archivePath, destDir, goos string) (string, error) {
	name := "mclauncher"
	if goos == "windows" {
		name = "mclauncher.exe"
	}
	dest := filepath.Join(destDir, name)
	var err error
	switch {
	case strings.HasSuffix(strings.ToLower(archivePath), ".zip"):
		err = extractZip(archivePath, name, dest)
	case strings.HasSuffix(strings.ToLower(archivePath), ".tar.gz"),
		strings.HasSuffix(strings.ToLower(archivePath), ".tgz"):
		err = extractTarGz(archivePath, name, dest)
	default:
		err = fmt.Errorf("unsupported archive %s", filepath.Base(archivePath))
	}
	if err != nil {
		return "", err
	}
	info, err := os.Stat(dest)
	if err != nil || info.Size() == 0 {
		return "", fmt.Errorf("archive does not contain %s", name)
	}
	if err := os.Chmod(dest, 0o755); err != nil {
		return "", err
	}
	return dest, nil
}

func extractZip(archivePath, want, dest string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()
	for _, file := range reader.File {
		if file.FileInfo().IsDir() || entryBase(file.Name) != want {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return err
		}
		err = writeBinary(dest, rc)
		rc.Close()
		return err
	}
	return fmt.Errorf("%s not found in %s", want, filepath.Base(archivePath))
}

func extractTarGz(archivePath, want, dest string) error {
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
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if header.Typeflag != tar.TypeReg || entryBase(header.Name) != want {
			continue
		}
		return writeBinary(dest, reader)
	}
	return fmt.Errorf("%s not found in %s", want, filepath.Base(archivePath))
}

func entryBase(name string) string {
	return path.Base(strings.ReplaceAll(name, "\\", "/"))
}

func writeBinary(dest string, src io.Reader) error {
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	written, copyErr := io.Copy(f, io.LimitReader(src, 512<<20))
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(dest)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(dest)
		return closeErr
	}
	if written == 0 {
		_ = os.Remove(dest)
		return fmt.Errorf("empty archive entry")
	}
	return nil
}

func installBinary(newBinary string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return installBinaryAt(exe, newBinary)
}

func installBinaryAt(exe, newBinary string) error {
	dir := filepath.Dir(exe)
	tmp, err := os.CreateTemp(dir, ".mclauncher-update-*")
	if err != nil {
		return fmt.Errorf("create update file next to %s: %w", exe, err)
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()
	src, err := os.Open(newBinary)
	if err != nil {
		tmp.Close()
		return err
	}
	_, copyErr := io.Copy(tmp, src)
	src.Close()
	closeErr := tmp.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if err := os.Chmod(tmpName, 0o755); err != nil {
		return err
	}

	if runtime.GOOS != "windows" {
		if err := os.Rename(tmpName, exe); err != nil {
			return err
		}
		cleanup = false
		return nil
	}

	// Windows cannot overwrite a running executable, but moving it aside is
	// allowed. The launcher continues from the .old file until it exits; the
	// next start removes that file.
	old := exe + ".old"
	_ = os.Remove(old)
	if err := os.Rename(exe, old); err != nil {
		return fmt.Errorf("move running launcher aside: %w", err)
	}
	if err := os.Rename(tmpName, exe); err != nil {
		_ = os.Rename(old, exe)
		return fmt.Errorf("write new launcher: %w", err)
	}
	cleanup = false
	_ = os.Remove(old)
	return nil
}

// CleanupOldBinary removes a .old launcher left by a previous Windows update.
func CleanupOldBinary() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	_ = os.Remove(exe + ".old")
}
