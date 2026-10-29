// Package mods inspects and manages the mods directory of one instance.
package mods

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ErrExists is returned by Add when the destination file already exists.
var ErrExists = errors.New("a mod with the same file name already exists")

type Mod struct {
	FileName    string   `json:"file_name"`
	Name        string   `json:"name"`
	Version     string   `json:"version,omitempty"`
	ID          string   `json:"id,omitempty"`
	Description string   `json:"description,omitempty"`
	Authors     string   `json:"authors,omitempty"`
	Loaders     []string `json:"loaders,omitempty"`
	Enabled     bool     `json:"enabled"`
}

func IsModFile(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".jar") || strings.HasSuffix(lower, ".jar.disabled")
}

func EnabledFileName(name string) string {
	return strings.TrimSuffix(name, ".disabled")
}

// Scan reads every jar in dir (including disabled ones) and extracts the
// metadata that Fabric, Quilt, Forge and legacy Forge mods use.
func Scan(dir string) ([]Mod, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]Mod, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !IsModFile(entry.Name()) {
			continue
		}
		mod := Mod{
			FileName: entry.Name(),
			Name:     displayName(entry.Name()),
			Enabled:  !strings.HasSuffix(strings.ToLower(entry.Name()), ".disabled"),
		}
		if info := readJarMeta(filepath.Join(dir, entry.Name())); info != nil {
			mod.merge(*info)
		}
		out = append(out, mod)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

func (m *Mod) merge(info jarMeta) {
	if info.Name != "" {
		m.Name = info.Name
	}
	if info.Version != "" {
		m.Version = info.Version
	}
	if info.ID != "" {
		m.ID = info.ID
	}
	if info.Description != "" {
		m.Description = info.Description
	}
	if info.Authors != "" {
		m.Authors = info.Authors
	}
	for _, loader := range info.Loaders {
		found := false
		for _, existing := range m.Loaders {
			if existing == loader {
				found = true
				break
			}
		}
		if !found {
			m.Loaders = append(m.Loaders, loader)
		}
	}
}

type jarMeta struct {
	Name        string
	Version     string
	ID          string
	Description string
	Authors     string
	Loaders     []string
}

func readJarMeta(path string) *jarMeta {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return nil
	}
	defer reader.Close()
	meta := &jarMeta{}
	for _, file := range reader.File {
		switch strings.ToLower(file.Name) {
		case "fabric.mod.json":
			if data := readZipFile(file, 4<<20); data != nil {
				if info := parseFabricJSON(data); info != nil {
					meta.merge(*info)
				}
			}
		case "quilt.mod.json":
			if data := readZipFile(file, 4<<20); data != nil {
				if info := parseQuiltJSON(data); info != nil {
					meta.merge(*info)
				}
			}
		case "meta-inf/mods.toml":
			if data := readZipFile(file, 4<<20); data != nil {
				if info := parseModsTOML(data); info != nil {
					meta.merge(*info)
				}
			}
		case "meta-inf/neoforge.mods.toml":
			if data := readZipFile(file, 4<<20); data != nil {
				if info := parseModsTOMLFor(data, "neoforge"); info != nil {
					meta.merge(*info)
				}
			}
		case "mcmod.info":
			if data := readZipFile(file, 4<<20); data != nil {
				if info := parseMcmodInfo(data); info != nil {
					meta.merge(*info)
				}
			}
		}
	}
	if meta.Name == "" && len(meta.Loaders) == 0 && meta.Description == "" {
		return nil
	}
	return meta
}

func (m *jarMeta) merge(info jarMeta) {
	if info.Name != "" && m.Name == "" {
		m.Name = info.Name
	}
	if info.Version != "" && m.Version == "" {
		m.Version = info.Version
	}
	if info.ID != "" && m.ID == "" {
		m.ID = info.ID
	}
	if info.Description != "" && m.Description == "" {
		m.Description = info.Description
	}
	if info.Authors != "" && m.Authors == "" {
		m.Authors = info.Authors
	}
	for _, loader := range info.Loaders {
		found := false
		for _, existing := range m.Loaders {
			if existing == loader {
				found = true
				break
			}
		}
		if !found {
			m.Loaders = append(m.Loaders, loader)
		}
	}
}

