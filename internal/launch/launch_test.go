package launch

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MoonCC233/MCLauncher-TUI/internal/config"
	"github.com/MoonCC233/MCLauncher-TUI/internal/java"
	"github.com/MoonCC233/MCLauncher-TUI/internal/mojang"
	"github.com/MoonCC233/MCLauncher-TUI/internal/paths"
)

func fixture(t *testing.T) Options {
	t.Helper()
	layout := paths.New(t.TempDir())
	if err := layout.Ensure(); err != nil {
		t.Fatal(err)
	}
	const id = "1.20.1"
	if err := os.MkdirAll(filepath.Dir(layout.ClientJar(id)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(layout.ClientJar(id), []byte("fake client jar"), 0o644); err != nil {
		t.Fatal(err)
	}
	return Options{
		Layout:    layout,
		Version:   &mojang.VersionJSON{ID: id, Type: "release", MainClass: "net.minecraft.client.main.Main", Assets: "12"},
		VersionID: id,
		Account:   config.Account{Username: "Steve", UUID: "0f31b1f0-e0c1-4f8b-8a9c-6d5e4f3a2b1c"},
		Java:      java.Info{Path: "java", Version: "17.0.13", Major: 17},
		GameDir:   filepath.Join(layout.Instances, "test-instance"),
		MemoryMB:  2048,
		Width:     1280,
		Height:    720,

		LauncherVersion: "0.1.0",
	}
}

func contains(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}

func indexOf(args []string, want string) int {
	for i, arg := range args {
		if arg == want {
			return i
		}
	}
	return -1
}

func TestBuildModernArguments(t *testing.T) {
	opts := fixture(t)
	opts.Version.Arguments = &mojang.Arguments{
		JVM: []mojang.Argument{
			{Values: []string{"-Djava.library.path=${natives_directory}", "-cp", "${classpath}"}},
		},
		Game: []mojang.Argument{
			{Values: []string{"--username", "${auth_player_name}", "--uuid", "${auth_uuid}", "--version", "${version_name}", "--userType", "${user_type}"}},
			{
				Rules:  []mojang.Rule{{Action: "allow", Features: map[string]bool{"has_custom_resolution": true}}},
				Values: []string{"--width", "${resolution_width}", "--height", "${resolution_height}"},
			},
		},
	}

	spec, err := Build(opts)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if spec.Path != "java" {
		t.Errorf("Path = %q", spec.Path)
	}
	if spec.Dir != opts.GameDir {
		t.Errorf("Dir = %q, want %q", spec.Dir, opts.GameDir)
	}
	if len(spec.Env) == 0 {
		t.Error("Env should inherit the current environment")
	}

	natives := opts.Layout.NativesDir("1.20.1")
	wantClasspath := opts.Layout.ClientJar("1.20.1")
	for _, want := range []string{
		"-Xms1024M",
		"-Xmx2048M",
		"-Djava.library.path=" + natives,
		"-cp",
		wantClasspath,
		"net.minecraft.client.main.Main",
		"--username", "Steve",
		"--uuid", "0f31b1f0e0c14f8b8a9c6d5e4f3a2b1c",
		"--version", "1.20.1",
		"--userType", "legacy",
		"--width", "1280",
		"--height", "720",
	} {
		if !contains(spec.Args, want) {
			t.Errorf("args missing %q:\n%v", want, spec.Args)
		}
	}
	if indexOf(spec.Args, "net.minecraft.client.main.Main") > indexOf(spec.Args, "--username") {
		t.Error("main class must come before game arguments")
	}
	for _, arg := range spec.Args {
		if strings.Contains(arg, "${") {
			t.Errorf("unfilled placeholder in %q", arg)
		}
	}
}

func TestBuildLegacyArguments(t *testing.T) {
	opts := fixture(t)
	opts.Width = 0
	opts.Height = 0
	opts.Version.MinecraftArguments = "--username ${auth_player_name} --gameDir ${game_directory} --assetsDir ${assets_root} --assetIndex ${assets_index_name} --uuid ${auth_uuid} --accessToken ${auth_access_token} --userType ${user_type}"

	spec, err := Build(opts)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if !contains(spec.Args, "-Djava.library.path="+opts.Layout.NativesDir("1.20.1")) {
		t.Errorf("legacy JVM args missing library path: %v", spec.Args)
	}
	if !contains(spec.Args, "-cp") || !contains(spec.Args, opts.Layout.ClientJar("1.20.1")) {
		t.Errorf("legacy JVM args missing classpath: %v", spec.Args)
	}
	for _, want := range []string{
		"--username", "Steve",
		"--assetsDir", opts.Layout.Assets,
		"--assetIndex", "12",
		"--uuid", "0f31b1f0e0c14f8b8a9c6d5e4f3a2b1c",
		"--accessToken", "0",
		"--userType", "legacy",
	} {
		if !contains(spec.Args, want) {
			t.Errorf("args missing %q:\n%v", want, spec.Args)
		}
	}
}

func TestBuildDefaultArguments(t *testing.T) {
	opts := fixture(t)
	spec, err := Build(opts)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	for _, want := range []string{
		"--username", "Steve",
		"--version", "1.20.1",
		"--gameDir", opts.GameDir,
		"--assetsDir", opts.Layout.Assets,
		"--assetIndex", "12",
		"--uuid", "0f31b1f0e0c14f8b8a9c6d5e4f3a2b1c",
		"--accessToken", "0",
		"--userType", "legacy",
		"--userProperties", "{}",
	} {
		if !contains(spec.Args, want) {
			t.Errorf("args missing %q:\n%v", want, spec.Args)
		}
	}
	if !contains(spec.Args, "-cp") || !contains(spec.Args, opts.Layout.ClientJar("1.20.1")) {
		t.Errorf("default JVM args missing classpath: %v", spec.Args)
	}
}

func TestBuildJVMArgsAndLogConfig(t *testing.T) {
	opts := fixture(t)
	opts.Version.Arguments = &mojang.Arguments{
		JVM: []mojang.Argument{{Values: []string{"-cp", "${classpath}"}}},
	}
	opts.JVMArgs = `-XX:+UseG1GC -Dfoo="bar baz"`
	logConfig := filepath.Join(t.TempDir(), "log4j.xml")
	if err := os.WriteFile(logConfig, []byte("<xml/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	opts.LogConfig = logConfig

	spec, err := Build(opts)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	for _, want := range []string{
		"-XX:+UseG1GC",
		"-Dfoo=bar baz",
		"-Dlog4j.configurationFile=" + logConfig,
	} {
		if !contains(spec.Args, want) {
			t.Errorf("args missing %q:\n%v", want, spec.Args)
		}
	}
	if indexOf(spec.Args, "-XX:+UseG1GC") > indexOf(spec.Args, "net.minecraft.client.main.Main") {
		t.Error("user JVM args should come before the main class")
	}

	// A missing log config is silently ignored.
	opts.LogConfig = filepath.Join(t.TempDir(), "missing.xml")
	spec, err = Build(opts)
	if err != nil {
		t.Fatalf("Build(missing log config): %v", err)
	}
	for _, arg := range spec.Args {
		if strings.HasPrefix(arg, "-Dlog4j.configurationFile=") {
			t.Errorf("missing log config should not be passed: %q", arg)
		}
	}
}

func TestBuildValidation(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*testing.T, *Options)
		wantErr string
	}{
		{"nil version", func(_ *testing.T, o *Options) { o.Version = nil }, "missing version metadata"},
		{"no java", func(_ *testing.T, o *Options) { o.Java.Path = "" }, "no Java runtime"},
		{"no main class", func(_ *testing.T, o *Options) { o.Version.MainClass = "" }, "no main class"},
		{"no version id", func(_ *testing.T, o *Options) { o.Version.ID = ""; o.VersionID = "" }, "missing version id"},
		{"no game dir", func(_ *testing.T, o *Options) { o.GameDir = "" }, "missing game directory"},
		{"no client jar", func(t *testing.T, o *Options) {
			if err := os.Remove(o.Layout.ClientJar("1.20.1")); err != nil {
				t.Fatal(err)
			}
		}, "client jar"},
		{"missing library", func(_ *testing.T, o *Options) {
			o.Version.Libraries = []mojang.Library{{Name: "com.example:missing:1.0"}}
		}, "library"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := fixture(t)
			tc.mutate(t, &opts)
			_, err := Build(opts)
			if err == nil {
				t.Fatalf("Build = nil error, want %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("Build error = %q, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

func TestCommandLineAndQuote(t *testing.T) {
	spec := &Spec{
		Path: `C:\Program Files\Java\bin\java.exe`,
		Args: []string{"-cp", "a b", "plain", "", `-Dx=a"b`},
	}
	want := `"C:\Program Files\Java\bin\java.exe" -cp "a b" plain "" "-Dx=a\"b"`
	if got := spec.CommandLine(); got != want {
		t.Errorf("CommandLine = %q, want %q", got, want)
	}
	if got := (&Spec{}).CommandLine(); got != `""` {
		t.Errorf("CommandLine(empty path) = %q", got)
	}
}

func TestExitCode(t *testing.T) {
	if got := ExitCode(nil); got != 0 {
		t.Errorf("ExitCode(nil) = %d, want 0", got)
	}
	if got := ExitCode(errors.New("boom")); got != -1 {
		t.Errorf("ExitCode(generic) = %d, want -1", got)
	}
}

func TestExitCodeExitError(t *testing.T) {
	if os.Getenv("MCLAUNCHER_EXIT_HELPER") == "1" {
		os.Exit(3)
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestExitCodeExitError")
	cmd.Env = append(os.Environ(), "MCLAUNCHER_EXIT_HELPER=1")
	err := cmd.Run()
	if err == nil {
		t.Fatal("helper process should have failed")
	}
	if got := ExitCode(err); got != 3 {
		t.Errorf("ExitCode(exit 3) = %d, want 3", got)
	}
}

func TestNilSession(t *testing.T) {
	var s *Session
	if s.Stop() != nil {
		t.Error("Stop on a nil session should be a no-op")
	}
	if s.Pid() != 0 || s.PidValue() != 0 {
		t.Error("Pid on a nil session should be 0")
	}
}
