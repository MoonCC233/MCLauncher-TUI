// Package packs installs, imports and toggles instance content: resource packs,
// shader packs and mods.
package packs

import (
	"context"
	"crypto/sha1"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/MoonCC233/MCLauncher-TUI/internal/mirror"
	"github.com/MoonCC233/MCLauncher-TUI/internal/util"
)

const (
	// DirName is the folder Minecraft reads resource packs from.
	DirName = "resourcepacks"

	manifestName = ".mclauncher-packs.json"
	optionsName  = "options.txt"
	disabledExt  = ".disabled"
)

// Kind is the type of content a folder holds.
type Kind string

const (
	// ResourcePack lives in resourcepacks/ and is enabled through options.txt.
	ResourcePack Kind = "resourcepack"
	// ShaderPack lives in shaderpacks/ and is chosen in game by Iris or OptiFine.
	ShaderPack Kind = "shader"
	// Mod lives in mods/ and is enabled by dropping the .disabled suffix.
	Mod Kind = "mod"
)

// Kinds lists the selectable content kinds, in picker order.
var Kinds = []Kind{ResourcePack, ShaderPack, Mod}

// Folder is the directory name Minecraft uses for the kind.
func (k Kind) Folder() string {
	switch k {
	case ShaderPack:
		return "shaderpacks"
	case Mod:
		return "mods"
	default:
		return DirName
	}
}

// Valid reports whether the kind is one this package understands.
func (k Kind) Valid() bool {
	for _, candidate := range Kinds {
		if candidate == k {
			return true
		}
	}
	return false
}

// Extension is the file suffix Minecraft expects for the kind.
func (k Kind) Extension() string {
	if k == Mod {
		return ".jar"
	}
	return ".zip"
}

// Record is one installed item. The enabled state is not stored here: options.txt
// (resource packs) and the .disabled suffix (mods) own it, so the game and the
// launcher cannot disagree.
type Record struct {
	ProjectID   string `json:"project_id,omitempty"`
	VersionID   string `json:"version_id,omitempty"`
	Title       string `json:"title"`
	Filename    string `json:"filename"`
	SHA512      string `json:"sha512,omitempty"`
	SHA1        string `json:"sha1,omitempty"`
	Size        int64  `json:"size,omitempty"`
	InstalledAt int64  `json:"installed_at"`
	ContentType string `json:"content_type,omitempty"`
	Source      string `json:"source,omitempty"`
}

// Manifest is the sidecar file listing what the launcher installed.
type Manifest struct {
	Packs []Record `json:"packs"`
}

// Dir returns the resource pack folder of an instance.
func Dir(instanceDir string) string { return DirFor(instanceDir, ResourcePack) }

// DirFor returns the folder holding the given kind for an instance.
func DirFor(instanceDir string, kind Kind) string {
	return filepath.Join(instanceDir, kind.Folder())
}

// OptionsPath returns the options.txt the game writes in its game directory.
func OptionsPath(instanceDir string) string { return filepath.Join(instanceDir, optionsName) }

// List returns the items found in the folder, newest first. Files the launcher
// did not install (copied in by hand or downloaded elsewhere) are listed too.
func List(instanceDir string, kind Kind) []Record {
	dir := DirFor(instanceDir, kind)
	manifest, _ := readManifest(instanceDir, kind)
	byName := make(map[string]Record, len(manifest.Packs))
	for _, record := range manifest.Packs {
		byName[strings.ToLower(record.Filename)] = record
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	out := make([]Record, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".") || !kind.matches(name) {
			continue
		}
		record, ok := byName[strings.ToLower(strings.TrimSuffix(name, disabledExt))]
		if !ok {
			record = Record{Filename: name, Title: displayName(name)}
		} else {
			record.Filename = name
		}
		if info, err := entry.Info(); err == nil {
			if record.Size == 0 {
				record.Size = info.Size()
			}
			if record.InstalledAt == 0 {
				record.InstalledAt = info.ModTime().Unix()
			}
		}
		out = append(out, record)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].InstalledAt != out[j].InstalledAt {
			return out[i].InstalledAt > out[j].InstalledAt
		}
		return strings.ToLower(out[i].Title) < strings.ToLower(out[j].Title)
	})
	return out
}

// matches reports whether a file name belongs to the kind: mods are .jar files,
// resource packs and shaders are .zip files, and a disabled mod keeps the suffix.
func (k Kind) matches(name string) bool {
	lower := strings.ToLower(strings.TrimSuffix(name, disabledExt))
	return strings.HasSuffix(lower, k.Extension())
}

