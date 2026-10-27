package java

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Info struct {
	Path    string
	Version string
	Major   int
	Vendor  string
	Source  string
}

func (i Info) Label() string {
	vendor := ""
	if i.Vendor != "" {
		vendor = " · " + i.Vendor
	}
	return "Java " + i.Version + vendor
}

func (i Info) Short() string {
	vendor := ""
	if i.Vendor != "" {
		vendor = " " + i.Vendor
	}
	return fmt.Sprintf("Java %d%s", i.Major, vendor)
}

var versionRE = regexp.MustCompile(`version "([^"]+)"`)

func majorOf(version string) int {
	version = strings.TrimSpace(version)
	if strings.HasPrefix(version, "1.") {
		parts := strings.Split(version, ".")
		if len(parts) > 1 {
			if n, err := strconv.Atoi(parts[1]); err == nil {
				return n
			}
		}
	}
	re := regexp.MustCompile(`^(\d+)`)
	if m := re.FindStringSubmatch(version); len(m) == 2 {
		if n, err := strconv.Atoi(m[1]); err == nil {
			return n
		}
	}
	return 0
}

func vendorOf(text string) string {
	lower := strings.ToLower(text)
	for _, pair := range [][2]string{
		{"temurin", "Temurin"},
		{"adoptium", "Adoptium"},
		{"microsoft", "Microsoft"},
		{"corretto", "Corretto"},
		{"zulu", "Zulu"},
		{"bellsoft", "BellSoft"},
		{"liberica", "Liberica"},
		{"graalvm", "GraalVM"},
		{"oracle", "Oracle"},
		{"openjdk", "OpenJDK"},
	} {
		if strings.Contains(lower, pair[0]) {
			return pair[1]
		}
	}
	return ""
}

func detect(path, source string) *Info {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "-version")
	hideWindow(cmd)
	out, _ := cmd.CombinedOutput()
	text := string(out)
	version := ""
	if m := versionRE.FindStringSubmatch(text); len(m) == 2 {
		version = m[1]
	}
	vendor := vendorOf(text)
	if version == "" {
		release := filepath.Join(filepath.Dir(filepath.Dir(path)), "release")
		if data, err := os.ReadFile(release); err == nil {
			content := string(data)
			if m := regexp.MustCompile(`(?m)^JAVA_VERSION="([^"]+)"`).FindStringSubmatch(content); len(m) == 2 {
				version = m[1]
			}
			if vendor == "" {
				if m := regexp.MustCompile(`(?m)^IMPLEMENTOR="([^"]*)"`).FindStringSubmatch(content); len(m) == 2 {
					vendor = m[1]
				}
			}
		}
	}
	if version == "" {
		return nil
	}
	major := majorOf(version)
	if major <= 0 {
		return nil
	}
	return &Info{Path: path, Version: version, Major: major, Vendor: vendor, Source: source}
}

func Detect(path string) *Info {
	return detect(path, "manual")
}

type candidate struct {
	path   string
	source string
}

func javaExe() string {
	if runtime.GOOS == "windows" {
		return "java.exe"
	}
	return "java"
}

func addPath(out *[]candidate, path, source string) {
	if path == "" {
		return
	}
	p := path
	if st, err := os.Stat(p); err == nil && st.IsDir() {
		p = filepath.Join(p, "bin", javaExe())
	}
	if st, err := os.Stat(p); err == nil && !st.IsDir() {
		*out = append(*out, candidate{p, source})
	}
}

func globCandidates(patterns []string, source string, out *[]candidate) {
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		for _, m := range matches {
			if st, err := os.Stat(m); err == nil && st.IsDir() {
				addPath(out, m, source)
			}
		}
	}
}

func walkCandidates(root string, maxDepth int, source string, out *[]candidate) {
	root = filepath.Clean(root)
	if st, err := os.Stat(root); err != nil || !st.IsDir() {
		return
	}
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, relErr := filepath.Rel(root, p)
		if relErr != nil {
			return nil
		}
		depth := 0
		if rel != "." {
			depth = strings.Count(rel, string(filepath.Separator)) + 1
		}
		if d.IsDir() {
			if depth >= maxDepth {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() == javaExe() && strings.EqualFold(filepath.Base(filepath.Dir(p)), "bin") {
			*out = append(*out, candidate{p, source})
		}
		return nil
	})
}

