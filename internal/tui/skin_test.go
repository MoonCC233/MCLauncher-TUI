package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MoonCC233/MCLauncher-TUI/internal/auth"
	"github.com/MoonCC233/MCLauncher-TUI/internal/config"
	"github.com/MoonCC233/MCLauncher-TUI/internal/skin"
)

func skinKey(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func modelWithOfflineAccount(t *testing.T) (*Model, *config.Account) {
	t.Helper()
	m := newTestModel(t)
	account, err := m.cfg.AddAccount("MoonCC")
	if err != nil {
		t.Fatal(err)
	}
	m.screen = ScreenAccounts
	m.accountCursor = 0
	m.accountOffset = 0
	m.cfg.SetActiveAccount(account.UUID)
	return m, account
}

func importTestSkin(t *testing.T, m *Model) {
	t.Helper()
	src := filepath.Join(t.TempDir(), "source.png")
	if err := skin.Default(skin.ModelClassic).Save(src); err != nil {
		t.Fatal(err)
	}
	m.updateAccounts(skinKey("s"))
	if m.modal != modalSkin {
		t.Fatalf("modal = %v, want modalSkin", m.modal)
	}
	m.updateSkinModal(skinKey("i"))
	if m.modal != modalPrompt || m.prompt.Kind != promptSkinPath {
		t.Fatalf("prompt = %v kind %v", m.modal, m.prompt.Kind)
	}
	m.prompt.Input.SetValue(src)
	if model, _ := m.updatePrompt(tea.KeyMsg{Type: tea.KeyEnter}); model == nil {
		t.Fatal("updatePrompt returned nil model")
	}
	if m.modal != modalSkin {
		t.Fatalf("modal after import = %v, want modalSkin", m.modal)
	}
	if m.skinModal.preview == nil {
		t.Fatal("preview was not loaded")
	}
}

func TestSkinModalOpensForOfflineAccount(t *testing.T) {
	m, account := modelWithOfflineAccount(t)
	m.updateAccounts(skinKey("s"))
	if m.modal != modalSkin {
		t.Fatalf("modal = %v, want modalSkin", m.modal)
	}
	lines := plainView(m)
	if !strings.Contains(strings.Join(lines, "\n"), account.Username) {
		t.Error("modal does not show the account name")
	}
	if !strings.Contains(strings.Join(lines, "\n"), m.t("No custom skin yet")) {
		t.Error("modal does not explain that there is no custom skin")
	}
	frameLines(t, m.View(), m.width, m.height)
}

func TestSkinImportStoresFileAndMetadata(t *testing.T) {
	m, account := modelWithOfflineAccount(t)
	importTestSkin(t, m)

	path := m.cfg.SkinFile(*account)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("skin file was not written: %v", err)
	}
	stored := m.cfg.Accounts[0]
	if !stored.HasCustomSkin() {
		t.Fatal("account metadata was not updated")
	}
	loaded, err := skin.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	hash, err := loaded.Hash()
	if err != nil {
		t.Fatal(err)
	}
	if stored.Skin.SHA256 != hash {
		t.Errorf("stored hash = %s, want %s", stored.Skin.SHA256, hash)
	}
	if stored.Skin.Model != skin.ModelClassic {
		t.Errorf("model = %s, want classic", stored.Skin.Model)
	}

	// The metadata survives a reload of the config.
	reloaded, err := config.Load(m.cfg.Path)
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.FindAccount(account.UUID); got == nil || !got.HasCustomSkin() {
		t.Errorf("reloaded account lost its skin: %+v", got)
	}
}

func TestSkinImportRejectsNonPNG(t *testing.T) {
	m, _ := modelWithOfflineAccount(t)
	m.updateAccounts(skinKey("s"))
	m.updateSkinModal(skinKey("i"))
	m.prompt.Input.SetValue(filepath.Join(t.TempDir(), "missing.png"))
	if _, _ = m.updatePrompt(tea.KeyMsg{Type: tea.KeyEnter}); m.modal != modalPrompt {
		t.Fatalf("modal after failure = %v, want modalPrompt", m.modal)
	}
	if m.prompt.Err == "" {
		t.Error("prompt did not report the failure")
	}
}

func TestSkinModelToggleAndReload(t *testing.T) {
	m, account := modelWithOfflineAccount(t)
	importTestSkin(t, m)

	m.updateSkinModal(skinKey("m"))
	if got := m.cfg.Accounts[0].SkinModel(); got != skin.ModelSlim {
		t.Fatalf("model = %s, want slim", got)
	}
	m.updateSkinModal(skinKey("m"))
	if got := m.cfg.Accounts[0].SkinModel(); got != skin.ModelClassic {
		t.Fatalf("model = %s, want classic", got)
	}

	// Editing the PNG outside the launcher and pressing r refreshes the hash.
	path := m.cfg.SkinFile(*account)
	replacement := skin.Default(skin.ModelSlim)
	if err := replacement.Save(path); err != nil {
		t.Fatal(err)
	}
	want, err := replacement.Hash()
	if err != nil {
		t.Fatal(err)
	}
	m.updateSkinModal(skinKey("r"))
	if got := m.cfg.Accounts[0].Skin.SHA256; got != want {
		t.Errorf("hash after reload = %s, want %s", got, want)
	}
	if m.skinModal.preview == nil {
		t.Error("preview was cleared after reload")
	}
}

func TestSkinRemoveFlow(t *testing.T) {
	m, account := modelWithOfflineAccount(t)
	importTestSkin(t, m)
	path := m.cfg.SkinFile(*account)

	m.updateSkinModal(skinKey("d"))
	if m.modal != modalConfirm || m.confirm.Kind != confirmRemoveSkin {
		t.Fatalf("confirm = %v kind %v", m.modal, m.confirm.Kind)
	}
	if _, _ = m.updateConfirm(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")}); m.modal != modalSkin {
		t.Fatalf("modal after removal = %v, want modalSkin", m.modal)
	}
	if m.cfg.Accounts[0].HasCustomSkin() {
		t.Error("metadata was not cleared")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("skin file still exists: %v", err)
	}
}

func TestSkinRejectsMicrosoftAccount(t *testing.T) {
	m := newTestModel(t)
	account, err := m.cfg.AddMicrosoftAccount(auth.MinecraftSession{
		Username:    "MoonCC",
		UUID:        "b50ad385829d3141a2167e7d7539ba7f",
		AccessToken: "token",
	})
	if err != nil {
		t.Fatal(err)
	}
	m.screen = ScreenAccounts
	m.accountCursor = 0
	m.cfg.SetActiveAccount(account.UUID)
	m.updateAccounts(skinKey("s"))
	if m.modal != modalAlert {
		t.Fatalf("modal = %v, want modalAlert", m.modal)
	}
	if !strings.Contains(m.alert.Body, "Mojang") {
		t.Errorf("alert body = %q", m.alert.Body)
	}
}

func TestSkinToggleWithoutSkinShowsHint(t *testing.T) {
	m, _ := modelWithOfflineAccount(t)
	m.updateAccounts(skinKey("s"))
	m.updateSkinModal(skinKey("m"))
	if m.modal != modalSkin {
		t.Errorf("modal = %v, want modalSkin", m.modal)
	}
	if !strings.Contains(m.toast.Text, m.t("Import a skin before choosing its model.")) {
		t.Errorf("toast = %q", m.toast.Text)
	}
}
