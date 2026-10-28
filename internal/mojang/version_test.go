package mojang

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/MoonCC233/MCLauncher-TUI/internal/util"
)

func TestAllowed(t *testing.T) {
	osName := util.OSName()
	osArch := util.OSArch()

	cases := []struct {
		name     string
		rules    []Rule
		features map[string]bool
		want     bool
	}{
		{"no rules", nil, nil, true},
		{"disallow all", []Rule{{Action: "disallow"}}, nil, false},
		{"allow current os", []Rule{{Action: "allow", OS: &OSRule{Name: osName}}}, nil, true},
		{"allow other os", []Rule{{Action: "allow", OS: &OSRule{Name: "not-" + osName}}}, nil, false},
		{"allow current arch", []Rule{{Action: "allow", OS: &OSRule{Arch: osArch}}}, nil, true},
		{"allow other arch", []Rule{{Action: "allow", OS: &OSRule{Arch: "not-" + osArch}}}, nil, false},
		{"os version regex", []Rule{{Action: "allow", OS: &OSRule{Version: ".*"}}}, nil, true},
		{"feature enabled", []Rule{{Action: "allow", Features: map[string]bool{"has_custom_resolution": true}}}, map[string]bool{"has_custom_resolution": true}, true},
		{"feature absent", []Rule{{Action: "allow", Features: map[string]bool{"has_custom_resolution": true}}}, map[string]bool{}, false},
		{"later disallow wins", []Rule{{Action: "allow"}, {Action: "disallow"}}, nil, false},
		{"later allow wins", []Rule{{Action: "disallow"}, {Action: "allow"}}, nil, true},
		{"unmatched disallow", []Rule{{Action: "disallow", Features: map[string]bool{"has_custom_resolution": true}}}, map[string]bool{}, false},
	}
	for _, tc := range cases {
		if got := Allowed(tc.rules, tc.features); got != tc.want {
			t.Errorf("%s: Allowed = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestSubstitute(t *testing.T) {
	repl := map[string]string{
		"auth_player_name": "Steve",
		"version_name":     "1.20.1",
	}
	got := Substitute("--username ${auth_player_name} --version ${version_name} --missing ${unknown}", repl)
	want := "--username Steve --version 1.20.1 --missing "
	if got != want {
		t.Errorf("Substitute = %q, want %q", got, want)
	}
	if got := Substitute("--username ${auth_player_name}", map[string]string{"auth_player_name": "Steve"}); got != "--username Steve" {
		t.Errorf("Substitute single = %q", got)
	}
	// With no replacements at all the input is returned untouched.
	if got := Substitute("${unknown}", nil); got != "${unknown}" {
		t.Errorf("Substitute(nil map) = %q", got)
	}
}

func TestEvaluateArguments(t *testing.T) {
	args := []Argument{
		{Values: []string{"--username", "${auth_player_name}"}},
		{
			Rules:  []Rule{{Action: "allow", Features: map[string]bool{"has_custom_resolution": true}}},
			Values: []string{"--width", "${resolution_width}"},
		},
		{
			Rules:  []Rule{{Action: "disallow", OS: &OSRule{Name: util.OSName()}}},
			Values: []string{"--hidden"},
		},
	}
	features := ArgFeatures(1280, 720)
	repl := map[string]string{
		"auth_player_name": "Steve",
		"resolution_width": "1280",
	}
	got := EvaluateArguments(args, features, repl)
	want := []string{"--username", "Steve", "--width", "1280"}
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Errorf("EvaluateArguments = %#v, want %#v", got, want)
	}

	// Zero resolution disables the custom-resolution arguments.
	got = EvaluateArguments(args, ArgFeatures(0, 0), repl)
	if strings.Join(got, "\x00") != strings.Join([]string{"--username", "Steve"}, "\x00") {
		t.Errorf("EvaluateArguments(no resolution) = %#v", got)
	}
}

func TestArgumentUnmarshalJSON(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		rules  int
		values []string
	}{
		{"string", `"-Xmx2G"`, 0, []string{"-Xmx2G"}},
		{"object string", `{"rules":[{"action":"allow"}],"value":"--demo"}`, 1, []string{"--demo"}},
		{"object array", `{"value":["--width","1280"]}`, 0, []string{"--width", "1280"}},
	}
	for _, tc := range cases {
		var arg Argument
		if err := json.Unmarshal([]byte(tc.input), &arg); err != nil {
			t.Errorf("%s: Unmarshal error: %v", tc.name, err)
			continue
		}
		if len(arg.Rules) != tc.rules {
			t.Errorf("%s: rules = %d, want %d", tc.name, len(arg.Rules), tc.rules)
		}
		if strings.Join(arg.Values, "|") != strings.Join(tc.values, "|") {
			t.Errorf("%s: values = %#v, want %#v", tc.name, arg.Values, tc.values)
		}
	}

	var arg Argument
	if err := json.Unmarshal([]byte(`{"value":5}`), &arg); err == nil {
		t.Error("numeric value should not unmarshal")
	}
	if err := json.Unmarshal([]byte(`{`), &arg); err == nil {
		t.Error("broken JSON should not unmarshal")
	}
	if err := json.Unmarshal([]byte(`{"Rules":null,"Values":["-Dlegacy=1"]}`), &arg); err != nil || len(arg.Values) != 1 || arg.Values[0] != "-Dlegacy=1" {
		t.Errorf("legacy argument = %+v, err = %v", arg, err)
	}
}

func TestArgumentMarshalJSONRoundTrip(t *testing.T) {
	args := []Argument{
		{Values: []string{"-Xmx2G"}},
		{Values: []string{"--width", "1280"}},
		{
			Rules:  []Rule{{Action: "allow", OS: &OSRule{Name: "windows"}}},
			Values: []string{"-Dfabric=1"},
		},
	}
	for _, arg := range args {
		data, err := json.Marshal(arg)
		if err != nil {
			t.Fatalf("Marshal(%+v): %v", arg, err)
		}
		var got Argument
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("Unmarshal(%s): %v", data, err)
		}
		if len(got.Rules) != len(arg.Rules) || strings.Join(got.Values, "|") != strings.Join(arg.Values, "|") {
			t.Errorf("round trip %s = %+v, want %+v", data, got, arg)
		}
	}
}

