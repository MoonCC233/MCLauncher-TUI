package loader

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/MoonCC233/MCLauncher-TUI/internal/mojang"
	"github.com/MoonCC233/MCLauncher-TUI/internal/util"
)

var fabricMetaBase = "https://meta.fabricmc.net/v2"

type fabricLoaderEntry struct {
	Loader struct {
		Version string `json:"version"`
		Stable  bool   `json:"stable"`
	} `json:"loader"`
}

func fabricVersions(ctx context.Context, client *http.Client, gameVersion string) ([]Version, error) {
	var entries []fabricLoaderEntry
	endpoint := fmt.Sprintf("%s/versions/loader/%s", fabricMetaBase, url.PathEscape(gameVersion))
	if err := util.FetchJSON(ctx, client, endpoint, &entries); err != nil {
		return nil, fmt.Errorf("fetch Fabric loader versions: %w", err)
	}
	versions := make([]Version, 0, len(entries))
	for _, entry := range entries {
		if !validProfileID(entry.Loader.Version) {
			continue
		}
		versions = append(versions, Version{Version: entry.Loader.Version, Stable: entry.Loader.Stable})
	}
	if len(versions) == 0 {
		return nil, fmt.Errorf("Fabric has no loader builds for Minecraft %s", gameVersion)
	}
	return versions, nil
}

func installFabric(ctx context.Context, opts Options) (string, error) {
	opts.stage("Fetching Fabric profile", opts.LoaderVersion)
	endpoint := fmt.Sprintf("%s/versions/loader/%s/%s/profile/json",
		fabricMetaBase, url.PathEscape(opts.GameVersion), url.PathEscape(opts.LoaderVersion))
	var profile mojang.VersionJSON
	if err := util.FetchJSON(ctx, opts.client(), endpoint, &profile); err != nil {
		return "", fmt.Errorf("fetch Fabric profile %s: %w", opts.LoaderVersion, err)
	}
	if !validProfileID(profile.ID) {
		return "", fmt.Errorf("Fabric returned an invalid profile id %q", profile.ID)
	}
	if profile.InheritsFrom == "" {
		profile.InheritsFrom = opts.GameVersion
	}
	if !strings.EqualFold(profile.InheritsFrom, opts.GameVersion) {
		return "", fmt.Errorf("Fabric profile inherits %s, expected %s", profile.InheritsFrom, opts.GameVersion)
	}
	if profile.MainClass == "" {
		return "", fmt.Errorf("Fabric profile has no main class")
	}
	if err := writeProfile(opts.Layout, &profile); err != nil {
		return "", err
	}
	opts.log("wrote Fabric profile " + profile.ID)
	return profile.ID, nil
}

func writeProfile(layout interface {
	VersionJSON(string) string
}, profile *mojang.VersionJSON) error {
	if profile == nil || !validProfileID(profile.ID) {
		return fmt.Errorf("invalid loader profile")
	}
	return util.WriteJSONAtomic(layout.VersionJSON(profile.ID), profile)
}
