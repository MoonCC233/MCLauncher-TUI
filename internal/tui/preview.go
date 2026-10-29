package tui

import (
	"time"

	"github.com/MoonCC233/MCLauncher-TUI/internal/auth"
	"github.com/MoonCC233/MCLauncher-TUI/internal/config"
	"github.com/MoonCC233/MCLauncher-TUI/internal/i18n"
	"github.com/MoonCC233/MCLauncher-TUI/internal/java"
	"github.com/MoonCC233/MCLauncher-TUI/internal/mojang"
	"github.com/MoonCC233/MCLauncher-TUI/internal/paths"
)

// Preview renders a deterministic frame with demo data. It is used by
// `mclauncher --preview` to generate documentation screenshots without
// touching the network or the real config. An optional language code selects
// a UI language (for example "zh-Hans" or "ar").
func Preview(screenName string, width, height int, language ...string) string {
	if width <= 0 {
		width = 120
	}
	if height <= 0 {
		height = 38
	}
	now := time.Now()
	cfg := &config.Config{
		DataVersion:       config.DataVersion,
		Settings:          config.DefaultSettings(),
		ActiveAccountUUID: "b50ad385-829d-3141-a216-7e7d7539ba7f",
		Accounts: []config.Account{
			{Username: "MoonCC", UUID: "b50ad385-829d-3141-a216-7e7d7539ba7f", Kind: "offline", CreatedAt: now.AddDate(0, -2, 0).Unix()},
			{Username: "Steve", UUID: "0f31b1f0e0c14f8b8a9c6d5e4f3a2b1c", Kind: "offline", CreatedAt: now.AddDate(0, 0, -9).Unix()},
			{Username: "Alex", UUID: "069a79f4-44e9-4726-a5be-fca90e38aaf5", Kind: "microsoft", ExpiresAt: now.Add(18*time.Hour + 24*time.Minute).Unix(), CreatedAt: now.AddDate(0, -4, 0).Unix()},
		},
		Instances: []config.Instance{
			{ID: "a1b2c3d4e5", Name: "Survival", VersionID: "1.21.4", MemoryMB: 6144, CreatedAt: now.AddDate(0, -1, 0).Unix(), LastPlayedAt: now.Add(-2 * 24 * time.Hour).Unix(), PlaySeconds: 12780},
			{ID: "f6e7d8c9b0", Name: "Creative Test", VersionID: "1.20.1", CreatedAt: now.AddDate(0, 0, -12).Unix(), LastPlayedAt: now.Add(-6 * 24 * time.Hour).Unix(), PlaySeconds: 3720},
			{ID: "99887766aa", Name: "Snapshot Lab", VersionID: "25w45a", MemoryMB: 8192, CreatedAt: now.AddDate(0, 0, -4).Unix()},
		},
	}
	cfg.Settings.JVMArgs = "-XX:+UseG1GC"
	if len(language) > 0 {
		cfg.Settings.Language = language[0]
	}

	layout := paths.New(`C:\Users\MoonCC\MCLauncher\.minecraft`)
	m := New(cfg, layout, nil, "0.2.0", "")
	m.width = width
	m.height = height
	m.ready = true
	m.manifest = demoManifest(now)
	m.manifestLoading = false
	m.manifestAt = now.Add(-12 * time.Minute)
	m.installed = map[string]bool{
		"1.21.4": true,
		"1.21.3": true,
		"1.20.1": true,
		"1.19.4": true,
		"1.18.2": true,
		"1.16.5": true,
		"1.12.2": true,
		"1.8.9":  true,
	}
	m.rebuildVersions()
	m.javaScanned = true
	m.javaScanning = false
	m.javaRuntimes = []java.Info{
		{Path: `C:\Program Files\Eclipse Adoptium\jdk-21.0.5.11-hotspot\bin\java.exe`, Version: "21.0.5", Major: 21, Vendor: "Temurin", Source: "Eclipse Adoptium"},
		{Path: `C:\Program Files\Microsoft\jdk-17.0.13.11-hotspot\bin\java.exe`, Version: "17.0.13", Major: 17, Vendor: "Microsoft", Source: "Microsoft"},
		{Path: `C:\Program Files\Java\jre1.8.0_431\bin\java.exe`, Version: "1.8.0_431", Major: 8, Vendor: "Oracle", Source: "Java"},
	}
	m.settingsCursor = 1
	m.versionCursor = 2
	m.ensureVersionVisible()
	m.instanceCursor = 0
	m.homeCursor = 0
	m.accountCursor = 2
	m.logs = nil
	push := func(source, text string, ago time.Duration) {
		m.logs = append(m.logs, LogLine{At: now.Add(-ago), Source: source, Text: text})
	}
	push("system", "mclauncher 0.2.0", 62*time.Second)
	push("launcher", "version manifest: network · 27 versions", 61*time.Second)
	push("launcher", "java scan: 3 runtimes", 60*time.Second)
	push("launcher", "installed 1.21.4 · 512 files · 283.4 MB", 59*time.Second)
	push("game", "[Render thread/INFO]: Setting user: MoonCC", 18*time.Second)
	push("game", "[Render thread/INFO]: Backend library: LWJGL version 3.3.3+5", 18*time.Second)
	push("game", "[Render thread/INFO]: OpenGL debug message: Using hardware acceleration", 17*time.Second)
	push("game", "[Worker-Main-1/INFO]: Loaded 1257 recipes", 15*time.Second)
	push("game", "[Worker-Main-2/INFO]: Preparing spawn area: 92%", 12*time.Second)
	push("game", "[Render thread/INFO]: Loaded 1257 advancements", 10*time.Second)
	push("game", "[Render thread/INFO]: Started 10 worker threads", 9*time.Second)
	push("game", "[Sound engine/INFO]: Sound engine started", 8*time.Second)
	push("game", "[Render thread/INFO]: Created: 1024x512x4 minecraft:textures/atlas/blocks.png-atlas", 7*time.Second)
	push("game", "[Render thread/INFO]: Created: 256x128x0 minecraft:textures/atlas/particles.png-atlas", 7*time.Second)
	push("game", "[Render thread/INFO]: Connecting to localhost:25565", 4*time.Second)
	push("game", "[Render thread/INFO]: Time elapsed: 1243 ms", 3*time.Second)

	m.game = &GameState{
		InstanceID:   "a1b2c3d4e5",
		InstanceName: "Survival",
		VersionID:    "1.21.4",
		Started:      now.Add(-3*time.Minute - 12*time.Second),
		Pid:          18472,
	}

	switch screenName {
	case "instances":
		m.screen = ScreenInstances
	case "versions":
		m.screen = ScreenVersions
	case "accounts":
		m.screen = ScreenAccounts
	case "settings":
		m.screen = ScreenSettings
	case "console":
		m.screen = ScreenConsole
	case "downloads":
		m.screen = ScreenDownloads
		m.task = &TaskState{
			Kind:         taskInstall,
			Title:        m.tf("Installing %s", "1.21.4"),
			VersionID:    "1.21.4",
			Stage:        "Downloading game files",
			Detail:       "museum_ui/bedrock_footprint.png",
			Done:         148_200_000,
			Total:        283_400_000,
			FilesDone:    271,
			FilesTotal:   512,
			Started:      now.Add(-32 * time.Second),
			Backgrounded: true,
		}
		m.taskHistory = []TaskRecord{
			{
				Kind:       taskInstall,
				Title:      m.tf("Installing %s", "1.21.4"),
				VersionID:  "1.21.4",
				Stage:      "Completed",
				Status:     taskStatusCompleted,
				Started:    now.Add(-26 * time.Minute),
				Finished:   now.Add(-24*time.Minute - 12*time.Second),
				Done:       283_400_000,
				Total:      283_400_000,
				FilesDone:  512,
				FilesTotal: 512,
			},
			{
				Kind:       taskInstall,
				Title:      m.tf("Installing %s", "1.20.1"),
				VersionID:  "1.20.1",
				Stage:      "Downloading game files",
				Status:     taskStatusCancelled,
				Started:    now.Add(-2*time.Hour - 8*time.Minute),
				Finished:   now.Add(-2*time.Hour - 6*time.Minute),
				Done:       42_100_000,
				Total:      244_800_000,
				FilesDone:  96,
				FilesTotal: 438,
			},
			{
				Kind:      taskInstall,
				Title:     m.tf("Installing %s", "1.21.3"),
				VersionID: "1.21.3",
				Stage:     "Downloading game files",
				Status:    taskStatusFailed,
				Started:   now.Add(-26 * time.Hour),
				Finished:  now.Add(-26*time.Hour + 41*time.Second),
				Err:       "download https://piston-data.mojang.com/...: connection reset by peer",
			},
		}
	case "theme":
		m.screen = ScreenSettings
		m.settingsCursor = 6
		m.openThemePicker()
		m.themePick = 2
	case "help":
		m.screen = ScreenHome
		m.modal = modalHelp
	case "progress":
		m.screen = ScreenConsole
		m.modal = modalProgress
		m.task = &TaskState{
			Kind:       taskInstall,
			Title:      m.tf("Installing %s", "1.21.4"),
			VersionID:  "1.21.4",
			Stage:      "Downloading game files",
			Detail:     "museum_ui/bedrock_footprint.png",
			Done:       148_200_000,
			Total:      283_400_000,
			FilesDone:  271,
			FilesTotal: 512,
			Started:    now.Add(-32 * time.Second),
		}
	case "form":
		m.screen = ScreenVersions
		m.openInstanceForm("1.21.4")
	case "language":
		m.screen = ScreenSettings
		m.openLanguagePicker()
		for i, info := range i18n.Supported {
			if info.Code == i18n.ZHHans {
				m.languagePick = i + 1
				break
			}
		}
	case "java":
		m.screen = ScreenSettings
		m.modal = modalJava
		m.javaPicker = 0
	case "account-type":
		m.screen = ScreenAccounts
		m.openAccountTypePicker()
	case "microsoft":
		m.screen = ScreenAccounts
		m.modal = modalMicrosoft
		m.ms = MicrosoftLoginState{
			Stage:    "waiting",
			ClientID: auth.DefaultMicrosoftClientID,
			Started:  now.Add(-38 * time.Second),
			Interval: 5 * time.Second,
			Code: &auth.DeviceCode{
				UserCode:        "K7QX-4M2P",
				VerificationURI: "https://www.microsoft.com/link",
				ExpiresIn:       900,
				Interval:        5,
			},
		}
	case "microsoft-error":
		m.screen = ScreenAccounts
		m.modal = modalMicrosoft
		m.ms = MicrosoftLoginState{
			Stage: "failed",
			Err:   "Xbox Live rejected the sign-in: this account has no Xbox profile; sign in at xbox.com once (XErr 2148916233)",
		}
	case "resolution":
		m.screen = ScreenSettings
		m.cfg.Settings.Width = 1920
		m.cfg.Settings.Height = 1080
		m.openResolutionPicker()
	default:
		m.screen = ScreenHome
	}
	return m.View()
}

