package launch

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/MoonCC233/MCLauncher-TUI/internal/config"
	"github.com/MoonCC233/MCLauncher-TUI/internal/java"
	"github.com/MoonCC233/MCLauncher-TUI/internal/mojang"
	"github.com/MoonCC233/MCLauncher-TUI/internal/paths"
	"github.com/MoonCC233/MCLauncher-TUI/internal/util"
)

// Options contains everything needed to build a Minecraft command line.
type Options struct {
	Layout          paths.Layout
	Version         *mojang.VersionJSON
	VersionID       string
	Account         config.Account
	Java            java.Info
	GameDir         string
	AssetsDir       string
	MemoryMB        int
	JVMArgs         string
	Width           int
	Height          int
	LogConfig       string
	LauncherVersion string
}

// Spec is a ready-to-run process description.
type Spec struct {
	Path string
	Args []string
	Dir  string
	Env  []string
}

func (s *Spec) CommandLine() string {
	parts := make([]string, 0, len(s.Args)+1)
	parts = append(parts, quote(s.Path))
	for _, arg := range s.Args {
		parts = append(parts, quote(arg))
	}
	return strings.Join(parts, " ")
}

func quote(s string) string {
	if s == "" {
		return `""`
	}
	if strings.ContainsAny(s, " \t\n\"") {
		return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
	}
	return s
}

// Build turns version metadata plus launcher settings into a Java invocation.
func Build(opts Options) (*Spec, error) {
	if opts.Version == nil {
		return nil, fmt.Errorf("missing version metadata")
	}
	if opts.Java.Path == "" {
		return nil, fmt.Errorf("no Java runtime selected")
	}
	if opts.Version.MainClass == "" {
		return nil, fmt.Errorf("version has no main class")
	}
	versionID := opts.VersionID
	if versionID == "" {
		versionID = opts.Version.EffectiveID("")
	}
	if versionID == "" {
		return nil, fmt.Errorf("missing version id")
	}
	gameDir := opts.GameDir
	if gameDir == "" {
		return nil, fmt.Errorf("missing game directory")
	}
	if err := os.MkdirAll(gameDir, 0o755); err != nil {
		return nil, fmt.Errorf("create instance directory: %w", err)
	}
	microsoft := opts.Account.IsMicrosoft()
	if microsoft && strings.TrimSpace(opts.Account.AccessToken) == "" {
		return nil, fmt.Errorf("Microsoft account %s has no access token — sign in again from the Accounts screen", opts.Account.Username)
	}
	assetsDir := opts.AssetsDir
	if assetsDir == "" {
		assetsDir = opts.Layout.Assets
	}
	clientJar := opts.Layout.ClientJar(versionID)
	if st, err := os.Stat(clientJar); err != nil || st.Size() == 0 {
		return nil, fmt.Errorf("client jar for %s is missing — reinstall it from the Versions screen", versionID)
	}

	features := mojang.ArgFeatures(opts.Width, opts.Height)
	res := mojang.ResolveLibraries(opts.Version, features)
	classpath := mojang.Classpath(opts.Layout, res, clientJar)
	for _, rel := range res.Classpath {
		path := filepath.Join(opts.Layout.Libraries, filepath.FromSlash(rel))
		if st, err := os.Stat(path); err != nil || st.Size() == 0 {
			return nil, fmt.Errorf("library %s is missing — reinstall %s", filepath.Base(rel), versionID)
		}
	}

	width, height := opts.Width, opts.Height
	if width <= 0 || height <= 0 {
		width, height = 0, 0
	}
	memory := util.Clamp(opts.MemoryMB, 512, 65536)
	nativesDir := opts.Layout.NativesDir(versionID)

	accessToken := "0"
	authSession := "0"
	userType := "legacy"
	xuid := ""
	if microsoft {
		accessToken = strings.TrimSpace(opts.Account.AccessToken)
		userType = "msa"
		xuid = opts.Account.XUID
	}

	replacements := map[string]string{
		"auth_player_name":    opts.Account.Username,
		"auth_uuid":           strings.ReplaceAll(opts.Account.UUID, "-", ""),
		"auth_access_token":   accessToken,
		"auth_session":        authSession,
		"auth_xuid":           xuid,
		"clientid":            "",
		"user_type":           userType,
		"user_properties":     "{}",
		"version_name":        versionID,
		"version_type":        opts.Version.TypeLabel(),
		"game_directory":      gameDir,
		"assets_root":         assetsDir,
		"game_assets":         assetsDir,
		"assets_index_name":   opts.Version.AssetID(),
		"natives_directory":   nativesDir,
		"launcher_name":       "mclauncher",
		"launcher_version":    opts.LauncherVersion,
		"classpath":           classpath,
		"classpath_separator": util.ClasspathSeparator(),
		"library_directory":   opts.Layout.Libraries,
		"resolution_width":    fmt.Sprintf("%d", width),
		"resolution_height":   fmt.Sprintf("%d", height),
	}

	var jvmArgs []string
	switch {
	case opts.Version.Arguments != nil && len(opts.Version.Arguments.JVM) > 0:
		jvmArgs = mojang.EvaluateArguments(opts.Version.Arguments.JVM, features, replacements)
	case opts.Version.MinecraftArguments != "":
		jvmArgs = []string{
			"-Djava.library.path=" + nativesDir,
			"-cp", classpath,
		}
	default:
		jvmArgs = []string{"-cp", classpath}
	}

	var launcherArgs []string
	launcherArgs = append(launcherArgs,
		fmt.Sprintf("-Xms%dM", min(memory, 1024)),
		fmt.Sprintf("-Xmx%dM", memory),
	)
	launcherArgs = append(launcherArgs, jvmArgs...)
	launcherArgs = append(launcherArgs, util.SplitArgs(opts.JVMArgs)...)
	if opts.LogConfig != "" {
		if st, err := os.Stat(opts.LogConfig); err == nil && !st.IsDir() {
			launcherArgs = append(launcherArgs, "-Dlog4j.configurationFile="+opts.LogConfig)
		}
	}
	launcherArgs = append(launcherArgs, opts.Version.MainClass)

	var gameArgs []string
	switch {
	case opts.Version.Arguments != nil && len(opts.Version.Arguments.Game) > 0:
		gameArgs = mojang.EvaluateArguments(opts.Version.Arguments.Game, features, replacements)
	case opts.Version.MinecraftArguments != "":
		filled := mojang.Substitute(opts.Version.MinecraftArguments, replacements)
		gameArgs = util.SplitArgs(filled)
	default:
		gameArgs = []string{
			"--username", opts.Account.Username,
			"--version", versionID,
			"--gameDir", gameDir,
			"--assetsDir", assetsDir,
			"--assetIndex", opts.Version.AssetID(),
			"--uuid", replacements["auth_uuid"],
			"--accessToken", accessToken,
			"--userType", userType,
			"--userProperties", "{}",
		}
		if xuid != "" {
			gameArgs = append(gameArgs, "--xuid", xuid)
		}
	}
	launcherArgs = append(launcherArgs, gameArgs...)

	args := make([]string, 0, len(launcherArgs))
	for _, arg := range launcherArgs {
		if arg == "" {
			continue
		}
		args = append(args, arg)
	}

	return &Spec{
		Path: opts.Java.Path,
		Args: args,
		Dir:  gameDir,
		Env:  os.Environ(),
	}, nil
}

