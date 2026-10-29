package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MoonCC233/MCLauncher-TUI/internal/auth"
)

func TestLoadMissingUsesDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Path != path {
		t.Errorf("Path = %q, want %q", cfg.Path, path)
	}
	if cfg.DataVersion != DataVersion {
		t.Errorf("DataVersion = %d, want %d", cfg.DataVersion, DataVersion)
	}
	if cfg.Settings != DefaultSettings() {
		t.Errorf("Settings = %+v, want %+v", cfg.Settings, DefaultSettings())
	}
	if len(cfg.Accounts) != 0 || len(cfg.Instances) != 0 {
		t.Error("fresh config should have no accounts or instances")
	}
}

func TestNormalize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	raw := `{
  "data_version": 0,
  "active_account_uuid": "missing",
  "settings": {
    "memory_mb": 1,
    "width": 1,
    "height": 99999,
    "download_threads": 100
  },
  "accounts": [
    {"username": " Alice ", "uuid": "", "kind": ""},
    {"username": "alice", "uuid": "", "kind": "offline"},
    {"username": "bad name", "uuid": "x", "kind": "offline"},
    {"username": "Bob", "uuid": "custom-uuid", "kind": "offline"},
    {"username": "Bob", "uuid": "CUSTOM-UUID", "kind": "offline"}
  ],
  "instances": [
    {"id": "a", "name": " Alpha ", "version_id": "1.21.4", "memory_mb": -5},
    {"id": "a", "name": "duplicate", "version_id": "1.20.1"},
    {"id": "", "name": "no id", "version_id": "1.20.1"},
    {"id": "b", "name": "", "version_id": "1.20.1"},
    {"id": "c", "name": "no version", "version_id": ""},
    {"id": "d", "name": "Big", "version_id": "1.20.1", "memory_mb": 100000}
  ]
}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DataVersion != DataVersion {
		t.Errorf("DataVersion = %d, want %d", cfg.DataVersion, DataVersion)
	}
	if cfg.Settings.MemoryMB != 4096 || cfg.Settings.Width != 854 || cfg.Settings.Height != 480 || cfg.Settings.DownloadThreads != 8 {
		t.Errorf("settings were not normalized: %+v", cfg.Settings)
	}

	if len(cfg.Accounts) != 2 {
		t.Fatalf("accounts = %+v, want 2 entries", cfg.Accounts)
	}
	alice := cfg.Accounts[0]
	if alice.Username != "Alice" || alice.UUID != auth.OfflineUUID("Alice") || alice.Kind != "offline" {
		t.Errorf("Alice was not normalized: %+v", alice)
	}
	if cfg.Accounts[1].Username != "Bob" || cfg.Accounts[1].UUID != "custom-uuid" {
		t.Errorf("Bob was not preserved: %+v", cfg.Accounts[1])
	}
	if cfg.ActiveAccountUUID != alice.UUID {
		t.Errorf("ActiveAccountUUID = %q, want %q", cfg.ActiveAccountUUID, alice.UUID)
	}

	if len(cfg.Instances) != 2 {
		t.Fatalf("instances = %+v, want 2 entries", cfg.Instances)
	}
	if cfg.Instances[0].Name != "Alpha" || cfg.Instances[0].MemoryMB != 0 {
		t.Errorf("Alpha was not normalized: %+v", cfg.Instances[0])
	}
	if cfg.Instances[1].ID != "d" || cfg.Instances[1].MemoryMB != 65536 {
		t.Errorf("Big was not clamped: %+v", cfg.Instances[1])
	}
}

func TestAccountLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cfg.AddAccount("ab"); err == nil {
		t.Error("AddAccount(ab) = nil error, want validation error")
	}

	first, err := cfg.AddAccount("MoonCC")
	if err != nil {
		t.Fatalf("AddAccount: %v", err)
	}
	if first.UUID != auth.OfflineUUID("MoonCC") || first.Kind != "offline" || first.CreatedAt == 0 {
		t.Errorf("unexpected account: %+v", first)
	}
	if cfg.ActiveAccountUUID != first.UUID {
		t.Errorf("first account should become active, got %q", cfg.ActiveAccountUUID)
	}
	if _, err := cfg.AddAccount("mooncc"); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("duplicate AddAccount error = %v, want already exists", err)
	}

	second, err := cfg.AddAccount("Steve")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ActiveAccountUUID != first.UUID {
		t.Error("adding an account should not steal the active slot")
	}

	// Everything must survive a reload.
	reloaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(reloaded.Accounts) != 2 {
		t.Fatalf("reloaded accounts = %+v", reloaded.Accounts)
	}
	if got := reloaded.ActiveAccount(); got == nil || got.Username != "MoonCC" {
		t.Errorf("reloaded active account = %+v", got)
	}

	if err := reloaded.RemoveAccount(first.UUID); err != nil {
		t.Fatal(err)
	}
	if got := reloaded.ActiveAccount(); got == nil || got.UUID != second.UUID {
		t.Errorf("after remove, active account = %+v, want Steve", got)
	}
	if err := reloaded.RemoveAccount(second.UUID); err != nil {
		t.Fatal(err)
	}
	if reloaded.ActiveAccountUUID != "" || reloaded.ActiveAccount() != nil {
		t.Errorf("removing every account should clear the active slot, got %q", reloaded.ActiveAccountUUID)
	}

	reloaded.SetActiveAccount("does-not-exist")
	if reloaded.ActiveAccount() != nil {
		t.Error("ActiveAccount should ignore an unknown UUID")
	}
}

func TestMicrosoftAccountLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := cfg.AddMicrosoftAccount(auth.MinecraftSession{Username: "MoonCC"}); err == nil {
		t.Error("AddMicrosoftAccount without a UUID should fail")
	}
	if _, err := cfg.AddMicrosoftAccount(auth.MinecraftSession{
		Username: "MoonCC",
		UUID:     "b50ad385829d3141a2167e7d7539ba7f",
	}); err == nil {
		t.Error("AddMicrosoftAccount without a Minecraft token should fail")
	}

	now := time.Now().Add(time.Hour).Unix()
	session := auth.MinecraftSession{
		Username:     "MoonCC",
		UUID:         "B50AD385829D3141A2167E7D7539BA7F",
		AccessToken:  "mc-token-1",
		RefreshToken: "ms-refresh-1",
		XUID:         "2535400000000001",
		ClientID:     auth.DefaultMicrosoftClientID,
		ExpiresAt:    now,
	}
	first, err := cfg.AddMicrosoftAccount(session)
	if err != nil {
		t.Fatalf("AddMicrosoftAccount: %v", err)
	}
	if first.Kind != "microsoft" || first.AccessToken != "mc-token-1" || first.RefreshToken != "ms-refresh-1" {
		t.Errorf("unexpected Microsoft account: %+v", first)
	}
	if first.UUID != "b50ad385-829d-3141-a216-7e7d7539ba7f" {
		t.Errorf("UUID = %q, want normalized UUID", first.UUID)
	}
	if !first.IsMicrosoft() {
		t.Error("IsMicrosoft() = false")
	}
	if cfg.ActiveAccountUUID != first.UUID {
		t.Errorf("ActiveAccountUUID = %q, want first account", cfg.ActiveAccountUUID)
	}
	createdAt := first.CreatedAt

	// Signing in again for the same profile replaces the tokens in place and
	// keeps the original creation time.
	session.AccessToken = "mc-token-2"
	replaced, err := cfg.AddMicrosoftAccount(session)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Accounts) != 1 {
		t.Fatalf("accounts = %+v, want a single entry", cfg.Accounts)
	}
	if replaced.AccessToken != "mc-token-2" || replaced.CreatedAt != createdAt {
		t.Errorf("replacement = %+v, want new token and original CreatedAt %d", replaced, createdAt)
	}

	second, err := cfg.AddMicrosoftAccount(auth.MinecraftSession{
		Username:     "Alex",
		UUID:         "069a79f444e94726a5befca90e38aaf5",
		AccessToken:  "mc-token-3",
		RefreshToken: "ms-refresh-3",
		XUID:         "2535400000000003",
		ClientID:     auth.DefaultMicrosoftClientID,
		ExpiresAt:    now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Accounts) != 2 {
		t.Fatalf("accounts = %+v, want two entries", cfg.Accounts)
	}
	if cfg.ActiveAccountUUID != first.UUID {
		t.Error("adding another Microsoft account should not steal the active slot")
	}

	refreshed := auth.MinecraftSession{
		Username:     "MoonCC",
		UUID:         "b50ad385829d3141a2167e7d7539ba7f",
		AccessToken:  "mc-token-4",
		RefreshToken: "ms-refresh-4",
		XUID:         "2535400000000001",
		ClientID:     auth.DefaultMicrosoftClientID,
		ExpiresAt:    now + 3600,
	}
	updated, err := cfg.UpdateMicrosoftAccount(first.UUID, refreshed)
	if err != nil || !updated {
		t.Fatalf("UpdateMicrosoftAccount = %v, %v", updated, err)
	}
	if cfg.Accounts[0].AccessToken != "mc-token-4" || cfg.Accounts[0].RefreshToken != "ms-refresh-4" {
		t.Errorf("refresh was not stored: %+v", cfg.Accounts[0])
	}
	if cfg.Accounts[1].UUID != second.UUID {
		t.Errorf("unrelated account was modified: %+v", cfg.Accounts[1])
	}
	if updated, err := cfg.UpdateMicrosoftAccount("does-not-exist", refreshed); err != nil || updated {
		t.Errorf("UpdateMicrosoftAccount(unknown) = %v, %v", updated, err)
	}

	reloaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(reloaded.Accounts) != 2 {
		t.Fatalf("reloaded accounts = %+v", reloaded.Accounts)
	}
	got := reloaded.Accounts[0]
	if got.AccessToken != "mc-token-4" || got.XUID != "2535400000000001" {
		t.Errorf("reloaded Microsoft account = %+v", got)
	}
}

func TestInstanceLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := cfg.AddInstance("", "1.21.4", 0); err == nil {
		t.Error("empty name should be rejected")
	}
	if _, err := cfg.AddInstance(strings.Repeat("x", 49), "1.21.4", 0); err == nil {
		t.Error("overlong name should be rejected")
	}
	if _, err := cfg.AddInstance("Survival", "", 0); err == nil {
		t.Error("empty version should be rejected")
	}

	inst, err := cfg.AddInstance(" Survival ", "1.21.4", 100000)
	if err != nil {
		t.Fatalf("AddInstance: %v", err)
	}
	if inst.Name != "Survival" || inst.VersionID != "1.21.4" || inst.MemoryMB != 65536 {
		t.Errorf("unexpected instance: %+v", inst)
	}
	if len(inst.ID) != 10 {
		t.Errorf("instance ID = %q, want 10 hex characters", inst.ID)
	}
	if cfg.FindInstance(inst.ID) == nil {
		t.Error("FindInstance could not find the new instance")
	}
	if cfg.MemoryFor(inst) != 65536 {
		t.Errorf("MemoryFor(instance) = %d, want 65536", cfg.MemoryFor(inst))
	}
	inst.MemoryMB = 0
	if cfg.MemoryFor(inst) != cfg.Settings.MemoryMB {
		t.Errorf("MemoryFor(instance without override) = %d, want %d", cfg.MemoryFor(inst), cfg.Settings.MemoryMB)
	}
	if cfg.MemoryFor(nil) != cfg.Settings.MemoryMB {
		t.Error("MemoryFor(nil) should fall back to global settings")
	}

	if err := cfg.UpdateInstance(inst, "", "1.20.1", 0); err == nil {
		t.Error("UpdateInstance with an empty name should fail")
	}
	if err := cfg.UpdateInstance(inst, "Renamed", "", 0); err == nil {
		t.Error("UpdateInstance with an empty version should fail")
	}
	if err := cfg.UpdateInstance(inst, "Renamed", "1.20.1", 1024); err != nil {
		t.Fatalf("UpdateInstance: %v", err)
	}
	if inst.Name != "Renamed" || inst.VersionID != "1.20.1" || inst.MemoryMB != 1024 {
		t.Errorf("instance was not updated: %+v", inst)
	}

	started := time.Now().Add(-5 * time.Minute)
	if err := cfg.TouchInstance(inst, started, 90*time.Second); err != nil {
		t.Fatal(err)
	}
	if inst.LastPlayedAt != started.Unix() || inst.PlaySeconds != 90 {
		t.Errorf("TouchInstance = last %d, play %d", inst.LastPlayedAt, inst.PlaySeconds)
	}
	if err := cfg.TouchInstance(inst, started, 30*time.Second); err != nil {
		t.Fatal(err)
	}
	if inst.PlaySeconds != 120 {
		t.Errorf("PlaySeconds = %d, want 120", inst.PlaySeconds)
	}

	reloaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	got := reloaded.FindInstance(inst.ID)
	if got == nil || got.Name != "Renamed" || got.PlaySeconds != 120 {
		t.Errorf("reloaded instance = %+v", got)
	}

	if err := reloaded.RemoveInstance(got); err != nil {
		t.Fatal(err)
	}
	if reloaded.FindInstance(inst.ID) != nil {
		t.Error("instance was not removed")
	}
}

func TestSaveWithoutPath(t *testing.T) {
	if err := (&Config{}).Save(); err == nil {
		t.Error("Save without a path should fail")
	}
}

func TestLanguageNormalization(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"en", "en"},
		{"EN_us", "en"},
		{"zh-CN", "zh-Hans"},
		{"zh_TW.UTF-8", "zh-Hant"},
		{"pt", "pt-BR"},
		{"pt_PT.UTF-8", "pt-PT"},
		{"pt-BR", "pt-BR"},
		{"de-AT", "de"},
		{"ja_JP", "ja"},
		{"offline", ""},
		{"xx-YY", ""},
	}
	for _, tc := range cases {
		path := filepath.Join(t.TempDir(), "config.json")
		raw := `{"settings":{"language":` + quoteJSON(tc.in) + `}}`
		if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
			t.Fatal(err)
		}
		cfg, err := Load(path)
		if err != nil {
			t.Fatalf("Load(%q): %v", tc.in, err)
		}
		if cfg.Settings.Language != tc.want {
			t.Errorf("language %q normalized to %q, want %q", tc.in, cfg.Settings.Language, tc.want)
		}
	}
}

func quoteJSON(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func TestLanguagePersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Settings.Language = "ru"
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Settings.Language != "ru" {
		t.Errorf("reloaded language = %q, want ru", reloaded.Settings.Language)
	}
}

func TestBackupCorrupt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if BackupCorrupt(path) != "" {
		t.Error("BackupCorrupt(missing) should return an empty path")
	}
	if err := os.WriteFile(path, []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	backup := BackupCorrupt(path)
	if backup == "" {
		t.Fatal("BackupCorrupt returned an empty path")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("original config should have been renamed")
	}
	data, err := os.ReadFile(backup)
	if err != nil || string(data) != "{broken" {
		t.Errorf("backup content = %q, err = %v", data, err)
	}
}
func TestThemeNormalization(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	raw := `{"settings":{"theme":"bogus","theme_accent":"#abc"}}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Settings.Theme != DefaultTheme {
		t.Errorf("invalid theme = %q, want %q", cfg.Settings.Theme, DefaultTheme)
	}
	if cfg.Settings.ThemeAccent != "#AABBCC" {
		t.Errorf("accent = %q, want #AABBCC", cfg.Settings.ThemeAccent)
	}

	cfg.Settings.Theme = "forest"
	cfg.Settings.ThemeAccent = "invalid"
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Settings.Theme != "forest" {
		t.Errorf("theme = %q, want forest", reloaded.Settings.Theme)
	}
	if reloaded.Settings.ThemeAccent != "" {
		t.Errorf("invalid accent should be cleared, got %q", reloaded.Settings.ThemeAccent)
	}
}

