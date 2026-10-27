package mojang

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/MoonCC233/MCLauncher-TUI/internal/paths"
	"github.com/MoonCC233/MCLauncher-TUI/internal/util"
)

// ExtractNatives clears the natives directory and unpacks every native jar
// into it. Zip-slip attempts and metadata files are ignored.
func ExtractNatives(layout paths.Layout, versionID string, res *LibraryResolution) error {
	dir := layout.NativesDir(versionID)
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, native := range res.Natives {
		jar := NativePath(layout, native)
		reader, err := zip.OpenReader(jar)
		if err != nil {
			return fmt.Errorf("open native jar %s: %w", filepath.Base(jar), err)
		}
		err = func() error {
			defer reader.Close()
			for _, f := range reader.File {
				if f.FileInfo().IsDir() {
					continue
				}
				name := strings.ReplaceAll(f.Name, "\\", "/")
				if !safeZipName(name) {
					continue
				}
				if excluded(name, native.Exclude) {
					continue
				}
				target := filepath.Join(dir, filepath.FromSlash(name))
				if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
					return err
				}
				src, err := f.Open()
				if err != nil {
					return err
				}
				dst, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
				if err != nil {
					src.Close()
					return err
				}
				_, copyErr := io.Copy(dst, io.LimitReader(src, 512<<20))
				src.Close()
				closeErr := dst.Close()
				if copyErr != nil {
					return copyErr
				}
				if closeErr != nil {
					return closeErr
				}
			}
			return nil
		}()
		if err != nil {
			return fmt.Errorf("extract %s: %w", filepath.Base(jar), err)
		}
	}
	return nil
}

func safeZipName(name string) bool {
	if name == "" {
		return false
	}
	clean := path.Clean(name)
	if clean == "." || strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "/") || strings.Contains(clean, ":") {
		return false
	}
	return true
}

func excluded(name string, patterns []string) bool {
	lower := strings.ToLower(name)
	if strings.HasPrefix(lower, "meta-inf/") {
		return true
	}
	for _, p := range patterns {
		p = strings.TrimSuffix(strings.ReplaceAll(p, "\\", "/"), "/")
		if p == "" {
			continue
		}
		if strings.HasPrefix(lower, strings.ToLower(p)+"/") || lower == strings.ToLower(p) {
			return true
		}
	}
	return false
}

// LegacyAssetPath resolves the path passed as ${game_assets} for old versions.
// For virtual indexes the installer creates a mirror under assets/virtual/<id>,
// while map_to_resources versions need assets copied into the game directory.
func LegacyAssetPath(layout paths.Layout, assetID string) string {
	virtual := layout.VirtualAssets(assetID)
	if st, err := os.Stat(filepath.Join(virtual, ".mclauncher-ok")); err == nil && !st.IsDir() {
		return virtual
	}
	if st, err := os.Stat(virtual); err == nil && st.IsDir() {
		return virtual
	}
	return layout.Assets
}

// ReadAssetIndex parses a downloaded asset index.
func ReadAssetIndex(layout paths.Layout, assetID string) (*AssetIndexFile, error) {
	var idx AssetIndexFile
	if err := util.ReadJSON(layout.AssetIndex(assetID), &idx); err != nil {
		return nil, err
	}
	return &idx, nil
}
