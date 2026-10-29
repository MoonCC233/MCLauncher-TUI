package paths

import (
	"os"
	"path/filepath"
	"strings"
)

// Layout describes every directory mclauncher owns inside its data root.
type Layout struct {
	Root       string
	ConfigFile string
	Versions   string
	Libraries  string
	Assets     string
	Indexes    string
	Objects    string
	Virtual    string
	LogConfigs string
	Instances  string
	Logs       string
	Cache      string
	Java       string
}

func New(root string) Layout {
	root = filepath.Clean(root)
	return Layout{
		Root:       root,
		ConfigFile: filepath.Join(root, "config.json"),
		Versions:   filepath.Join(root, "versions"),
		Libraries:  filepath.Join(root, "libraries"),
		Assets:     filepath.Join(root, "assets"),
		Indexes:    filepath.Join(root, "assets", "indexes"),
		Objects:    filepath.Join(root, "assets", "objects"),
		Virtual:    filepath.Join(root, "assets", "virtual"),
		LogConfigs: filepath.Join(root, "assets", "log_configs"),
		Instances:  filepath.Join(root, "instances"),
		Logs:       filepath.Join(root, "logs"),
		Cache:      filepath.Join(root, "cache"),
		Java:       filepath.Join(root, "java"),
	}
}

func (l Layout) Ensure() error {
	for _, dir := range []string{
		l.Root, l.Versions, l.Libraries, l.Assets, l.Indexes, l.Objects,
		l.Virtual, l.LogConfigs, l.Instances, l.Logs, l.Cache, l.Java,
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func (l Layout) VersionDir(id string) string  { return filepath.Join(l.Versions, id) }
func (l Layout) VersionJSON(id string) string { return filepath.Join(l.Versions, id, id+".json") }
func (l Layout) ClientJar(id string) string   { return filepath.Join(l.Versions, id, id+".jar") }
func (l Layout) NativesDir(id string) string  { return filepath.Join(l.Versions, id, "natives") }
func (l Layout) Marker(id string) string {
	return filepath.Join(l.Versions, id, ".mclauncher-installed.json")
}
func (l Layout) LoaderMeta(id string) string {
	return filepath.Join(l.Versions, id, ".mclauncher-loader.json")
}
func (l Layout) InstanceDir(instanceID string) string {
	return filepath.Join(l.Instances, instanceID)
}
func (l Layout) ModsDir(instanceID string) string {
	return filepath.Join(l.InstanceDir(instanceID), "mods")
}
func (l Layout) AssetIndex(id string) string {
	return filepath.Join(l.Indexes, id+".json")
}
func (l Layout) AssetObject(hash string) string {
	if len(hash) < 2 {
		return filepath.Join(l.Objects, hash)
	}
	return filepath.Join(l.Objects, hash[:2], hash)
}
func (l Layout) VirtualAssets(indexID string) string {
	return filepath.Join(l.Virtual, indexID)
}
func (l Layout) LauncherLog() string { return filepath.Join(l.Logs, "launcher.log") }
func (l Layout) ManifestCache() string {
	return filepath.Join(l.Cache, "version_manifest_v2.json")
}
func (l Layout) DownloadsHistory() string {
	return filepath.Join(l.Root, "downloads-history.json")
}

// DefaultRoot resolves the default data directory. Portable installs place
// everything in .minecraft next to the executable's working directory;
// MCLAUNCHER_DATA_DIR can still redirect it.
func DefaultRoot() (string, error) {
	if override := strings.TrimSpace(os.Getenv("MCLAUNCHER_DATA_DIR")); override != "" {
		return ResolveDataDir(override)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return "", err
	}
	return filepath.Join(abs, ".minecraft"), nil
}

// ResolveDataDir expands ~ and turns a relative path into an absolute one.
func ResolveDataDir(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return DefaultRoot()
	}
	if value == "~" || strings.HasPrefix(value, "~/") || strings.HasPrefix(value, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		rest := strings.TrimLeft(value[1:], `/\`)
		if rest == "" {
			value = home
		} else {
			value = filepath.Join(home, rest)
		}
	}
	abs, err := filepath.Abs(value)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}
