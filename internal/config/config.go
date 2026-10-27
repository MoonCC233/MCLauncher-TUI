package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MoonCC233/MCLauncher-TUI/internal/auth"
	"github.com/MoonCC233/MCLauncher-TUI/internal/util"
)

const DataVersion = 1

type Account struct {
	Username  string `json:"username"`
	UUID      string `json:"uuid"`
	Kind      string `json:"kind"`
	CreatedAt int64  `json:"created_at"`
}

func (a Account) UUIDHex() string { return strings.ReplaceAll(a.UUID, "-", "") }

type Instance struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	VersionID    string `json:"version_id"`
	MemoryMB     int    `json:"memory_mb,omitempty"`
	CreatedAt    int64  `json:"created_at"`
	LastPlayedAt int64  `json:"last_played_at,omitempty"`
	PlaySeconds  int64  `json:"play_seconds,omitempty"`
}

type Settings struct {
	MemoryMB        int    `json:"memory_mb"`
	JavaPath        string `json:"java_path"`
	JVMArgs         string `json:"jvm_args"`
	Width           int    `json:"width"`
	Height          int    `json:"height"`
	DownloadThreads int    `json:"download_threads"`
}

func DefaultSettings() Settings {
	return Settings{
		MemoryMB:        4096,
		Width:           854,
		Height:          480,
		DownloadThreads: 8,
	}
}

type Config struct {
	Path              string     `json:"-"`
	DataVersion       int        `json:"data_version"`
	ActiveAccountUUID string     `json:"active_account_uuid"`
	Settings          Settings   `json:"settings"`
	Accounts          []Account  `json:"accounts"`
	Instances         []Instance `json:"instances"`
}