func displayName(filename string) string {
	name := strings.TrimSuffix(filename, disabledExt)
	return strings.TrimSuffix(name, filepath.Ext(name))
}

// SanitizeFilename keeps the filename usable on every platform and inside the
// content folder.
func SanitizeFilename(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	name = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		case r == '.', r == '-', r == '_', r == ' ':
			return r
		default:
			return '-'
		}
	}, name)
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." {
		name = "pack"
	}
	return name
}

// validatePackURL is a variable so tests can talk to a local server.
var validatePackURL = ValidateURL

// packHostsAllowed reports whether a download host is trusted for catalog
// installs. Players can still add any https URL by hand through InstallURL.
var packHostsAllowed = func(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	switch {
	case host == "cdn.modrinth.com", strings.HasSuffix(host, ".modrinth.com"):
		return true
	case host == "edge.forgecdn.net", host == "mediafilez.forgecdn.net":
		return true
	case strings.HasSuffix(host, ".forgecdn.net"):
		return true
	case host == "mod.mcimirror.top", strings.HasSuffix(host, ".mcimirror.top"):
		return true
	}
	return false
}

// ValidateURL checks that a catalog download uses https on a trusted host.
func ValidateURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("invalid pack url: %w", err)
	}
	if parsed.Scheme != "https" {
		return fmt.Errorf("pack url must use https")
	}
	if !packHostsAllowed(parsed.Hostname()) {
		return fmt.Errorf("untrusted pack host %q", parsed.Hostname())
	}
	return nil
}

// contentMirror maps the content CDN hosts onto the keyless read-only mirror.
// Both families keep the path unchanged, so only the host has to be replaced.
const contentMirror = "https://mod.mcimirror.top"

// candidateURLs lists the places a content file can be downloaded from. The
// official CDN is tried first unless the player enabled the download mirror in
// Settings, in which case the mirror is preferred and the CDN stays as a
// fallback. URLs on other hosts are returned unchanged.
func candidateURLs(raw string) []string {
	raw = strings.TrimSpace(raw)
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return []string{raw}
	}
	host := strings.ToLower(parsed.Hostname())
	switch host {
	case "cdn.modrinth.com", "edge.forgecdn.net", "mediafilez.forgecdn.net":
	default:
		return []string{raw}
	}
	mirrored := contentMirror + parsed.EscapedPath()
	if mirrored == raw {
		return []string{raw}
	}
	if mirror.Enabled() {
		return []string{mirrored, raw}
	}
	return []string{raw, mirrored}
}

// Install downloads a catalog file into the instance folder. When a SHA-512 or
// SHA-1 is provided the file is verified before it becomes visible to the game.
func Install(ctx context.Context, client *http.Client, instanceDir string, kind Kind, record Record, rawURL string, onProgress func(int64)) (Record, error) {
	if err := validatePackURL(rawURL); err != nil {
		return Record{}, err
	}
	return InstallURL(ctx, client, instanceDir, kind, record, rawURL, onProgress)
}

// InstallURL downloads any https URL the player pasted. It is the entry point for
// content hosted outside Modrinth and CurseForge, so it only requires https and a
// sane name. Known content CDNs additionally get a mirror fallback.
func InstallURL(ctx context.Context, client *http.Client, instanceDir string, kind Kind, record Record, rawURL string, onProgress func(int64)) (Record, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return Record{}, fmt.Errorf("pack url must be an https link")
	}
	if client == nil {
		client = util.NewHTTPClient()
	}
	filename := SanitizeFilename(record.Filename)
	if !strings.HasSuffix(strings.ToLower(filename), kind.Extension()) {
		filename += kind.Extension()
	}
	dir := DirFor(instanceDir, kind)
	if err := util.EnsureDir(dir); err != nil {
		return Record{}, err
	}
	dest := filepath.Join(dir, filename)

	// Reinstalling a file that is already on disk with the expected checksum
	// does not need the network.
	if alreadyInstalled(dest, record) {
		return finishRecord(instanceDir, kind, record, filename, dest)
	}

	temp := dest + ".download"
	urls := candidateURLs(rawURL)
	spec := util.DownloadSpec{
		URL:    urls[0],
		Dest:   temp,
		Size:   record.Size,
		SHA1:   record.SHA1,
		SHA512: record.SHA512,
	}
	if len(urls) > 1 {
		spec.Alt = urls[1:]
	}
	if err := util.DownloadFile(ctx, client, spec, onProgress); err != nil {
		_ = os.Remove(temp)
		_ = os.Remove(temp + ".part")
		return Record{}, err
	}
	if err := os.Rename(temp, dest); err != nil {
		_ = os.Remove(temp)
		return Record{}, err
	}
	return finishRecord(instanceDir, kind, record, filename, dest)
}