func TestTitleAnimNormalization(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if cfg := DefaultSettings(); cfg.TitleAnim != DefaultTitleAnim {
		t.Fatalf("default title animation = %q, want %q", cfg.TitleAnim, DefaultTitleAnim)
	}
	raw := `{"settings":{"title_anim":"BOGUS"}}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Settings.TitleAnim != DefaultTitleAnim {
		t.Errorf("invalid title animation = %q, want %q", cfg.Settings.TitleAnim, DefaultTitleAnim)
	}

	cfg.Settings.TitleAnim = "WAVE"
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Settings.TitleAnim != "wave" {
		t.Errorf("title animation = %q, want wave", reloaded.Settings.TitleAnim)
	}
}

func TestTitleFontNormalization(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if cfg := DefaultSettings(); cfg.TitleFont != DefaultTitleFont {
		t.Fatalf("default title font = %q, want %q", cfg.TitleFont, DefaultTitleFont)
	}
	raw := `{"settings":{"title_font":"BOGUS"}}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Settings.TitleFont != DefaultTitleFont {
		t.Errorf("invalid title font = %q, want %q", cfg.Settings.TitleFont, DefaultTitleFont)
	}

	cfg.Settings.TitleFont = "LINE"
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Settings.TitleFont != "line" {
		t.Errorf("title font = %q, want line", reloaded.Settings.TitleFont)
	}
}

func TestUnknownFieldsSurviveSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	raw := `{
  "data_version": 1,
  "future_top_level": {"nested": true},
  "settings": {
    "memory_mb": 2048,
    "title_font": "line",
    "future_setting": "keep me"
  }
}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Settings.TitleFont != "line" {
		t.Fatalf("title font = %q, want line", cfg.Settings.TitleFont)
	}

	// A later change rewrites the file; the keys this build does not know about
	// have to survive, otherwise a newer launcher's settings are lost.
	cfg.Settings.MemoryMB = 3072
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var saved map[string]any
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if _, ok := saved["future_top_level"]; !ok {
		t.Errorf("unknown top-level key was dropped: %s", data)
	}
	settings, ok := saved["settings"].(map[string]any)
	if !ok {
		t.Fatalf("settings missing: %s", data)
	}
	if settings["future_setting"] != "keep me" {
		t.Errorf("unknown setting was dropped: %s", data)
	}
	if settings["title_font"] != "line" {
		t.Errorf("title font = %v, want line", settings["title_font"])
	}
	if settings["memory_mb"] != float64(3072) {
		t.Errorf("memory = %v, want 3072", settings["memory_mb"])
	}
	if _, ok := saved["preserved"]; ok {
		t.Errorf("internal field leaked into the file: %s", data)
	}
}

func TestUnknownFieldsSurviveReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	raw := `{"settings":{"title_anim":"wave","from_the_future":1}}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		cfg, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Settings.TitleAnim != "wave" {
			t.Fatalf("round %d: title animation = %q, want wave", i, cfg.Settings.TitleAnim)
		}
		if err := cfg.Save(); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "from_the_future") {
		t.Errorf("unknown setting was lost across reloads: %s", data)
	}
}
