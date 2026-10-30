package tui

import (
	"errors"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/MoonCC233/MCLauncher-TUI/internal/config"
	"github.com/MoonCC233/MCLauncher-TUI/internal/skin"
	"github.com/MoonCC233/MCLauncher-TUI/internal/util"
)

// skinModalState caches the preview loaded from disk. A nil preview means the
// account has no custom skin (the modal then draws the generated default).
type skinModalState struct {
	preview *skin.Skin
	err     string
}

func (m *Model) openSkinModal() {
	acc := m.currentAccount()
	if acc == nil {
		m.alert = AlertState{Title: m.t("No account selected"), Body: m.t("Add an offline account from the Accounts screen first.")}
		m.modal = modalAlert
		return
	}
	if acc.IsMicrosoft() {
		m.alert = AlertState{
			Title: m.t("Microsoft account"),
			Body:  m.t("Microsoft accounts use their Mojang skin. Switch to an offline account to use a custom skin."),
		}
		m.modal = modalAlert
		return
	}
	m.skinModal = skinModalState{}
	m.reloadSkinPreview()
	m.modal = modalSkin
}

func (m *Model) updateSkinModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.modal = modalNone
	case "i":
		m.openPrompt(promptSkinPath, m.t("Import a skin"), "",
			m.t("Path to a 64×64 or 64×32 PNG file. It is copied next to your launcher config."), 260)
	case "m":
		m.toggleSkinModel()
	case "o":
		m.openSkinFile()
	case "r":
		m.reloadSkinFromDisk()
	case "d":
		m.startSkinRemove()
	}
	return m, nil
}

func (m *Model) reloadSkinPreview() {
	acc := m.currentAccount()
	if acc == nil || !acc.HasCustomSkin() {
		m.skinModal.preview = nil
		m.skinModal.err = ""
		return
	}
	path := m.cfg.SkinFile(*acc)
	loaded, err := skin.Load(path)
	if err != nil {
		m.skinModal.preview = nil
		if os.IsNotExist(err) {
			m.skinModal.err = m.t("The skin file is missing. Import a PNG first.")
		} else {
			m.skinModal.err = m.tf("Could not read skin: %s", err.Error())
		}
		return
	}
	m.skinModal.preview = loaded
	m.skinModal.err = ""
}

// applySkinPath imports a PNG, stores the canonical copy next to the config
// and records its hash so the launch path can trust the file later.
func (m *Model) applySkinPath(value string) error {
	acc := m.currentAccount()
	if acc == nil {
		return errors.New(m.t("No account selected"))
	}
	if acc.IsMicrosoft() {
		return errors.New(m.t("Custom skins are only available for offline accounts."))
	}
	src, err := resolveSkinPath(value)
	if err != nil {
		return fmt.Errorf("%s", m.tf("Could not read skin: %s", err.Error()))
	}
	loaded, err := skin.Load(src)
	if err != nil {
		return fmt.Errorf("%s", m.tf("Could not read skin: %s", err.Error()))
	}
	dest := m.cfg.SkinFile(*acc)
	if dest == "" {
		return errors.New("config path is unavailable")
	}
	if err := loaded.Save(dest); err != nil {
		return fmt.Errorf("%s", m.tf("Could not save skin: %s", err.Error()))
	}
	hash, err := loaded.Hash()
	if err != nil {
		return fmt.Errorf("%s", m.tf("Could not save skin: %s", err.Error()))
	}
	meta := config.AccountSkin{
		Model:     acc.SkinModel(),
		SHA256:    hash,
		UpdatedAt: time.Now().Unix(),
	}
	if err := m.cfg.SetAccountSkin(acc.UUID, meta); err != nil {
		return err
	}
	m.skinModal = skinModalState{preview: loaded}
	m.modal = modalSkin
	m.setToast(m.t("Skin imported"), toastSuccess)
	m.addLog("launcher", "imported skin for "+acc.Username)
	return nil
}

