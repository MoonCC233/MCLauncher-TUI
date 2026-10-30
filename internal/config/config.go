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
	"github.com/MoonCC233/MCLauncher-TUI/internal/i18n"
	"github.com/MoonCC233/MCLauncher-TUI/internal/skin"
	"github.com/MoonCC233/MCLauncher-TUI/internal/util"
)

const DataVersion = 1

const DefaultTheme = "opencode"

var ThemeIDs = []string{"opencode", "midnight", "forest", "rose", "mono"}

func ValidTheme(id string) bool {
	for _, candidate := range ThemeIDs {
		if candidate == id {
			return true
		}
	}
	return false
}

// DefaultTitleAnim is the animation used by the home screen title.
const DefaultTitleAnim = "sheen"

// DefaultTitleFont is the glyph style used by the home screen title.
const DefaultTitleFont = "block"

// TitleFontIDs lists the selectable title fonts, in picker order.
var TitleFontIDs = []string{"block", "line"}

func ValidTitleFont(id string) bool {
	for _, candidate := range TitleFontIDs {
		if candidate == id {
			return true
		}
	}
	return false
}

// TitleAnimIDs lists the selectable title animations, in picker order.
var TitleAnimIDs = []string{"sheen", "wave", "breathe", "off"}

func ValidTitleAnim(id string) bool {
	for _, candidate := range TitleAnimIDs {
		if candidate == id {
			return true
		}
	}
	return false
}

// AccountSkin points at the custom skin stored next to the config file. Only
// offline accounts use it; Microsoft accounts keep their Mojang skin.
type AccountSkin struct {
	Model     string `json:"model,omitempty"`
	SHA256    string `json:"sha256,omitempty"`
	UpdatedAt int64  `json:"updated_at,omitempty"`
}

type Account struct {
	Username     string       `json:"username"`
	UUID         string       `json:"uuid"`
	Kind         string       `json:"kind"`
	AccessToken  string       `json:"access_token,omitempty"`
	RefreshToken string       `json:"refresh_token,omitempty"`
	XUID         string       `json:"xuid,omitempty"`
	ClientID     string       `json:"client_id,omitempty"`
	ExpiresAt    int64        `json:"expires_at,omitempty"`
	CreatedAt    int64        `json:"created_at"`
	Skin         *AccountSkin `json:"skin,omitempty"`
}

func (a Account) IsMicrosoft() bool { return a.Kind == "microsoft" }

func (a Account) UUIDHex() string { return strings.ReplaceAll(a.UUID, "-", "") }

// HasCustomSkin reports whether this account has a usable offline skin.
func (a Account) HasCustomSkin() bool {
	return !a.IsMicrosoft() && a.Skin != nil && a.Skin.SHA256 != ""
}

// SkinModel returns the model to send to the game.
func (a Account) SkinModel() string {
	if a.Skin == nil {
		return skin.ModelClassic
	}
	return skin.NormalizeModel(a.Skin.Model)
}

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
	MemoryMB          int    `json:"memory_mb"`
	JavaPath          string `json:"java_path"`
	JVMArgs           string `json:"jvm_args"`
	Width             int    `json:"width"`
	Height            int    `json:"height"`
	DownloadThreads   int    `json:"download_threads"`
	KeepGameRunning   bool   `json:"keep_game_running,omitempty"`
	Language          string `json:"language,omitempty"`
	Theme             string `json:"theme,omitempty"`
	ThemeAccent       string `json:"theme_accent,omitempty"`
	TitleAnim         string `json:"title_anim,omitempty"`
	TitleFont         string `json:"title_font,omitempty"`
	GameWindowTitle   string `json:"game_window_title,omitempty"`
	SeenAnnouncement  string `json:"seen_announcement,omitempty"`
	DataDir           string `json:"data_dir,omitempty"`
	MicrosoftClientID string `json:"microsoft_client_id,omitempty"`

	// preserved holds keys written by a newer launcher (see preserve.go).
	preserved *preservedFields
}

func DefaultSettings() Settings {
	return Settings{
		MemoryMB:        4096,
		Width:           854,
		Height:          480,
		DownloadThreads: 8,
		Theme:           DefaultTheme,
		TitleAnim:       DefaultTitleAnim,
		TitleFont:       DefaultTitleFont,
	}
}

