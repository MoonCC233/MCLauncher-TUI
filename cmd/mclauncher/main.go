package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"
	"github.com/muesli/termenv"

	"github.com/MoonCC233/MCLauncher-TUI/internal/config"
	"github.com/MoonCC233/MCLauncher-TUI/internal/i18n"
	"github.com/MoonCC233/MCLauncher-TUI/internal/paths"
	"github.com/MoonCC233/MCLauncher-TUI/internal/tui"
	"github.com/MoonCC233/MCLauncher-TUI/internal/update"
	"github.com/MoonCC233/MCLauncher-TUI/internal/util"
	"github.com/MoonCC233/MCLauncher-TUI/internal/version"
)

func main() {
	var (
		dataDir       = flag.String("data-dir", "", "data directory (overrides MCLAUNCHER_DATA_DIR)")
		showVersion   = flag.Bool("version", false, "print version and exit")
		preview       = flag.Bool("preview", false, "render a demo frame to stdout and exit")
		previewScreen = flag.String("preview-screen", "home", "preview screen: home, instances, versions, accounts, settings, console, downloads, help, form, progress, theme, language, java, account-type, microsoft, microsoft-error, resolution, loader, loader-versions, loader-java, content")
		previewWidth  = flag.Int("width", 120, "preview width")
		previewHeight = flag.Int("height", 38, "preview height")
		lang          = flag.String("lang", "", "UI language: en, es, fr, de, zh-Hans, zh-Hant, ja, ar, pt-BR, pt-PT, ru, ko, tr, vi")
	)
	flag.Parse()
	configureConsole()

	if *showVersion {
		fmt.Printf("mclauncher %s\n", version.Full())
		return
	}

	if *preview {
		lipgloss.SetColorProfile(termenv.TrueColor)
		lipgloss.SetHasDarkBackground(true)
		fmt.Print(tui.Preview(*previewScreen, *previewWidth, *previewHeight, *lang))
		fmt.Print("\n")
		return
	}

	if !term.IsTerminal(os.Stdin.Fd()) || !term.IsTerminal(os.Stdout.Fd()) {
		fmt.Fprintln(os.Stderr, "mclauncher is a full-screen terminal UI.")
		fmt.Fprintln(os.Stderr, "Run it in Windows Terminal (or another modern terminal), or use --preview for a static frame.")
		os.Exit(1)
	}

	var bootstrapRoot string
	if *dataDir != "" {
		abs, absErr := paths.ResolveDataDir(*dataDir)
		if absErr != nil {
			fmt.Fprintln(os.Stderr, "invalid data directory:", absErr)
			os.Exit(1)
		}
		bootstrapRoot = abs
	} else {
		root, rootErr := paths.DefaultRoot()
		if rootErr != nil {
			fmt.Fprintln(os.Stderr, "could not resolve data directory:", rootErr)
			os.Exit(1)
		}
		bootstrapRoot = root
	}
	if err := os.MkdirAll(bootstrapRoot, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "could not create data directory:", err)
		os.Exit(1)
	}
	bootstrap := paths.New(bootstrapRoot)

	cfg, cfgErr := config.Load(bootstrap.ConfigFile)
	notice := ""
	if cfgErr != nil {
		backup := config.BackupCorrupt(bootstrap.ConfigFile)
		notice = "Config could not be loaded; starting with defaults."
		if backup != "" {
			notice = "Config was corrupt and has been backed up."
		}
		log.Printf("config load failed: %v (backup: %s)", cfgErr, backup)
		cfg = &config.Config{
			Path:        bootstrap.ConfigFile,
			DataVersion: config.DataVersion,
			Settings:    config.DefaultSettings(),
		}
	}

	dataRoot := bootstrapRoot
	envOverride := strings.TrimSpace(os.Getenv("MCLAUNCHER_DATA_DIR")) != ""
	if *dataDir == "" && !envOverride && cfg.Settings.DataDir != "" {
		resolved, resolveErr := paths.ResolveDataDir(cfg.Settings.DataDir)
		if resolveErr != nil {
			notice = "Could not use the configured data directory; using the default."
			log.Printf("data directory %q: %v", cfg.Settings.DataDir, resolveErr)
		} else {
			dataRoot = resolved
		}
	}
	layout := paths.New(dataRoot)
	if layout.Root == bootstrapRoot && cfg.Settings.DataDir != "" {
		cfg.Settings.DataDir = ""
	}
	if err := layout.Ensure(); err != nil {
		fmt.Fprintln(os.Stderr, "could not create data directory:", err)
		os.Exit(1)
	}

	logFile, logErr := os.OpenFile(layout.LauncherLog(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if logErr == nil {
		defer logFile.Close()
		log.SetOutput(logFile)
		log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	} else {
		log.SetOutput(os.Stderr)
	}
	log.Printf("mclauncher %s starting (data: %s)", version.Full(), layout.Root)
	defer log.Printf("mclauncher exiting")

	if _, statErr := os.Stat(cfg.Path); os.IsNotExist(statErr) {
		if saveErr := cfg.Save(); saveErr != nil {
			log.Printf("could not write initial config: %v", saveErr)
		}
	}
	if *lang != "" {
		canonical, ok := i18n.Canonical(*lang)
		if !ok {
			fmt.Fprintf(os.Stderr, "unsupported language %q\n", *lang)
			fmt.Fprintln(os.Stderr, "supported: en, es, fr, de, zh-Hans, zh-Hant, ja, ar, pt-BR, pt-PT, ru, ko, tr, vi")
			os.Exit(1)
		}
		cfg.Settings.Language = string(canonical)
		if err := cfg.Save(); err != nil {
			log.Printf("could not save language: %v", err)
		}
	}

	update.CleanupOldBinary()
	model := tui.New(cfg, layout, util.NewHTTPClient(), version.Full(), notice)
	program := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	model.SetSender(program.Send)
	final, runErr := program.Run()
	// Bubble Tea can exit on input errors as well as the quit key; make sure a
	// game started by the launcher does not survive that path.
	if finalModel, ok := final.(*tui.Model); ok {
		finalModel.Shutdown()
	} else {
		model.Shutdown()
	}
	if runErr != nil {
		log.Printf("tui error: %v", runErr)
		fmt.Fprintln(os.Stderr, "mclauncher:", runErr)
		os.Exit(1)
	}
	if err := cfg.Save(); err != nil {
		log.Printf("could not save config on exit: %v", err)
	}
}