// reloadSkinFromDisk refreshes metadata after the player edits the PNG in an
// external image editor.
func (m *Model) reloadSkinFromDisk() {
	acc := m.currentAccount()
	if acc == nil {
		return
	}
	if !acc.HasCustomSkin() {
		m.setToast(m.t("The skin file is missing. Import a PNG first."), toastInfo)
		return
	}
	path := m.cfg.SkinFile(*acc)
	loaded, err := skin.Load(path)
	if err != nil {
		m.setToast(m.tf("Could not read skin: %s", err.Error()), toastError)
		return
	}
	hash, err := loaded.Hash()
	if err != nil {
		m.setToast(m.tf("Could not read skin: %s", err.Error()), toastError)
		return
	}
	meta := config.AccountSkin{
		Model:     acc.SkinModel(),
		SHA256:    hash,
		UpdatedAt: time.Now().Unix(),
	}
	if err := m.cfg.SetAccountSkin(acc.UUID, meta); err != nil {
		m.setToast(m.tf("Could not save skin: %s", err.Error()), toastError)
		return
	}
	m.skinModal.preview = loaded
	m.skinModal.err = ""
	m.setToast(m.t("Skin reloaded"), toastSuccess)
}

func (m *Model) toggleSkinModel() {
	acc := m.currentAccount()
	if acc == nil {
		return
	}
	if !acc.HasCustomSkin() {
		m.setToast(m.t("Import a skin before choosing its model."), toastInfo)
		return
	}
	next := skin.ModelClassic
	if acc.SkinModel() == skin.ModelClassic {
		next = skin.ModelSlim
	}
	meta := *acc.Skin
	meta.Model = next
	if err := m.cfg.SetAccountSkin(acc.UUID, meta); err != nil {
		m.alert = AlertState{Title: m.t("Offline skin"), Body: err.Error()}
		m.modal = modalAlert
		return
	}
	m.setToast(m.tf("Skin model: %s", m.t(next)), toastSuccess)
}

func (m *Model) openSkinFile() {
	acc := m.currentAccount()
	if acc == nil {
		return
	}
	path := m.cfg.SkinFile(*acc)
	if !acc.HasCustomSkin() || path == "" {
		m.setToast(m.t("The skin file is missing. Import a PNG first."), toastInfo)
		return
	}
	if _, err := os.Stat(path); err != nil {
		m.setToast(m.t("The skin file is missing. Import a PNG first."), toastInfo)
		return
	}
	if err := util.OpenPath(path); err != nil {
		m.setToast(m.tf("Could not read skin: %s", err.Error()), toastError)
	}
}

func (m *Model) startSkinRemove() {
	acc := m.currentAccount()
	if acc == nil {
		return
	}
	if !acc.HasCustomSkin() {
		m.setToast(m.t("No custom skin yet"), toastInfo)
		return
	}
	m.confirm = ConfirmState{
		Kind:   confirmRemoveSkin,
		Title:  m.t("Remove this skin?"),
		Body:   m.tf("The custom skin for “%s” will be deleted. The account itself stays.", acc.Username),
		Target: acc.UUID,
	}
	m.modal = modalConfirm
}

func (m *Model) removeSkin(uuid string) error {
	acc := m.cfg.FindAccount(uuid)
	if acc == nil {
		return fmt.Errorf("account not found")
	}
	path := m.cfg.SkinFile(*acc)
	if err := m.cfg.ClearAccountSkin(uuid); err != nil {
		return err
	}
	if path != "" {
		_ = os.Remove(path)
	}
	m.skinModal = skinModalState{}
	m.modal = modalSkin
	m.setToast(m.t("Skin removed"), toastSuccess)
	return nil
}

func resolveSkinPath(value string) (string, error) {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"'`)
	if value == "" {
		return "", fmt.Errorf("path is empty")
	}
	if value == "~" || strings.HasPrefix(value, "~/") || strings.HasPrefix(value, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if value == "~" {
			value = home
		} else {
			value = filepath.Join(home, value[2:])
		}
	}
	return filepath.Abs(value)
}

