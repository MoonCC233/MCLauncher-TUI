package util

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const userAgent = "mclauncher/0.1.0 (+https://github.com/MoonCC233/MCLauncher-TUI)"

// downloadStallTimeout is how long a response body may make no progress before
// the attempt is cancelled. It is a variable so tests can shorten it.
var downloadStallTimeout = 45 * time.Second

func NewHTTPClient() *http.Client {
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          128,
		MaxIdleConnsPerHost:   32,
		IdleConnTimeout:       45 * time.Second,
		TLSHandshakeTimeout:   20 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	return &http.Client{Transport: transport}
}

type DownloadSpec struct {
	URL string
	// Alt holds further URLs to try when URL fails, for example the official
	// server after a mirror answered 404.
	Alt  []string
	Dest string
	SHA1 string
	Size int64
}

func DownloadFile(ctx context.Context, client *http.Client, spec DownloadSpec, onProgress func(int64)) error {
	if spec.URL == "" {
		return fmt.Errorf("missing download URL for %s", filepath.Base(spec.Dest))
	}
	if err := EnsureDir(filepath.Dir(spec.Dest)); err != nil {
		return err
	}
	part := spec.Dest + ".part"
	var lastErr error
	for _, url := range append([]string{spec.URL}, spec.Alt...) {
		if url == "" {
			continue
		}
		for attempt := 0; attempt < 3; attempt++ {
			if err := ctx.Err(); err != nil {
				return err
			}
			if onProgress != nil {
				onProgress(0)
			}
			if err := downloadOnce(ctx, client, spec, url, part, onProgress); err != nil {
				lastErr = err
				_ = os.Remove(part)
				// A missing file or a refused request is not worth retrying on the
				// same host; fall through to the next URL instead.
				if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "403") {
					break
				}
				if attempt < 2 {
					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-time.After(time.Duration(attempt+1) * 700 * time.Millisecond):
					}
				}
				continue
			}
			if err := os.Rename(part, spec.Dest); err != nil {
				_ = os.Remove(part)
				lastErr = err
				continue
			}
			return nil
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("download failed")
	}
	return lastErr
}

func downloadOnce(ctx context.Context, client *http.Client, spec DownloadSpec, url, part string, onProgress func(int64)) error {
	attemptCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(attemptCtx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Encoding", "identity")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: HTTP %d", url, resp.StatusCode)
	}
	// Some mirrors accept the connection and then stop sending data. Abort the
	// attempt when the body has made no progress for a while so DownloadFile
	// can retry instead of waiting for the full 15 minute deadline.
	stall := time.AfterFunc(downloadStallTimeout, func() {
		// Closing the body also unblocks a Read that is stuck in the HTTP
		// transport; cancelling alone is not always enough.
		_ = resp.Body.Close()
		cancel()
	})
	defer stall.Stop()
	f, err := os.OpenFile(part, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	success := false
	defer func() {
		_ = f.Close()
		if !success {
			_ = os.Remove(part)
		}
	}()
	h := sha1.New()
	var written int64
	buf := make([]byte, 128*1024)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			stall.Reset(downloadStallTimeout)
			if _, err := f.Write(buf[:n]); err != nil {
				return err
			}
			h.Write(buf[:n])
			written += int64(n)
			if onProgress != nil {
				onProgress(written)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("read %s: %w", spec.URL, readErr)
		}
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if spec.Size > 0 && written != spec.Size {
		return fmt.Errorf("size mismatch for %s: got %d, want %d", filepath.Base(spec.Dest), written, spec.Size)
	}
	if spec.SHA1 != "" {
		got := hex.EncodeToString(h.Sum(nil))
		if !strings.EqualFold(got, spec.SHA1) {
			return fmt.Errorf("checksum mismatch for %s", filepath.Base(spec.Dest))
		}
	}
	success = true
	return nil
}

func FetchJSON(ctx context.Context, client *http.Client, url string, out any) error {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := fetchJSONOnce(ctx, client, url, out)
		if err == nil {
			return nil
		}
		lastErr = err
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 600 * time.Millisecond):
		}
	}
	return lastErr
}

func fetchJSONOnce(ctx context.Context, client *http.Client, url string, out any) error {
	attemptCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(attemptCtx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: HTTP %d", url, resp.StatusCode)
	}
	limited := io.LimitReader(resp.Body, 64<<20)
	if err := json.NewDecoder(limited).Decode(out); err != nil {
		return fmt.Errorf("decode %s: %w", url, err)
	}
	return nil
}
