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