func TestMavenPath(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"org.lwjgl:lwjgl:3.3.2", "org/lwjgl/lwjgl/3.3.2/lwjgl-3.3.2.jar"},
		{"org.lwjgl:lwjgl:3.3.2:natives-windows", "org/lwjgl/lwjgl/3.3.2/lwjgl-3.3.2-natives-windows.jar"},
		{"net.minecraft:client:1.20.1", "net/minecraft/client/1.20.1/client-1.20.1.jar"},
		{"invalid", ""},
		{"a:b", ""},
	}
	for _, tc := range cases {
		if got := MavenPath(tc.in); got != tc.want {
			t.Errorf("MavenPath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestVersionHelpers(t *testing.T) {
	v := &VersionJSON{}
	if got := v.EffectiveID("fallback"); got != "fallback" {
		t.Errorf("EffectiveID = %q", got)
	}
	if got := v.TypeLabel(); got != "release" {
		t.Errorf("TypeLabel = %q", got)
	}
	if got := v.JavaMajor(); got != 8 {
		t.Errorf("JavaMajor = %d", got)
	}
	if got := v.AssetID(); got != "legacy" {
		t.Errorf("AssetID = %q", got)
	}

	v = &VersionJSON{
		ID:          "1.20.1",
		Type:        "snapshot",
		Assets:      "legacy-assets",
		AssetIndex:  &AssetIndexRef{ID: "12"},
		JavaVersion: &JavaVersion{MajorVersion: 17},
	}
	if got := v.EffectiveID("fallback"); got != "1.20.1" {
		t.Errorf("EffectiveID = %q", got)
	}
	if got := v.TypeLabel(); got != "snapshot" {
		t.Errorf("TypeLabel = %q", got)
	}
	if got := v.JavaMajor(); got != 17 {
		t.Errorf("JavaMajor = %d", got)
	}
	if got := v.AssetID(); got != "12" {
		t.Errorf("AssetID(index) = %q", got)
	}
	v.AssetIndex = nil
	if got := v.AssetID(); got != "legacy-assets" {
		t.Errorf("AssetID(assets) = %q", got)
	}
	v.JavaVersion.MajorVersion = 0
	if got := v.JavaMajor(); got != 8 {
		t.Errorf("JavaMajor(zero) = %d", got)
	}
}

func TestArgFeatures(t *testing.T) {
	on := ArgFeatures(1280, 720)
	if !on["has_custom_resolution"] {
		t.Error("has_custom_resolution should be enabled")
	}
	for _, key := range []string{"is_demo_user", "has_quick_plays_support", "is_quick_play_singleplayer", "is_quick_play_multiplayer", "is_quick_play_realms"} {
		if on[key] {
			t.Errorf("%s should default to false", key)
		}
	}
	if ArgFeatures(1280, 0)["has_custom_resolution"] {
		t.Error("a zero height should disable custom resolution")
	}
	if ArgFeatures(0, 0)["has_custom_resolution"] {
		t.Error("zero dimensions should disable custom resolution")
	}
}
