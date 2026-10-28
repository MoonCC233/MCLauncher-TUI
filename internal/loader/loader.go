package loader

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/MoonCC233/MCLauncher-TUI/internal/paths"
	"github.com/MoonCC233/MCLauncher-TUI/internal/util"
)

type Kind string

const (
	Fabric   Kind = "fabric"
	NeoForge Kind = "neoforge"
)

type Version struct {
	Version string
	Stable  bool
}

type Result struct {
	VersionID     string
	GameVersion   string
	Loader        Kind
	LoaderVersion string
}

type Options struct {
	Kind          Kind
	GameVersion   string
	LoaderVersion string
	Layout        paths.Layout
	Client        *http.Client
	JavaPath      string
	OnStage       func(stage, detail string)
	OnLog         func(line string)
}

func (o Options) stage(stage, detail string) {
	if o.OnStage != nil {
		o.OnStage(stage, detail)
	}
}

func (o Options) log(line string) {
	line = strings.TrimSpace(line)
	if line != "" && o.OnLog != nil {
		o.OnLog(line)
	}
}

func (o Options) client() *http.Client {
	if o.Client != nil {
		return o.Client
	}
	return util.NewHTTPClient()
}

func Label(kind Kind) string {
	switch kind {
	case Fabric:
		return "Fabric"
	case NeoForge:
		return "NeoForge"
	}
	return string(kind)
}

func ValidKind(kind Kind) bool {
	return kind == Fabric || kind == NeoForge
}

func Versions(ctx context.Context, client *http.Client, kind Kind, gameVersion string) ([]Version, error) {
	gameVersion = strings.TrimSpace(gameVersion)
	if gameVersion == "" {
		return nil, fmt.Errorf("game version is required")
	}
	if client == nil {
		client = util.NewHTTPClient()
	}
	switch kind {
	case Fabric:
		return fabricVersions(ctx, client, gameVersion)
	case NeoForge:
		return neoForgeVersions(ctx, client, gameVersion)
	default:
		return nil, fmt.Errorf("unknown mod loader %q", kind)
	}
}

func Install(ctx context.Context, opts Options) (Result, error) {
	opts.GameVersion = strings.TrimSpace(opts.GameVersion)
	opts.LoaderVersion = strings.TrimSpace(opts.LoaderVersion)
	if !ValidKind(opts.Kind) {
		return Result{}, fmt.Errorf("unknown mod loader %q", opts.Kind)
	}
	if opts.GameVersion == "" {
		return Result{}, fmt.Errorf("game version is required")
	}
	if opts.LoaderVersion == "" {
		versions, err := Versions(ctx, opts.client(), opts.Kind, opts.GameVersion)
		if err != nil {
			return Result{}, err
		}
		for _, version := range versions {
			if version.Stable {
				opts.LoaderVersion = version.Version
				break
			}
		}
		if opts.LoaderVersion == "" && len(versions) > 0 {
			opts.LoaderVersion = versions[0].Version
		}
		if opts.LoaderVersion == "" {
			return Result{}, fmt.Errorf("no %s versions found for Minecraft %s", Label(opts.Kind), opts.GameVersion)
		}
	}
	var (
		versionID string
		err       error
	)
	switch opts.Kind {
	case Fabric:
		versionID, err = installFabric(ctx, opts)
	case NeoForge:
		versionID, err = installNeoForge(ctx, opts)
	}
	if err != nil {
		return Result{}, err
	}
	return Result{
		VersionID:     versionID,
		GameVersion:   opts.GameVersion,
		Loader:        opts.Kind,
		LoaderVersion: opts.LoaderVersion,
	}, nil
}

var safeID = regexp.MustCompile(`^[A-Za-z0-9._+@-]+$`)

func validProfileID(id string) bool {
	return id != "" && id != "." && id != ".." && safeID.MatchString(id)
}