// Line is a single line of game output.
type Line struct {
	Stream string
	Text   string
	At     time.Time
}

// Session owns a running Java process and streams its output.
type Session struct {
	cmd     *exec.Cmd
	lines   chan Line
	done    chan struct{}
	waitErr error
	started time.Time

	stopOnce sync.Once
	stopErr  error
}

func Start(spec *Spec) (*Session, error) {
	if spec == nil {
		return nil, fmt.Errorf("missing launch spec")
	}
	cmd := exec.Command(spec.Path, spec.Args...)
	cmd.Dir = spec.Dir
	cmd.Env = spec.Env
	hideWindow(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start Java: %w", err)
	}

	s := &Session{
		cmd:     cmd,
		lines:   make(chan Line, 4096),
		done:    make(chan struct{}),
		started: time.Now(),
	}
	go func() {
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			pump(stdout, "stdout", s.lines)
		}()
		go func() {
			defer wg.Done()
			pump(stderr, "stderr", s.lines)
		}()
		wg.Wait()
		s.waitErr = cmd.Wait()
		close(s.lines)
		close(s.done)
	}()
	return s, nil
}

func pump(r io.Reader, stream string, out chan<- Line) {
	reader := bufio.NewReaderSize(r, 64*1024)
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			line = strings.TrimRight(line, "\r\n")
			if len(line) > 16*1024 {
				line = line[:16*1024] + "…"
			}
			out <- Line{Stream: stream, Text: line, At: time.Now()}
		}
		if err != nil {
			return
		}
	}
}

func (s *Session) Lines() <-chan Line    { return s.lines }
func (s *Session) Done() <-chan struct{} { return s.done }
func (s *Session) Pid() int {
	if s == nil || s.cmd == nil || s.cmd.Process == nil {
		return 0
	}
	return s.cmd.Process.Pid
}
func (s *Session) PidValue() int        { return s.Pid() }
func (s *Session) StartedAt() time.Time { return s.started }

// Wait blocks until the process and its output streams have finished.
func (s *Session) Wait() error {
	<-s.done
	return s.waitErr
}

// Stop terminates the process tree. Calling it more than once is safe.
func (s *Session) Stop() error {
	if s == nil {
		return nil
	}
	select {
	case <-s.done:
		return nil
	default:
	}
	s.stopOnce.Do(func() {
		s.stopErr = terminate(s.cmd)
	})
	return s.stopErr
}

func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if ok := asExitError(err, &exitErr); ok {
		return exitErr.ExitCode()
	}
	return -1
}

func asExitError(err error, target **exec.ExitError) bool {
	for err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			*target = e
			return true
		}
		type unwrapper interface{ Unwrap() error }
		u, ok := err.(unwrapper)
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}