func Candidates() []candidate {
	var out []candidate
	addPath(&out, os.Getenv("JAVA_HOME"), "JAVA_HOME")
	if p, err := exec.LookPath(javaExe()); err == nil {
		addPath(&out, p, "PATH")
	}
	switch runtime.GOOS {
	case "windows":
		programFiles := []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)")}
		local := os.Getenv("LOCALAPPDATA")
		roaming := os.Getenv("APPDATA")
		vendors := []string{"Java", "Eclipse Adoptium", "Microsoft", "Amazon Corretto", "Zulu", "BellSoft", "Semeru"}
		for _, pf := range programFiles {
			if pf == "" {
				continue
			}
			for _, vendor := range vendors {
				globCandidates([]string{filepath.Join(pf, vendor, "*", "bin", javaExe())}, vendor, &out)
			}
			walkCandidates(filepath.Join(pf, "Minecraft Launcher", "runtime"), 5, "Minecraft Launcher", &out)
		}
		if local != "" {
			for _, vendor := range []string{"Eclipse Adoptium", "Microsoft", "Amazon Corretto"} {
				globCandidates([]string{filepath.Join(local, "Programs", vendor, "*", "bin", javaExe())}, "Local Programs", &out)
			}
			globCandidates([]string{filepath.Join(local, "Programs", "Microsoft", "jdk-*", "bin", javaExe())}, "Local Programs", &out)
			storeRoot := filepath.Join(local, "Packages")
			globCandidates([]string{filepath.Join(storeRoot, "Microsoft.4297127D64EC6_*", "LocalCache", "Local", "runtime")}, "Minecraft Store", &out)
			packages, _ := filepath.Glob(filepath.Join(storeRoot, "Microsoft.4297127D64EC6_*"))
			for _, pkg := range packages {
				walkCandidates(filepath.Join(pkg, "LocalCache", "Local", "runtime"), 5, "Minecraft Store", &out)
			}
		}
		if roaming != "" {
			walkCandidates(filepath.Join(roaming, ".minecraft", "runtime"), 5, "Minecraft", &out)
		}
		home, _ := os.UserHomeDir()
		if home != "" {
			walkCandidates(filepath.Join(home, ".jdks"), 3, "IntelliJ", &out)
			walkCandidates(filepath.Join(home, ".gradle", "jdks"), 4, "Gradle", &out)
			globCandidates([]string{filepath.Join(home, "scoop", "apps", "openjdk*", "current", "bin", javaExe())}, "Scoop", &out)
			globCandidates([]string{filepath.Join(home, "scoop", "apps", "temurin*", "current", "bin", javaExe())}, "Scoop", &out)
		}
	case "darwin":
		globCandidates([]string{
			"/Library/Java/JavaVirtualMachines/*/Contents/Home/bin/java",
			"/System/Library/Java/JavaVirtualMachines/*/Contents/Home/bin/java",
			"/opt/homebrew/opt/openjdk*/bin/java",
			"/usr/local/opt/openjdk*/bin/java",
		}, "System", &out)
		home, _ := os.UserHomeDir()
		globCandidates([]string{filepath.Join(home, "Library", "Java", "JavaVirtualMachines", "*", "Contents", "Home", "bin", "java")}, "User", &out)
		walkCandidates(filepath.Join(home, ".gradle", "jdks"), 4, "Gradle", &out)
		walkCandidates(filepath.Join(home, "Library", "Application Support", "minecraft", "runtime"), 5, "Minecraft", &out)
	default:
		globCandidates([]string{
			"/usr/lib/jvm/*/bin/java",
			"/usr/java/*/bin/java",
			"/opt/openjdk*/bin/java",
			"/opt/java*/bin/java",
		}, "System", &out)
		home, _ := os.UserHomeDir()
		walkCandidates(filepath.Join(home, ".jdks"), 3, "IntelliJ", &out)
		walkCandidates(filepath.Join(home, ".gradle", "jdks"), 4, "Gradle", &out)
		globCandidates([]string{filepath.Join(home, ".sdkman", "candidates", "java", "*", "bin", "java")}, "SDKMAN", &out)
		walkCandidates(filepath.Join(home, ".minecraft", "runtime"), 5, "Minecraft", &out)
		walkCandidates(filepath.Join(home, ".local", "share", "PrismLauncher", "java"), 3, "Prism", &out)
	}
	return out
}

func Scan() []Info {
	seen := map[string]bool{}
	var results []Info
	for _, c := range Candidates() {
		key := filepath.Clean(c.path)
		if runtime.GOOS == "windows" {
			key = strings.ToLower(key)
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		if info := detect(c.path, c.source); info != nil {
			results = append(results, *info)
		}
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Major != results[j].Major {
			return results[i].Major > results[j].Major
		}
		return compareVersions(results[i].Version, results[j].Version) > 0
	})
	return results
}

func compareVersions(a, b string) int {
	ra := regexp.MustCompile(`\d+`).FindAllString(a, -1)
	rb := regexp.MustCompile(`\d+`).FindAllString(b, -1)
	for i := 0; i < len(ra) && i < len(rb); i++ {
		x, _ := strconv.Atoi(ra[i])
		y, _ := strconv.Atoi(rb[i])
		if x != y {
			if x > y {
				return 1
			}
			return -1
		}
	}
	if len(ra) > len(rb) {
		return 1
	}
	if len(ra) < len(rb) {
		return -1
	}
	return 0
}

// Select chooses a Java runtime for the required major version. Explicit
// overrides win, but are reported as a warning when they are older than the
// version metadata asks for.
func Select(runtimes []Info, required int, override string) (Info, string, error) {
	if required < 8 {
		required = 8
	}
	if strings.TrimSpace(override) != "" {
		path := strings.TrimSpace(override)
		if info := Detect(path); info != nil {
			warning := ""
			if info.Major < required {
				warning = fmt.Sprintf("Java %d is older than the recommended Java %d", info.Major, required)
			}
			return *info, warning, nil
		}
		return Info{}, "", fmt.Errorf("no Java runtime at %s", path)
	}
	if len(runtimes) == 0 {
		return Info{}, "", fmt.Errorf("no Java runtime found — install Java %d (Temurin) and rescan in Settings", required)
	}
	if required <= 8 {
		for _, rt := range runtimes {
			if rt.Major == 8 {
				return rt, "", nil
			}
		}
		return Info{}, "", fmt.Errorf("this version needs Java 8, but no Java 8 runtime was found")
	}
	var above []Info
	for _, rt := range runtimes {
		if rt.Major == required {
			return rt, "", nil
		}
		if rt.Major > required {
			above = append(above, rt)
		}
	}
	if len(above) > 0 {
		sort.Slice(above, func(i, j int) bool {
			if above[i].Major != above[j].Major {
				return above[i].Major < above[j].Major
			}
			return compareVersions(above[i].Version, above[j].Version) > 0
		})
		return above[0], fmt.Sprintf("using Java %d (version asks for %d)", above[0].Major, required), nil
	}
	return Info{}, "", fmt.Errorf("this version needs Java %d or newer, but no compatible runtime was found", required)
}