func Load(path string) (*Config, error) {
	cfg := &Config{
		Path:        path,
		DataVersion: DataVersion,
		Settings:    DefaultSettings(),
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	if err := util.ReadJSON(path, cfg); err != nil {
		return nil, err
	}
	cfg.Path = path
	cfg.normalize()
	return cfg, nil
}

func (c *Config) normalize() {
	if c.DataVersion <= 0 {
		c.DataVersion = DataVersion
	}
	if c.Settings.MemoryMB < 512 {
		c.Settings.MemoryMB = 4096
	}
	if c.Settings.MemoryMB > 65536 {
		c.Settings.MemoryMB = 65536
	}
	if c.Settings.Width < 320 || c.Settings.Width > 7680 {
		c.Settings.Width = 854
	}
	if c.Settings.Height < 240 || c.Settings.Height > 4320 {
		c.Settings.Height = 480
	}
	if c.Settings.DownloadThreads < 1 || c.Settings.DownloadThreads > 32 {
		c.Settings.DownloadThreads = 8
	}
	// Keep only sane accounts, ignoring duplicates by UUID or (because
	// Minecraft names are case-insensitive) by username.
	accounts := make([]Account, 0, len(c.Accounts))
	seen := map[string]bool{}
	seenName := map[string]bool{}
	for _, a := range c.Accounts {
		a.Username = strings.TrimSpace(a.Username)
		if auth.ValidateUsername(a.Username) != nil {
			continue
		}
		if a.UUID == "" {
			a.UUID = auth.OfflineUUID(a.Username)
		}
		if a.Kind == "" {
			a.Kind = "offline"
		}
		key := strings.ToLower(a.UUID)
		nameKey := strings.ToLower(a.Username)
		if seen[key] || seenName[nameKey] {
			continue
		}
		seen[key] = true
		seenName[nameKey] = true
		accounts = append(accounts, a)
	}
	c.Accounts = accounts
	instances := make([]Instance, 0, len(c.Instances))
	seenI := map[string]bool{}
	for _, inst := range c.Instances {
		inst.Name = strings.TrimSpace(inst.Name)
		inst.VersionID = strings.TrimSpace(inst.VersionID)
		if inst.ID == "" || inst.Name == "" || inst.VersionID == "" || seenI[inst.ID] {
			continue
		}
		seenI[inst.ID] = true
		inst.MemoryMB = util.Clamp(inst.MemoryMB, 0, 65536)
		instances = append(instances, inst)
	}
	c.Instances = instances
	if c.ActiveAccount() == nil {
		if len(c.Accounts) > 0 {
			c.ActiveAccountUUID = c.Accounts[0].UUID
		} else {
			c.ActiveAccountUUID = ""
		}
	}
}

func (c *Config) Save() error {
	if c.Path == "" {
		return fmt.Errorf("config has no path")
	}
	c.DataVersion = DataVersion
	return util.WriteJSONAtomic(c.Path, c)
}

func (c *Config) ActiveAccount() *Account {
	for i := range c.Accounts {
		if c.Accounts[i].UUID == c.ActiveAccountUUID {
			return &c.Accounts[i]
		}
	}
	return nil
}

func (c *Config) SetActiveAccount(uuid string) {
	c.ActiveAccountUUID = uuid
}

func (c *Config) AddAccount(name string) (*Account, error) {
	name = strings.TrimSpace(name)
	if err := auth.ValidateUsername(name); err != nil {
		return nil, err
	}
	for _, a := range c.Accounts {
		if strings.EqualFold(a.Username, name) {
			return nil, fmt.Errorf("account %q already exists", name)
		}
	}
	account := Account{
		Username:  name,
		UUID:      auth.OfflineUUID(name),
		Kind:      "offline",
		CreatedAt: time.Now().Unix(),
	}
	c.Accounts = append(c.Accounts, account)
	if c.ActiveAccountUUID == "" {
		c.ActiveAccountUUID = account.UUID
	}
	if err := c.Save(); err != nil {
		return nil, err
	}
	return &c.Accounts[len(c.Accounts)-1], nil
}

func (c *Config) RemoveAccount(uuid string) error {
	out := c.Accounts[:0]
	for _, a := range c.Accounts {
		if a.UUID != uuid {
			out = append(out, a)
		}
	}
	c.Accounts = out
	if c.ActiveAccountUUID == uuid {
		c.ActiveAccountUUID = ""
		if len(c.Accounts) > 0 {
			c.ActiveAccountUUID = c.Accounts[0].UUID
		}
	}
	return c.Save()
}

func (c *Config) MemoryFor(inst *Instance) int {
	if inst != nil && inst.MemoryMB > 0 {
		return inst.MemoryMB
	}
	return c.Settings.MemoryMB
}

func (c *Config) AddInstance(name, versionID string, memoryMB int) (*Instance, error) {
	name = strings.TrimSpace(name)
	versionID = strings.TrimSpace(versionID)
	if name == "" {
		return nil, fmt.Errorf("instance name cannot be empty")
	}
	if len([]rune(name)) > 48 {
		return nil, fmt.Errorf("instance name is too long (max 48 characters)")
	}
	if versionID == "" {
		return nil, fmt.Errorf("version cannot be empty")
	}
	id, err := newID()
	if err != nil {
		return nil, err
	}
	inst := Instance{
		ID:        id,
		Name:      name,
		VersionID: versionID,
		MemoryMB:  util.Clamp(memoryMB, 0, 65536),
		CreatedAt: time.Now().Unix(),
	}
	c.Instances = append(c.Instances, inst)
	if err := c.Save(); err != nil {
		return nil, err
	}
	return &c.Instances[len(c.Instances)-1], nil
}

func (c *Config) UpdateInstance(inst *Instance, name, versionID string, memoryMB int) error {
	name = strings.TrimSpace(name)
	versionID = strings.TrimSpace(versionID)
	if name == "" {
		return fmt.Errorf("instance name cannot be empty")
	}
	if len([]rune(name)) > 48 {
		return fmt.Errorf("instance name is too long (max 48 characters)")
	}
	if versionID == "" {
		return fmt.Errorf("version cannot be empty")
	}
	inst.Name = name
	inst.VersionID = versionID
	inst.MemoryMB = util.Clamp(memoryMB, 0, 65536)
	return c.Save()
}

func (c *Config) RemoveInstance(inst *Instance) error {
	out := c.Instances[:0]
	for _, i := range c.Instances {
		if i.ID != inst.ID {
			out = append(out, i)
		}
	}
	c.Instances = out
	return c.Save()
}

func (c *Config) FindInstance(id string) *Instance {
	for i := range c.Instances {
		if c.Instances[i].ID == id {
			return &c.Instances[i]
		}
	}
	return nil
}

func (c *Config) TouchInstance(inst *Instance, startedAt time.Time, played time.Duration) error {
	inst.LastPlayedAt = startedAt.Unix()
	if played > 0 {
		inst.PlaySeconds += int64(played.Seconds())
	}
	return c.Save()
}

func newID() (string, error) {
	buf := make([]byte, 5)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func BackupCorrupt(path string) string {
	if _, err := os.Stat(path); err != nil {
		return ""
	}
	target := filepath.Join(filepath.Dir(path), fmt.Sprintf("config.corrupt-%d.json", time.Now().Unix()))
	if err := os.Rename(path, target); err != nil {
		return ""
	}
	return target
}
