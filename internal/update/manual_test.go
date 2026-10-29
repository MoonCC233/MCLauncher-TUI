//go:build manual

package update

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/MoonCC233/MCLauncher-TUI/internal/util"
)

func TestManualUpdateDownload(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	client := util.NewHTTPClient()
	release, err := Check(ctx, client, "0.2.0")
	if err != nil {
		t.Fatal(err)
	}
	if release == nil {
		t.Fatal("no update found")
	}
	t.Logf("latest %s", release.TagName)
	asset, err := FindAsset(release, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	checksum := checksumAsset(release)
	sumText, err := fetchText(ctx, client, checksum.URL, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	sum, err := checksumFor(sumText, asset.Name)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	archive := filepath.Join(dir, asset.Name)
	var last int64
	err = downloadFile(ctx, client, asset.URL, archive, asset.Size, sum, func(done int64) {
		last = done
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("downloaded %d bytes", last)
	if _, err := os.Stat(archive); err != nil {
		t.Fatal(err)
	}
	binary, err := extractBinary(archive, dir, runtime.GOOS)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(binary)
	if err != nil || info.Size() == 0 {
		t.Fatalf("extracted binary: %v", err)
	}
	t.Logf("extracted %s (%d bytes)", binary, info.Size())
}