func demoManifest(now time.Time) *mojang.Manifest {
	type demo struct {
		id   string
		kind string
		date string
	}
	items := []demo{
		{"25w45a", "snapshot", "2025-11-05T12:00:00+00:00"},
		{"25w14craftmine", "snapshot", "2025-04-01T12:00:00+00:00"},
		{"1.21.4", "release", "2024-12-03T10:12:00+00:00"},
		{"1.21.3", "release", "2024-10-23T12:00:00+00:00"},
		{"1.21.2", "release", "2024-10-22T11:00:00+00:00"},
		{"1.21.1", "release", "2024-08-08T14:00:00+00:00"},
		{"1.21", "release", "2024-06-13T10:00:00+00:00"},
		{"24w14potato", "snapshot", "2024-04-01T12:00:00+00:00"},
		{"1.20.6", "release", "2024-04-29T12:00:00+00:00"},
		{"1.20.4", "release", "2023-12-07T12:00:00+00:00"},
		{"1.20.2", "release", "2023-09-21T12:00:00+00:00"},
		{"1.20.1", "release", "2023-06-12T12:00:00+00:00"},
		{"1.19.4", "release", "2023-03-14T12:00:00+00:00"},
		{"1.19.2", "release", "2022-08-05T12:00:00+00:00"},
		{"1.18.2", "release", "2022-02-28T12:00:00+00:00"},
		{"1.17.1", "release", "2021-07-06T12:00:00+00:00"},
		{"1.16.5", "release", "2021-01-14T12:00:00+00:00"},
		{"20w14infinite", "snapshot", "2020-04-01T12:00:00+00:00"},
		{"1.15.2", "release", "2020-01-17T12:00:00+00:00"},
		{"1.14.4", "release", "2019-07-19T12:00:00+00:00"},
		{"3D Shareware v1.34", "release", "2019-04-01T12:00:00+00:00"},
		{"1.13.2", "release", "2018-10-22T12:00:00+00:00"},
		{"1.12.2", "release", "2017-09-18T12:00:00+00:00"},
		{"1.11.2", "release", "2016-12-21T12:00:00+00:00"},
		{"1.10.2", "release", "2016-06-23T12:00:00+00:00"},
		{"1.9.4", "release", "2016-05-10T12:00:00+00:00"},
		{"1.RV-Pre1", "snapshot", "2016-03-31T12:00:00+00:00"},
		{"1.8.9", "release", "2015-12-09T12:00:00+00:00"},
		{"15w14a", "snapshot", "2015-04-01T12:00:00+00:00"},
		{"1.7.10", "release", "2014-05-14T12:00:00+00:00"},
		{"1.6.4", "release", "2013-09-19T12:00:00+00:00"},
		{"1.5.2", "release", "2013-05-02T12:00:00+00:00"},
		{"2.0", "release", "2013-04-01T12:00:00+00:00"},
		{"1.4.7", "release", "2013-01-09T12:00:00+00:00"},
		{"1.3.2", "release", "2012-08-16T12:00:00+00:00"},
		{"1.2.5", "release", "2012-04-04T12:00:00+00:00"},
		{"1.1", "release", "2012-01-12T12:00:00+00:00"},
		{"1.0", "release", "2011-11-24T12:00:00+00:00"},
		{"Beta 1.7.3", "old_beta", "2011-07-08T12:00:00+00:00"},
		{"Alpha 1.2.6", "old_alpha", "2010-12-03T12:00:00+00:00"},
	}
	m := &mojang.Manifest{Source: "network", LatestRelease: "1.21.4", LatestSnapshot: "25w45a"}
	for _, item := range items {
		m.Versions = append(m.Versions, mojang.VersionSummary{
			ID:          item.id,
			Type:        item.kind,
			ReleaseTime: item.date,
			Latest:      item.id == "1.21.4",
		})
	}
	return m
}
