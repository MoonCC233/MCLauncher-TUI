package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"
	"github.com/muesli/termenv"

	"github.com/MoonCC233/MCLauncher-TUI/internal/config"
	"github.com/MoonCC233/MCLauncher-TUI/internal/i18n"
	"github.com/MoonCC233/MCLauncher-TUI/internal/paths"
	"github.com/MoonCC233/MCLauncher-TUI/internal/tui"
	"github.com/MoonCC233/MCLauncher-TUI/internal/util"
	"github.com/MoonCC233/MCLauncher-TUI/internal/version"
)

func main() {
	var (
		dataDir       = flag.String("data-dir", "", "data directory (overrides MCLAUNCHER_DATA_DIR)")
		showVersion   = flag.Bool("version", false, "print version and exit")
		preview       = flag.Bool("preview", false, "render a demo frame to stdout and exit")
		previewScreen = flag.String("preview-screen", "home", "preview screen: home, instances, versions, accounts, settings, console, help, form, progress, language, java")
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

	var (
		layout paths.Layout
		err    error
	)
	if *dataDir != "" {
		abs, absErr := filepath.Abs(*dataDir)
		if absErr != nil {
			fmt.Fprintln(os.Stderr, "invalid data directory:", absErr)
			os.Exit(1)
		}
		layout = paths.New(abs)
	} else {
		root, rootErr := paths.DefaultRoot()
		if rootErr != nil {
			fmt.Fprintln(os.Stderr, "could not resolve data directory:", rootErr)
			os.Exit(1)
		}
		layout = paths.New(root)
	}
	if err = layout.Ensure(); err != nil {
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
	log.Printf("mclauncher %s starting", version.Full())
	defer log.Printf("mclauncher exiting")

	cfg, cfgErr := config.Load(layout.ConfigFile)
	notice := ""
	if cfgErr != nil {
		backup := config.BackupCorrupt(layout.ConfigFile)
		notice = "Config could not be loaded; starting with defaults."
		if backup != "" {
			notice = "Config was corrupt and has been backed up."
		}
		log.Printf("config load failed: %v (backup: %s)", cfgErr, backup)
		cfg = &config.Config{
			Path:        layout.ConfigFile,
			DataVersion: config.DataVersion,
			Settings:    config.DefaultSettings(),
		}
	}
	if _, statErr := os.Stat(layout.ConfigFile); os.IsNotExist(statErr) {
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

	model := tui.New(cfg, layout, util.NewHTTPClient(), version.Full(), notice)
	program := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	model.SetSender(program.Send)
	if _, err := program.Run(); err != nil {
		log.Printf("tui error: %v", err)
		fmt.Fprintln(os.Stderr, "mclauncher:", err)
		os.Exit(1)
	}
	if err := cfg.Save(); err != nil {
		log.Printf("could not save config on exit: %v", err)
	}
}
