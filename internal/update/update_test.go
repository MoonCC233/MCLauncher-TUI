package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"v0.3.0", "0.2.1", 1},
		{"0.2.1", "v0.2.1", 0},
		{"0.3.0-rc.1", "0.3.0", -1},
		{"0.3.0-rc.2", "0.3.0-rc.1", 1},
		{"0.3.0 (abcdef)", "0.3.0", 0},
		{"1.0.0", "0.99.99", 1},
	}
	for _, tc := range cases {
		if got := CompareVersions(tc.a, tc.b); got != tc.want {
			t.Errorf("CompareVersions(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestIsDevelopment(t *testing.T) {
	for _, value := range []string{"", "dev", "dev-123", "dev (abcdef)", "unknown"} {
		if !IsDevelopment(value) {
			t.Errorf("IsDevelopment(%q) = false", value)
		}
	}
	for _, value := range []string{"0.2.1", "v0.3.0"} {
		if IsDevelopment(value) {
			t.Errorf("IsDevelopment(%q) = true", value)
		}
	}
}

func TestFindAsset(t *testing.T) {
	release := &Release{TagName: "v0.3.0", Assets: []Asset{
		{Name: "checksums.txt"},
		{Name: "mclauncher_0.3.0_windows_amd64.zip"},
		{Name: "mclauncher_0.3.0_linux_arm64.tar.gz"},
	}}
	asset, err := FindAsset(release, "windows", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if asset.Name != "mclauncher_0.3.0_windows_amd64.zip" {
		t.Errorf("asset = %q", asset.Name)
	}
	if _, err := FindAsset(release, "darwin", "amd64"); err == nil {
		t.Error("FindAsset should fail for a missing platform")
	}
}

func TestChecksumFor(t *testing.T) {
	sum := strings.Repeat("a", 64)
	text := "other  " + strings.Repeat("b", 64) + "\n" + sum + "  *mclauncher.zip\n"
	got, err := checksumFor(text, "mclauncher.zip")
	if err != nil {
		t.Fatal(err)
	}
	if got != sum {
		t.Errorf("checksum = %q, want %q", got, sum)
	}
	if _, err := checksumFor(text, "missing.zip"); err == nil {
		t.Error("checksumFor should fail for a missing file")
	}
}

func TestMirrorCandidates(t *testing.T) {
	t.Setenv("MCLAUNCHER_UPDATE_MIRROR", "https://mirror.test/{url}")
	urls := apiCandidates("https://api.github.com/repos/x/y/releases/latest")
	if len(urls) == 0 || urls[0] != "https://mirror.test/https://api.github.com/repos/x/y/releases/latest" {
		t.Errorf("api candidates = %#v", urls)
	}
	urls = downloadCandidates("https://github.com/x/y/releases/download/v1/file.zip")
	if len(urls) == 0 || urls[0] != "https://mirror.test/https://github.com/x/y/releases/download/v1/file.zip" {
		t.Errorf("download candidates = %#v", urls)
	}
}

func TestExtractZipBinary(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "update.zip")
	f, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("mclauncher.exe")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte("new-binary"))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()

	binary, err := extractBinary(archive, dir, "windows")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new-binary" {
		t.Errorf("extracted = %q", data)
	}
}

func TestExtractTarGzBinary(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "update.tar.gz")
	f, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	payload := []byte("tar-binary")
	if err := tw.WriteHeader(&tar.Header{Name: "mclauncher", Mode: 0o755, Size: int64(len(payload))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(payload); err != nil {
		t.Fatal(err)
	}
	tw.Close()
	gz.Close()
	f.Close()

	binary, err := extractBinary(archive, dir, "linux")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "tar-binary" {
		t.Errorf("extracted = %q", data)
	}
}

func TestInstallBinaryAt(t *testing.T) {
	dir := t.TempDir()
	current := filepath.Join(dir, "mclauncher")
	newBinary := filepath.Join(dir, "new-binary")
	if err := os.WriteFile(current, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newBinary, []byte("new"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := installBinaryAt(current, newBinary); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(current)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new" {
		t.Errorf("current binary = %q", data)
	}
}