// alreadyInstalled reports whether dest already matches the record.
func alreadyInstalled(dest string, record Record) bool {
	info, err := os.Stat(dest)
	if err != nil || info.IsDir() {
		return false
	}
	if record.Size > 0 && info.Size() != record.Size {
		return false
	}
	if record.SHA512 != "" {
		sum, err := SHA512File(dest)
		return err == nil && strings.EqualFold(sum, record.SHA512)
	}
	if record.SHA1 != "" {
		sum, err := SHA1File(dest)
		return err == nil && strings.EqualFold(sum, record.SHA1)
	}
	// Without a checksum, an existing file of the expected size is reused.
	return record.Size > 0
}

func finishRecord(instanceDir string, kind Kind, record Record, filename, dest string) (Record, error) {
	record.Filename = filename
	record.ContentType = string(kind)
	record.InstalledAt = time.Now().Unix()
	if record.Title == "" {
		record.Title = displayName(filename)
	}
	if record.Size == 0 {
		if info, err := os.Stat(dest); err == nil {
			record.Size = info.Size()
		}
	}
	if err := upsertRecord(instanceDir, kind, record); err != nil {
		return Record{}, err
	}
	return record, nil
}

// ImportFile copies a file the player downloaded elsewhere into the instance
// folder and records where it came from.
func ImportFile(instanceDir string, kind Kind, source string) (Record, error) {
	info, err := os.Stat(source)
	if err != nil {
		return Record{}, err
	}
	if info.IsDir() {
		return Record{}, fmt.Errorf("%s is a directory", filepath.Base(source))
	}
	filename := SanitizeFilename(filepath.Base(source))
	if !strings.HasSuffix(strings.ToLower(filename), kind.Extension()) {
		return Record{}, fmt.Errorf("%s is not a %s file", filename, kind.Extension())
	}
	dir := DirFor(instanceDir, kind)
	if err := util.EnsureDir(dir); err != nil {
		return Record{}, err
	}
	dest := filepath.Join(dir, filename)
	if err := copyFile(source, dest); err != nil {
		return Record{}, err
	}
	record := Record{
		Title:       displayName(filename),
		Filename:    filename,
		Size:        info.Size(),
		InstalledAt: time.Now().Unix(),
		ContentType: string(kind),
		Source:      source,
	}
	if err := upsertRecord(instanceDir, kind, record); err != nil {
		return Record{}, err
	}
	return record, nil
}

// Remove deletes an item from disk, from the manifest and from the game state.
func Remove(instanceDir string, kind Kind, filename string) error {
	path := filepath.Join(DirFor(instanceDir, kind), filename)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := dropRecord(instanceDir, kind, strings.TrimSuffix(filename, disabledExt)); err != nil {
		return err
	}
	if kind == ResourcePack {
		_, err := SetEnabled(instanceDir, kind, filename, false)
		return err
	}
	return nil
}

// Enabled reports whether the item is currently active.
func Enabled(instanceDir string, kind Kind, filename string) bool {
	switch kind {
	case Mod:
		return !strings.HasSuffix(strings.ToLower(filename), disabledExt)
	case ShaderPack:
		// Shaders are selected inside the game, so presence means available.
		return true
	default:
		entries, err := readEnabledPacks(instanceDir)
		if err != nil {
			return false
		}
		want := packEntry(SanitizeFilename(strings.TrimSuffix(filename, disabledExt)))
		for _, entry := range entries {
			if strings.EqualFold(entry, want) {
				return true
			}
		}
		return false
	}
}