func readZipFile(file *zip.File, limit int64) []byte {
	rc, err := file.Open()
	if err != nil {
		return nil
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, limit))
	if err != nil {
		return nil
	}
	return data
}

func parseFabricJSON(data []byte) *jarMeta {
	var raw struct {
		ID          string          `json:"id"`
		Name        string          `json:"name"`
		Version     string          `json:"version"`
		Description string          `json:"description"`
		Authors     json.RawMessage `json:"authors"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	info := &jarMeta{
		ID:          raw.ID,
		Name:        raw.Name,
		Version:     raw.Version,
		Description: raw.Description,
		Authors:     authorsFromJSON(raw.Authors),
		Loaders:     []string{"fabric"},
	}
	if info.Name == "" {
		info.Name = info.ID
	}
	return info
}

func parseQuiltJSON(data []byte) *jarMeta {
	var raw struct {
		QuiltLoader struct {
			ID       string `json:"id"`
			Version  string `json:"version"`
			Metadata struct {
				Name         string          `json:"name"`
				Description  string          `json:"description"`
				Contributors json.RawMessage `json:"contributors"`
			} `json:"metadata"`
		} `json:"quilt_loader"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	loader := raw.QuiltLoader
	info := &jarMeta{
		ID:          loader.ID,
		Name:        loader.Metadata.Name,
		Version:     loader.Version,
		Description: loader.Metadata.Description,
		Authors:     authorsFromJSON(loader.Metadata.Contributors),
		Loaders:     []string{"quilt"},
	}
	if info.Name == "" {
		info.Name = info.ID
	}
	return info
}

func parseMcmodInfo(data []byte) *jarMeta {
	type legacyMod struct {
		ModID       string   `json:"modid"`
		Name        string   `json:"name"`
		Version     string   `json:"version"`
		Description string   `json:"description"`
		AuthorList  []string `json:"authorList"`
		Authors     string   `json:"authors"`
	}
	var list []legacyMod
	if err := json.Unmarshal(data, &list); err != nil {
		var wrapper struct {
			ModList []legacyMod `json:"modList"`
		}
		if err := json.Unmarshal(data, &wrapper); err != nil {
			return nil
		}
		list = wrapper.ModList
	}
	if len(list) == 0 {
		return nil
	}
	first := list[0]
	info := &jarMeta{
		ID:          first.ModID,
		Name:        first.Name,
		Version:     first.Version,
		Description: first.Description,
		Authors:     strings.Join(first.AuthorList, ", "),
		Loaders:     []string{"forge"},
	}
	if info.Authors == "" {
		info.Authors = first.Authors
	}
	if info.Name == "" {
		info.Name = info.ID
	}
	return info
}

// parseModsTOML reads the first [[mods]] section. The launcher only needs a few
// display fields, so a full TOML parser would be unnecessary weight.
func parseModsTOML(data []byte) *jarMeta {
	return parseModsTOMLFor(data, "forge")
}

func parseModsTOMLFor(data []byte, loader string) *jarMeta {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	start := strings.Index(text, "[[mods]]")
	if start < 0 {
		return &jarMeta{Loaders: []string{loader}}
	}
	section := text[start+len("[[mods]]"):]
	if next := strings.Index(section, "[["); next >= 0 {
		section = section[:next]
	}
	fields := parseTOMLFields(section)
	info := &jarMeta{
		ID:          fields["modId"],
		Name:        fields["displayName"],
		Version:     fields["version"],
		Description: fields["description"],
		Authors:     fields["authors"],
		Loaders:     []string{loader},
	}
	if info.Name == "" {
		info.Name = info.ID
	}
	return info
}

func parseTOMLFields(section string) map[string]string {
	fields := map[string]string{}
	lines := strings.Split(section, "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		value := strings.TrimSpace(line[eq+1:])
		if key == "" || strings.HasPrefix(key, "[") {
			continue
		}
		if strings.HasPrefix(value, `"""`) || strings.HasPrefix(value, "'''") {
			quote := value[:3]
			value = strings.TrimPrefix(value, quote)
			if end := strings.Index(value, quote); end >= 0 {
				fields[key] = strings.TrimSpace(value[:end])
				continue
			}
			var b strings.Builder
			b.WriteString(value)
			for i+1 < len(lines) {
				i++
				value = lines[i]
				if end := strings.Index(value, quote); end >= 0 {
					if b.Len() > 0 {
						b.WriteString("\n")
					}
					b.WriteString(value[:end])
					break
				}
				if b.Len() > 0 {
					b.WriteString("\n")
				}
				b.WriteString(value)
			}
			fields[key] = strings.TrimSpace(b.String())
			continue
		}
		if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') {
			quote := value[0]
			if end := strings.IndexByte(value[1:], quote); end >= 0 {
				value = value[1 : 1+end]
			} else {
				value = value[1:]
			}
		} else if idx := strings.Index(value, " #"); idx >= 0 {
			value = strings.TrimSpace(value[:idx])
		}
		fields[key] = value
	}
	return fields
}

func authorsFromJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var names []string
	if err := json.Unmarshal(raw, &names); err == nil {
		return strings.Join(names, ", ")
	}
	var objects []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &objects); err == nil {
		for _, obj := range objects {
			if obj.Name != "" {
				names = append(names, obj.Name)
			}
		}
		return strings.Join(names, ", ")
	}
	var byRole map[string][]string
	if err := json.Unmarshal(raw, &byRole); err == nil {
		var out []string
		for _, role := range byRole {
			out = append(out, role...)
		}
		return strings.Join(out, ", ")
	}
	return ""
}

func displayName(fileName string) string {
	name := fileName
	name = strings.TrimSuffix(name, ".disabled")
	name = strings.TrimSuffix(name, filepath.Ext(name))
	return strings.TrimSpace(name)
}

// Add copies a local jar into dir. The file name is preserved because mods
// reference each other by file name in some packs.
func Add(dir, source string) (*Mod, error) {
	info, err := os.Stat(source)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, fmt.Errorf("%s is a directory", source)
	}
	if !IsModFile(source) {
		return nil, fmt.Errorf("%s is not a .jar file", filepath.Base(source))
	}
	name := filepath.Base(source)
	if name == "" || name == "." || name == ".." {
		return nil, fmt.Errorf("invalid mod file name")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	dest, err := safeChild(dir, name)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(dest); err == nil {
		return nil, ErrExists
	}
	if err := copyFile(source, dest); err != nil {
		return nil, err
	}
	mod := Mod{
		FileName: name,
		Name:     displayName(name),
		Enabled:  !strings.HasSuffix(strings.ToLower(name), ".disabled"),
	}
	if meta := readJarMeta(dest); meta != nil {
		mod.merge(*meta)
	}
	return &mod, nil
}

func Remove(dir, fileName string) error {
	path, err := safeChild(dir, fileName)
	if err != nil {
		return err
	}
	if !IsModFile(fileName) {
		return fmt.Errorf("%s is not a mod file", fileName)
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("%s is a directory", fileName)
	}
	return os.Remove(path)
}

// SetEnabled toggles the .jar.disabled suffix. The returned file name is the
// new on-disk name.
func SetEnabled(dir, fileName string, enabled bool) (string, error) {
	path, err := safeChild(dir, fileName)
	if err != nil {
		return "", err
	}
	if !IsModFile(fileName) {
		return "", fmt.Errorf("%s is not a mod file", fileName)
	}
	target := EnabledFileName(fileName)
	if !enabled {
		target += ".disabled"
	}
	if target == fileName {
		return fileName, nil
	}
	dest, err := safeChild(dir, target)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(dest); err == nil {
		return "", ErrExists
	}
	if err := os.Rename(path, dest); err != nil {
		return "", err
	}
	return target, nil
}

func safeChild(dir, name string) (string, error) {
	if name == "" || name != filepath.Base(name) || strings.ContainsAny(name, `/\`) {
		return "", fmt.Errorf("invalid file name %q", name)
	}
	return filepath.Join(dir, name), nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		_ = os.Remove(dst)
		return err
	}
	return out.Close()
}
