package mojang

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/MoonCC233/MCLauncher-TUI/internal/util"
)

type VersionJSON struct {
	ID                 string           `json:"id"`
	InheritsFrom       string           `json:"inheritsFrom,omitempty"`
	Type               string           `json:"type"`
	MainClass          string           `json:"mainClass"`
	Assets             string           `json:"assets"`
	AssetIndex         *AssetIndexRef   `json:"assetIndex"`
	Downloads          VersionDownloads `json:"downloads"`
	JavaVersion        *JavaVersion     `json:"javaVersion"`
	Libraries          []Library        `json:"libraries"`
	Arguments          *Arguments       `json:"arguments"`
	MinecraftArguments string           `json:"minecraftArguments"`
	Logging            *Logging         `json:"logging"`
}

type AssetIndexRef struct {
	ID        string `json:"id"`
	SHA1      string `json:"sha1"`
	Size      int64  `json:"size"`
	TotalSize int64  `json:"totalSize"`
	URL       string `json:"url"`
}

type VersionDownloads struct {
	Client DownloadInfo `json:"client"`
	Server DownloadInfo `json:"server"`
}

type DownloadInfo struct {
	Path string `json:"path"`
	SHA1 string `json:"sha1"`
	Size int64  `json:"size"`
	URL  string `json:"url"`
}

type JavaVersion struct {
	Component    string `json:"component"`
	MajorVersion int    `json:"majorVersion"`
}

type Library struct {
	Name      string            `json:"name"`
	URL       string            `json:"url"`
	Downloads LibraryDownloads  `json:"downloads"`
	Natives   map[string]string `json:"natives"`
	Extract   *ExtractRule      `json:"extract"`
	Rules     []Rule            `json:"rules"`
}

type LibraryDownloads struct {
	Artifact    DownloadInfo            `json:"artifact"`
	Classifiers map[string]DownloadInfo `json:"classifiers"`
}

type ExtractRule struct {
	Exclude []string `json:"exclude"`
}

type Arguments struct {
	Game []Argument `json:"game"`
	JVM  []Argument `json:"jvm"`
}

type Argument struct {
	Rules  []Rule
	Values []string
}

func (a *Argument) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		a.Values = []string{s}
		return nil
	}
	var obj struct {
		Rules []Rule          `json:"rules"`
		Value json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	a.Rules = obj.Rules
	if len(obj.Value) == 0 {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(data, &fields); err != nil {
			return err
		}
		values, ok := fields["Values"]
		if !ok {
			return fmt.Errorf("argument value: missing")
		}
		if err := json.Unmarshal(values, &a.Values); err != nil {
			return fmt.Errorf("argument value: %w", err)
		}
		if rules, ok := fields["Rules"]; ok {
			if err := json.Unmarshal(rules, &a.Rules); err != nil {
				return fmt.Errorf("argument rules: %w", err)
			}
		}
		return nil
	}
	var one string
	if err := json.Unmarshal(obj.Value, &one); err == nil {
		a.Values = []string{one}
		return nil
	}
	var many []string
	if err := json.Unmarshal(obj.Value, &many); err != nil {
		return fmt.Errorf("argument value: %w", err)
	}
	a.Values = many
	return nil
}

func (a Argument) MarshalJSON() ([]byte, error) {
	if len(a.Rules) == 0 && len(a.Values) == 1 {
		return json.Marshal(a.Values[0])
	}
	var value any = a.Values
	if len(a.Values) == 1 {
		value = a.Values[0]
	}
	return json.Marshal(struct {
		Rules []Rule `json:"rules,omitempty"`
		Value any    `json:"value"`
	}{Rules: a.Rules, Value: value})
}

type Rule struct {
	Action   string          `json:"action"`
	OS       *OSRule         `json:"os"`
	Features map[string]bool `json:"features"`
}

type OSRule struct {
	Name    string `json:"name"`
	Arch    string `json:"arch"`
	Version string `json:"version"`
}

type Logging struct {
	Client *LoggingClient `json:"client"`
}

type LoggingClient struct {
	Argument string       `json:"argument"`
	File     DownloadInfo `json:"file"`
	Type     string       `json:"type"`
}

type AssetIndexFile struct {
	Objects        map[string]AssetObject `json:"objects"`
	Virtual        bool                   `json:"virtual"`
	MapToResources bool                   `json:"map_to_resources"`
}

