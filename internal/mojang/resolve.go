package mojang

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/MoonCC233/MCLauncher-TUI/internal/paths"
	"github.com/MoonCC233/MCLauncher-TUI/internal/util"
)

type Artifact struct {
	Name string
	Path string // maven-relative path
	URL  string
	SHA1 string
	Size int64
}

type NativeJar struct {
	Name    string
	Path    string
	URL     string
	SHA1    string
	Size    int64
	Exclude []string
}

type LibraryResolution struct {
	Artifacts []Artifact
	Natives   []NativeJar
	Classpath []string // relative paths, in version-json order
	Warnings  []string
}

func archBits() string {
	if util.OSArch() == "x86" {
		return "32"
	}
	return "64"
}

func ResolveLibraries(v *VersionJSON, features map[string]bool) *LibraryResolution {
	res := &LibraryResolution{}
	seenArtifact := map[string]bool{}
	seenNative := map[string]bool{}
	seenCP := map[string]bool{}
	osName := util.OSName()

	addArtifact := func(name string, info DownloadInfo) {
		if info.Path == "" {
			return
		}
		rel := strings.ReplaceAll(info.Path, "\\", "/")
		if seenArtifact[rel] {
			return
		}
		seenArtifact[rel] = true
		res.Artifacts = append(res.Artifacts, Artifact{
			Name: name,
			Path: rel,
			URL:  info.URL,
			SHA1: info.SHA1,
			Size: info.Size,
		})
		if !seenCP[rel] {
			seenCP[rel] = true
			res.Classpath = append(res.Classpath, rel)
		}
	}
	addNative := func(name string, info DownloadInfo, exclude []string) {
		if info.Path == "" {
			return
		}
		rel := strings.ReplaceAll(info.Path, "\\", "/")
		if seenNative[rel] {
			return
		}
		seenNative[rel] = true
		res.Natives = append(res.Natives, NativeJar{
			Name:    name,
			Path:    rel,
			URL:     info.URL,
			SHA1:    info.SHA1,
			Size:    info.Size,
			Exclude: append([]string(nil), exclude...),
		})
	}

	for _, lib := range v.Libraries {
		if !Allowed(lib.Rules, features) {
			continue
		}
		var exclude []string
		if lib.Extract != nil {
			exclude = lib.Extract.Exclude
		}

		regular := lib.Downloads.Artifact
		hasRegular := regular.Path != ""
		if !hasRegular && len(strings.Split(lib.Name, ":")) >= 3 {
			coords := strings.Split(lib.Name, ":")
			classifier := ""
			if len(coords) > 3 {
				classifier = coords[3]
			}
			path := MavenPath(lib.Name)
			if path == "" {
				path = func() string {
					file := coords[1] + "-" + coords[2]
					if classifier != "" {
						file += "-" + classifier
					}
					return strings.ReplaceAll(coords[0], ".", "/") + "/" + coords[1] + "/" + coords[2] + "/" + file + ".jar"
				}()
			}
			regular.Path = path
			hasRegular = true
		}
		if hasRegular {
			if regular.URL == "" && !lib.Local {
				base := lib.URL
				if base == "" {
					base = "https://libraries.minecraft.net/"
				}
				regular.URL = strings.TrimRight(base, "/") + "/" + regular.Path
			}
			if regular.SHA1 == "" {
				regular.SHA1 = lib.SHA1
			}
			if regular.Size <= 0 {
				regular.Size = lib.Size
			}
			addArtifact(lib.Name, regular)
		}

		// Legacy natives: downloads.classifiers["natives-windows"] (or similar).
		if classifierTemplate, ok := lib.Natives[osName]; ok {
			classifier := strings.ReplaceAll(classifierTemplate, "${arch}", archBits())
			if info, exists := lib.Downloads.Classifiers[classifier]; exists {
				if info.URL == "" && info.Path != "" {
					base := lib.URL
					if base == "" {
						base = "https://libraries.minecraft.net/"
					}
					info.URL = strings.TrimRight(base, "/") + "/" + info.Path
				}
				addNative(lib.Name, info, exclude)
			} else if hasRegular && strings.Contains(lib.Name, "natives") {
				// Rare: natives map points at the artifact itself.
				addNative(lib.Name, regular, exclude)
			}
		}

		// Modern natives are separate Maven entries such as
		// org.lwjgl:lwjgl:3.3.2:natives-windows. Keep them on the classpath
		// and extract their shared objects as well.
		if strings.Contains(lib.Name, ":natives-") && hasRegular {
			addNative(lib.Name, regular, exclude)
		}
	}
	return res
}

func ArtifactPath(layout paths.Layout, a Artifact) string {
	return filepath.Join(layout.Libraries, filepath.FromSlash(a.Path))
}

func NativePath(layout paths.Layout, n NativeJar) string {
	return filepath.Join(layout.Libraries, filepath.FromSlash(n.Path))
}

func Classpath(layout paths.Layout, res *LibraryResolution, clientJar string) string {
	parts := make([]string, 0, len(res.Classpath)+1)
	for _, rel := range res.Classpath {
		parts = append(parts, filepath.Join(layout.Libraries, filepath.FromSlash(rel)))
	}
	if clientJar != "" {
		parts = append(parts, clientJar)
	}
	return strings.Join(parts, util.ClasspathSeparator())
}

func (res *LibraryResolution) Validate() error {
	for _, rel := range res.Classpath {
		if !strings.HasSuffix(strings.ToLower(rel), ".jar") {
			return fmt.Errorf("unexpected classpath entry %q", rel)
		}
	}
	return nil
}