func (m *Model) skinModalBody(contentW int) []string {
	acc := m.currentAccount()
	if acc == nil {
		return []string{styleMuted.Render(m.t("No account selected"))}
	}
	preview := m.skinModal.preview
	if preview == nil {
		preview = skin.Default(acc.SkinModel())
	}
	sprite := renderSkinSprite(preview, panelColor())
	modelName := m.t(acc.SkinModel())
	infoW := max(12, contentW-skin.SpriteWidth-3)

	var info []string
	info = append(info, styleBold.Render(acc.Username))
	info = append(info, styleDim.Render("UUID  "+acc.UUID))
	info = append(info, "")
	info = append(info, styleMuted.Render(m.t("model"))+"  "+styleCyan.Render(modelName))
	info = append(info, styleDim.Render(fmt.Sprintf("%d×%d PNG", preview.Width(), preview.Height())))
	info = append(info, "")
	if acc.HasCustomSkin() {
		updated := util.FormatUnix(acc.Skin.UpdatedAt)
		if acc.Skin.UpdatedAt <= 0 {
			updated = m.t("never")
		}
		info = append(info, styleGreen.Render("● ")+
			styleText.Render(m.tf("custom skin · %s · updated %s", modelName, updated)))
	} else {
		info = append(info, styleMuted.Render(m.t("No custom skin yet")))
	}
	if path := m.cfg.SkinFile(*acc); path != "" {
		info = append(info, styleDim.Render(util.ShortenPath(path, infoW)))
	}
	if m.skinModal.err != "" {
		info = append(info, "", styleRed.Render("✗ "+m.skinModal.err))
	}
	info = append(info, "")
	for _, line := range wrapPlain(m.t("Import a PNG, or edit the saved file in any image editor and press r to reload."), infoW) {
		info = append(info, styleDim.Render(line))
	}

	if contentW < skin.SpriteWidth+3+24 {
		out := append([]string{}, sprite...)
		out = append(out, "")
		return append(out, info...)
	}
	height := max(len(sprite), len(info))
	out := make([]string, 0, height)
	for i := 0; i < height; i++ {
		left := strings.Repeat(" ", skin.SpriteWidth)
		if i < len(sprite) {
			left = sprite[i]
		}
		right := ""
		if i < len(info) {
			right = info[i]
		}
		out = append(out, left+"   "+fitLine(right, infoW))
	}
	return out
}

// renderSkinSprite turns the 16×32 silhouette into 16 half-block rows.
func renderSkinSprite(s *skin.Skin, bg color.NRGBA) []string {
	rows := skin.Sprite(s, bg)
	out := make([]string, 0, (len(rows)+1)/2)
	for y := 0; y < len(rows); y += 2 {
		var b strings.Builder
		for x := 0; x < len(rows[y]); x++ {
			top := rows[y][x]
			bottom := top
			if y+1 < len(rows) {
				bottom = rows[y+1][x]
			}
			b.WriteString(lipgloss.NewStyle().
				Foreground(lipgloss.Color(rgbHex(top))).
				Background(lipgloss.Color(rgbHex(bottom))).
				Render("▀"))
		}
		out = append(out, b.String())
	}
	return out
}

func panelColor() color.NRGBA {
	if c, ok := parseHexRGB(string(colPanelAlt)); ok {
		return c
	}
	return color.NRGBA{R: 0x1c, G: 0x1c, B: 0x21, A: 0xff}
}

func parseHexRGB(value string) (color.NRGBA, bool) {
	value = strings.TrimPrefix(strings.TrimSpace(value), "#")
	if len(value) != 6 {
		return color.NRGBA{}, false
	}
	r, errR := strconv.ParseUint(value[0:2], 16, 8)
	g, errG := strconv.ParseUint(value[2:4], 16, 8)
	b, errB := strconv.ParseUint(value[4:6], 16, 8)
	if errR != nil || errG != nil || errB != nil {
		return color.NRGBA{}, false
	}
	return color.NRGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 0xff}, true
}

func rgbHex(c color.NRGBA) string {
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}