// SetEnabled switches an item on or off and reports whether anything changed.
func SetEnabled(instanceDir string, kind Kind, filename string, enabled bool) (bool, error) {
	filename = SanitizeFilename(strings.TrimSuffix(filename, disabledExt))
	if filename == "" {
		return false, fmt.Errorf("pack filename is empty")
	}
	if kind == Mod {
		return setModEnabled(instanceDir, filename, enabled)
	}
	if kind == ShaderPack {
		// Nothing to write: the shader is chosen in game.
		return false, nil
	}

	lines, err := readLines(OptionsPath(instanceDir))
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	entries, index := parseResourcePacks(lines)
	want := packEntry(filename)
	present := false
	filtered := make([]string, 0, len(entries)+1)
	for _, entry := range entries {
		if strings.EqualFold(entry, want) {
			present = true
			continue
		}
		filtered = append(filtered, entry)
	}
	if enabled && !present {
		filtered = append(filtered, want)
	}
	if present == enabled {
		return false, nil
	}

	encoded, err := json.Marshal(filtered)
	if err != nil {
		return false, err
	}
	line := "resourcePacks:" + string(encoded)
	if index >= 0 {
		lines[index] = line
	} else {
		lines = append(lines, line)
	}
	if err := util.EnsureDir(instanceDir); err != nil {
		return false, err
	}
	if err := os.WriteFile(OptionsPath(instanceDir), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		return false, err
	}
	return true, nil
}

func setModEnabled(instanceDir, filename string, enabled bool) (bool, error) {
	dir := DirFor(instanceDir, Mod)
	active := filepath.Join(dir, filename)
	disabled := active + disabledExt
	if enabled {
		if _, err := os.Stat(active); err == nil {
			return false, nil
		}
		if err := os.Rename(disabled, active); err != nil {
			return false, err
		}
		return true, nil
	}
	if _, err := os.Stat(disabled); err == nil {
		return false, nil
	}
	if err := os.Rename(active, disabled); err != nil {
		return false, err
	}
	return true, nil
}

// SHA512File returns the lowercase hex SHA-512 of a file.
func SHA512File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha512.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// SHA1File returns the lowercase hex SHA-1 of a file.
func SHA1File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha1.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func packEntry(filename string) string { return "file/" + filename }

var resourcePacksLine = regexp.MustCompile(`^resourcePacks\s*:`)

// parseResourcePacks returns the entries of the resourcePacks line and its index.
func parseResourcePacks(lines []string) ([]string, int) {
	for i, line := range lines {
		if !resourcePacksLine.MatchString(line) {
			continue
		}
		raw := strings.TrimSpace(line[strings.Index(line, ":")+1:])
		entries := []string{}
		if raw != "" {
			if err := json.Unmarshal([]byte(raw), &entries); err != nil {
				entries = nil
			}
		}
		return entries, i
	}
	return nil, -1
}

func readEnabledPacks(instanceDir string) ([]string, error) {
	lines, err := readLines(OptionsPath(instanceDir))
	if err != nil {
		return nil, err
	}
	entries, _ := parseResourcePacks(lines)
	return entries, nil
}

func readLines(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.TrimSuffix(text, "\n")
	if text == "" {
		return nil, nil
	}
	return strings.Split(text, "\n"), nil
}

func readManifest(instanceDir string, kind Kind) (Manifest, error) {
	var manifest Manifest
	err := util.ReadJSON(filepath.Join(DirFor(instanceDir, kind), manifestName), &manifest)
	if err != nil && !os.IsNotExist(err) {
		return Manifest{}, err
	}
	return manifest, nil
}

func writeManifest(instanceDir string, kind Kind, manifest Manifest) error {
	if err := util.EnsureDir(DirFor(instanceDir, kind)); err != nil {
		return err
	}
	return util.WriteJSONAtomic(filepath.Join(DirFor(instanceDir, kind), manifestName), manifest)
}

func upsertRecord(instanceDir string, kind Kind, record Record) error {
	manifest, err := readManifest(instanceDir, kind)
	if err != nil {
		return err
	}
	replaced := false
	for i := range manifest.Packs {
		if strings.EqualFold(manifest.Packs[i].Filename, record.Filename) {
			manifest.Packs[i] = record
			replaced = true
			break
		}
	}
	if !replaced {
		manifest.Packs = append(manifest.Packs, record)
	}
	return writeManifest(instanceDir, kind, manifest)
}

func dropRecord(instanceDir string, kind Kind, filename string) error {
	manifest, err := readManifest(instanceDir, kind)
	if err != nil {
		return err
	}
	kept := manifest.Packs[:0]
	for _, record := range manifest.Packs {
		if strings.EqualFold(record.Filename, filename) {
			continue
		}
		kept = append(kept, record)
	}
	manifest.Packs = kept
	return writeManifest(instanceDir, kind, manifest)
}

func copyFile(source, dest string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		_ = os.Remove(dest)
		return err
	}
	return out.Close()
}