type AssetObject struct {
	Hash string `json:"hash"`
	Size int64  `json:"size"`
}

func (v *VersionJSON) EffectiveID(fallback string) string {
	if v.ID != "" {
		return v.ID
	}
	return fallback
}

func (v *VersionJSON) TypeLabel() string {
	if v.Type == "" {
		return "release"
	}
	return v.Type
}

func (v *VersionJSON) JavaMajor() int {
	if v.JavaVersion != nil && v.JavaVersion.MajorVersion > 0 {
		return v.JavaVersion.MajorVersion
	}
	return 8
}

func (v *VersionJSON) AssetID() string {
	if v.AssetIndex != nil && v.AssetIndex.ID != "" {
		return v.AssetIndex.ID
	}
	if v.Assets != "" {
		return v.Assets
	}
	return "legacy"
}

// Allowed evaluates Mojang's rule list. Later matching rules override earlier
// ones, and an empty list means the entry is allowed.
func Allowed(rules []Rule, features map[string]bool) bool {
	if len(rules) == 0 {
		return true
	}
	allowed := false
	for _, rule := range rules {
		if ruleMatches(rule, features) {
			allowed = rule.Action != "disallow"
		}
	}
	return allowed
}

func ruleMatches(rule Rule, features map[string]bool) bool {
	if rule.OS != nil {
		if rule.OS.Name != "" && rule.OS.Name != util.OSName() {
			return false
		}
		if rule.OS.Arch != "" && rule.OS.Arch != util.OSArch() {
			return false
		}
		if rule.OS.Version != "" {
			re, err := regexp.Compile(rule.OS.Version)
			if err != nil || !re.MatchString(osVersion()) {
				return false
			}
		}
	}
	for key, want := range rule.Features {
		if features[key] != want {
			return false
		}
	}
	return true
}

func ArgFeatures(width, height int) map[string]bool {
	return map[string]bool{
		"is_demo_user":               false,
		"has_custom_resolution":      width > 0 && height > 0,
		"has_quick_plays_support":    false,
		"is_quick_play_singleplayer": false,
		"is_quick_play_multiplayer":  false,
		"is_quick_play_realms":       false,
	}
}

var placeholderRE = regexp.MustCompile(`\$\{[^}]+\}`)

func Substitute(s string, replacements map[string]string) string {
	if len(replacements) == 0 {
		return s
	}
	pairs := make([]string, 0, len(replacements)*2)
	for key, value := range replacements {
		pairs = append(pairs, "${"+key+"}", value)
	}
	out := strings.NewReplacer(pairs...).Replace(s)
	// Unfilled placeholders are safer as empty strings than as literal tokens.
	return placeholderRE.ReplaceAllString(out, "")
}

func EvaluateArguments(args []Argument, features map[string]bool, replacements map[string]string) []string {
	var out []string
	for _, arg := range args {
		if !Allowed(arg.Rules, features) {
			continue
		}
		for _, value := range arg.Values {
			out = append(out, Substitute(value, replacements))
		}
	}
	return out
}

func MavenPath(name string) string {
	parts := strings.Split(name, ":")
	if len(parts) < 3 {
		return ""
	}
	group, artifact, version := parts[0], parts[1], parts[2]
	classifier := ""
	if len(parts) > 3 {
		classifier = parts[3]
	}
	file := artifact + "-" + version
	if classifier != "" {
		file += "-" + classifier
	}
	file += ".jar"
	return strings.ReplaceAll(group, ".", "/") + "/" + artifact + "/" + version + "/" + file
}

func evaluateClassifierPath(lib Library, classifier string, info DownloadInfo) (DownloadInfo, bool) {
	if info.Path == "" {
		coords := strings.Split(lib.Name, ":")
		if len(coords) < 3 {
			return info, false
		}
		file := coords[1] + "-" + coords[2]
		if classifier != "" {
			file += "-" + classifier
		}
		file += ".jar"
		info.Path = strings.ReplaceAll(coords[0], ".", "/") + "/" + coords[1] + "/" + coords[2] + "/" + file
	}
	if info.URL == "" {
		base := lib.URL
		if base == "" {
			base = "https://libraries.minecraft.net/"
		}
		info.URL = strings.TrimRight(base, "/") + "/" + strings.ReplaceAll(info.Path, "\\", "/")
	}
	return info, true
}
