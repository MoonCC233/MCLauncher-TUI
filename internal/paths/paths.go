package paths

import (
	"os"
	"path/filepath"
	"runtime"
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
func (l Layout) InstanceDir(instanceID string) string {
	return filepath.Join(l.Instances, instanceID)
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

// DefaultRoot resolves the per-user data directory, honouring
// MCLAUNCHER_DATA_DIR for portable installs.
func DefaultRoot() (string, error) {
	if override := os.Getenv("MCLAUNCHER_DATA_DIR"); override != "" {
		abs, err := filepath.Abs(override)
		if err != nil {
			return "", err
		}
		return abs, nil
	}
	switch runtime.GOOS {
	case "windows":
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "mclauncher"), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "AppData", "Roaming", "mclauncher"), nil
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support", "mclauncher"), nil
	default:
		if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
			return filepath.Join(xdg, "mclauncher"), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".local", "share", "mclauncher"), nil
	}
}