type Config struct {
	Path              string     `json:"-"`
	DataVersion       int        `json:"data_version"`
	ActiveAccountUUID string     `json:"active_account_uuid"`
	Settings          Settings   `json:"settings"`
	Accounts          []Account  `json:"accounts"`
	Instances         []Instance `json:"instances"`

	// preserved holds keys written by a newer launcher (see preserve.go).
	preserved *preservedFields
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
	if c.Settings.Language != "" {
		if lang, ok := i18n.Canonical(c.Settings.Language); ok {
			c.Settings.Language = string(lang)
		} else {
			// An unknown language means "follow the system locale".
			c.Settings.Language = ""
		}
	}
	c.Settings.Theme = strings.ToLower(strings.TrimSpace(c.Settings.Theme))
	if !ValidTheme(c.Settings.Theme) {
		c.Settings.Theme = DefaultTheme
	}
	if accent, ok := util.NormalizeHexColor(c.Settings.ThemeAccent); ok {
		c.Settings.ThemeAccent = accent
	} else {
		c.Settings.ThemeAccent = ""
	}
	c.Settings.TitleAnim = strings.ToLower(strings.TrimSpace(c.Settings.TitleAnim))
	if !ValidTitleAnim(c.Settings.TitleAnim) {
		c.Settings.TitleAnim = DefaultTitleAnim
	}
	c.Settings.TitleFont = strings.ToLower(strings.TrimSpace(c.Settings.TitleFont))
	if !ValidTitleFont(c.Settings.TitleFont) {
		c.Settings.TitleFont = DefaultTitleFont
	}
	c.Settings.DataDir = strings.TrimSpace(c.Settings.DataDir)
	c.Settings.MicrosoftClientID = strings.TrimSpace(c.Settings.MicrosoftClientID)
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
		a.Kind = strings.TrimSpace(a.Kind)
		if a.Kind == "" {
			a.Kind = "offline"
		}
		if a.Kind == "microsoft" {
			a.UUID = auth.NormalizeUUID(a.UUID)
			if a.UUID == "" {
				continue
			}
			a.AccessToken = strings.TrimSpace(a.AccessToken)
			a.RefreshToken = strings.TrimSpace(a.RefreshToken)
			a.XUID = strings.TrimSpace(a.XUID)
			a.ClientID = strings.TrimSpace(a.ClientID)
			if a.ExpiresAt < 0 {
				a.ExpiresAt = 0
			}
			a.Skin = nil
		} else {
			a.Kind = "offline"
			if a.UUID == "" {
				a.UUID = auth.OfflineUUID(a.Username)
			}
			a.AccessToken = ""
			a.RefreshToken = ""
			a.XUID = ""
			a.ClientID = ""
			a.ExpiresAt = 0
			a.Skin = normalizeAccountSkin(a.Skin)
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

// FindAccount returns the account with uuid, or nil.
func (c *Config) FindAccount(uuid string) *Account {
	for i := range c.Accounts {
		if strings.EqualFold(c.Accounts[i].UUID, uuid) {
			return &c.Accounts[i]
		}
	}
	return nil
}

// SkinsDir is the custom-skin folder next to the config file. Keeping skins
// beside the bootstrap config means switching data directories never loses
// them.
func (c *Config) SkinsDir() string {
	if c.Path == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(c.Path), "skins")
}

// SkinFile is the PNG path for an account, or "" when it cannot be resolved.
func (c *Config) SkinFile(a Account) string {
	dir := c.SkinsDir()
	uuid := strings.ToLower(a.UUIDHex())
	if dir == "" || uuid == "" {
		return ""
	}
	return filepath.Join(dir, uuid+".png")
}

// SetAccountSkin stores skin metadata for an offline account.
func (c *Config) SetAccountSkin(uuid string, meta AccountSkin) error {
	for i := range c.Accounts {
		if !strings.EqualFold(c.Accounts[i].UUID, uuid) {
			continue
		}
		if c.Accounts[i].IsMicrosoft() {
			return fmt.Errorf("Microsoft accounts use Mojang skins")
		}
		normalized := normalizeAccountSkin(&meta)
		if normalized == nil {
			return fmt.Errorf("invalid skin metadata")
		}
		c.Accounts[i].Skin = normalized
		return c.Save()
	}
	return fmt.Errorf("account not found")
}

// ClearAccountSkin forgets an account's custom skin metadata.
func (c *Config) ClearAccountSkin(uuid string) error {
	for i := range c.Accounts {
		if !strings.EqualFold(c.Accounts[i].UUID, uuid) {
			continue
		}
		c.Accounts[i].Skin = nil
		return c.Save()
	}
	return fmt.Errorf("account not found")
}

func normalizeAccountSkin(s *AccountSkin) *AccountSkin {
	if s == nil {
		return nil
	}
	hash := strings.ToLower(strings.TrimSpace(s.SHA256))
	if len(hash) != 64 {
		return nil
	}
	if _, err := hex.DecodeString(hash); err != nil {
		return nil
	}
	s.Model = skin.NormalizeModel(strings.ToLower(strings.TrimSpace(s.Model)))
	s.SHA256 = hash
	if s.UpdatedAt < 0 {
		s.UpdatedAt = 0
	}
	return s
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

// AddMicrosoftAccount stores a fresh Microsoft login, replacing an existing
// entry for the same Minecraft profile.
func (c *Config) AddMicrosoftAccount(session auth.MinecraftSession) (*Account, error) {
	name := strings.TrimSpace(session.Username)
	if err := auth.ValidateUsername(name); err != nil {
		return nil, err
	}
	uuid := auth.NormalizeUUID(session.UUID)
	if uuid == "" {
		return nil, fmt.Errorf("Microsoft profile has no UUID")
	}
	if strings.TrimSpace(session.AccessToken) == "" {
		return nil, fmt.Errorf("Microsoft sign-in returned no Minecraft token")
	}
	account := Account{
		Username:     name,
		UUID:         uuid,
		Kind:         "microsoft",
		AccessToken:  session.AccessToken,
		RefreshToken: session.RefreshToken,
		XUID:         session.XUID,
		ClientID:     session.ClientID,
		ExpiresAt:    session.ExpiresAt,
		CreatedAt:    time.Now().Unix(),
	}
	out := c.Accounts[:0]
	for _, existing := range c.Accounts {
		sameUUID := strings.EqualFold(existing.UUID, uuid)
		sameName := strings.EqualFold(existing.Username, name)
		if sameUUID || sameName {
			if existing.CreatedAt > 0 {
				account.CreatedAt = existing.CreatedAt
			}
			if existing.HasCustomSkin() {
				_ = os.Remove(c.SkinFile(existing))
			}
			if c.ActiveAccountUUID == existing.UUID {
				c.ActiveAccountUUID = uuid
			}
			continue
		}
		out = append(out, existing)
	}
	c.Accounts = append(out, account)
	if c.ActiveAccountUUID == "" {
		c.ActiveAccountUUID = account.UUID
	}
	if err := c.Save(); err != nil {
		return nil, err
	}
	return &c.Accounts[len(c.Accounts)-1], nil
}

// UpdateMicrosoftAccount replaces the tokens of an existing account after a
// refresh. It returns false when the account disappeared.
func (c *Config) UpdateMicrosoftAccount(uuid string, session auth.MinecraftSession) (bool, error) {
	for i := range c.Accounts {
		if !strings.EqualFold(c.Accounts[i].UUID, uuid) {
			continue
		}
		c.Accounts[i].Kind = "microsoft"
		c.Accounts[i].Username = session.Username
		c.Accounts[i].UUID = auth.NormalizeUUID(session.UUID)
		c.Accounts[i].AccessToken = session.AccessToken
		c.Accounts[i].RefreshToken = session.RefreshToken
		c.Accounts[i].XUID = session.XUID
		c.Accounts[i].ClientID = session.ClientID
		c.Accounts[i].ExpiresAt = session.ExpiresAt
		c.Accounts[i].Skin = nil
		if c.ActiveAccountUUID == uuid {
			c.ActiveAccountUUID = c.Accounts[i].UUID
		}
		return true, c.Save()
	}
	return false, nil
}

func (c *Config) RemoveAccount(uuid string) error {
	skinPath := ""
	out := c.Accounts[:0]
	for _, a := range c.Accounts {
		if a.UUID != uuid {
			out = append(out, a)
			continue
		}
		if a.HasCustomSkin() {
			skinPath = c.SkinFile(a)
		}
	}
	c.Accounts = out
	if skinPath != "" {
		_ = os.Remove(skinPath)
	}
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
		// Roll the in-memory addition back so the UI never shows an instance
		// that did not reach disk.
		c.Instances = c.Instances[:len(c.Instances)-1]
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
