package tui

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/MoonCC233/MCLauncher-TUI/internal/auth"
	"github.com/MoonCC233/MCLauncher-TUI/internal/util"
)

// MicrosoftLoginState tracks one device-code sign-in attempt. The zero value
// means no attempt is active.
type MicrosoftLoginState struct {
	Stage    string // starting, waiting, completing, failed
	Code     *auth.DeviceCode
	ClientID string
	Err      string
	Started  time.Time
	Interval time.Duration
	Attempt  int
}

type microsoftCodeMsg struct {
	code *auth.DeviceCode
	err  error
}

type microsoftPollMsg struct {
	token *auth.TokenResponse
	err   error
}

type microsoftSessionMsg struct {
	session *auth.MinecraftSession
	err     error
}

type microsoftTickMsg struct {
	attempt int
}

type microsoftRefreshedMsg struct {
	uuid    string
	session *auth.MinecraftSession
	err     error
}

func (m *Model) microsoftClientID() string {
	return auth.NormalizeMicrosoftClientID(m.cfg.Settings.MicrosoftClientID)
}

func (m *Model) openAccountTypePicker() {
	m.accountTypePick = 0
	m.modal = modalAccountAdd
}

func (m *Model) updateAccountType(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.modal = modalNone
		return m, nil
	case "up", "k":
		m.accountTypePick--
	case "down", "j":
		m.accountTypePick++
	case "enter":
		return m.chooseAccountType()
	}
	m.accountTypePick = clampInt(m.accountTypePick, 0, 1)
	return m, nil
}

func (m *Model) chooseAccountType() (tea.Model, tea.Cmd) {
	if m.accountTypePick == 0 {
		return m, m.openMicrosoftLogin()
	}
	m.openAccountForm()
	return m, nil
}

func (m *Model) accountTypePickerBody(contentW int) ([]string, []hitRegion) {
	options := []struct {
		label string
		right string
	}{
		{m.t("Sign in with Microsoft"), m.t("microsoft")},
		{m.t("Offline account"), m.t("offline")},
	}
	var body []string
	var hits []hitRegion
	for i, option := range options {
		selected := i == m.accountTypePick
		marker := "  "
		if selected {
			marker = stylePrimary.Render("▸ ")
		}
		body = append(body, javaPickerRow(marker, option.label, option.right, false, selected, contentW))
		hits = append(hits, hitRegion{Kind: hitAccountTypeRow, Index: i, Y: 3 + len(body) - 1, W: contentW, H: 1})
	}
	return body, hits
}

// openMicrosoftLogin starts the device-code flow. The browser step happens
// outside the launcher; the modal only shows the code and polls Microsoft.
func (m *Model) openMicrosoftLogin() tea.Cmd {
	clientID := m.microsoftClientID()
	m.ms = MicrosoftLoginState{
		Stage:    "starting",
		ClientID: clientID,
		Started:  time.Now(),
	}
	m.modal = modalMicrosoft
	m.addLog("launcher", "microsoft sign-in: requesting device code")
	return beginMicrosoftCmd(m.client, clientID)
}

func beginMicrosoftCmd(client *http.Client, clientID string) tea.Cmd {
	if client == nil {
		client = util.NewHTTPClient()
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		code, err := auth.BeginDeviceCode(ctx, client, clientID)
		return microsoftCodeMsg{code: code, err: err}
	}
}

func pollMicrosoftCmd(client *http.Client, clientID string, code *auth.DeviceCode) tea.Cmd {
	if client == nil {
		client = util.NewHTTPClient()
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		token, err := auth.PollDeviceCode(ctx, client, clientID, code)
		return microsoftPollMsg{token: token, err: err}
	}
}

func completeMicrosoftCmd(client *http.Client, clientID string, token *auth.TokenResponse) tea.Cmd {
	if client == nil {
		client = util.NewHTTPClient()
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		session, err := auth.CompleteLogin(ctx, client, clientID, token)
		return microsoftSessionMsg{session: session, err: err}
	}
}

func (m *Model) scheduleMicrosoftPoll() tea.Cmd {
	m.ms.Attempt++
	attempt := m.ms.Attempt
	delay := m.ms.Interval
	if delay <= 0 {
		delay = 5 * time.Second
	}
	return tea.Tick(delay, func(time.Time) tea.Msg { return microsoftTickMsg{attempt: attempt} })
}

func (m *Model) handleMicrosoftCode(msg microsoftCodeMsg) (tea.Model, tea.Cmd) {
	if m.modal != modalMicrosoft {
		return m, nil
	}
	if msg.err != nil {
		m.ms.Stage = "failed"
		m.ms.Err = msg.err.Error()
		m.addLog("error", "microsoft sign-in: "+msg.err.Error())
		return m, nil
	}
	m.ms.Code = msg.code
	m.ms.Stage = "waiting"
	m.ms.Interval = time.Duration(msg.code.Interval) * time.Second
	if m.ms.Interval < time.Second {
		m.ms.Interval = 5 * time.Second
	}
	m.addLog("launcher", fmt.Sprintf("microsoft sign-in: code %s", msg.code.UserCode))
	return m, m.scheduleMicrosoftPoll()
}

func (m *Model) handleMicrosoftPoll(msg microsoftPollMsg) (tea.Model, tea.Cmd) {
	if m.modal != modalMicrosoft || m.ms.Stage != "waiting" {
		return m, nil
	}
	switch {
	case msg.err == nil && msg.token != nil:
		m.ms.Stage = "completing"
		return m, completeMicrosoftCmd(m.client, m.ms.ClientID, msg.token)
	case msg.err == auth.ErrAuthorizationPending, msg.err == auth.ErrSlowDown:
		if msg.err == auth.ErrSlowDown {
			m.ms.Interval += 5 * time.Second
		}
		return m, m.scheduleMicrosoftPoll()
	case msg.err != nil:
		m.ms.Stage = "failed"
		m.ms.Err = msg.err.Error()
		m.addLog("error", "microsoft sign-in: "+msg.err.Error())
		return m, nil
	}
	return m, m.scheduleMicrosoftPoll()
}

func (m *Model) handleMicrosoftSession(msg microsoftSessionMsg) (tea.Model, tea.Cmd) {
	if m.modal != modalMicrosoft {
		return m, nil
	}
	if msg.err != nil {
		m.ms.Stage = "failed"
		m.ms.Err = msg.err.Error()
		m.addLog("error", "microsoft sign-in: "+msg.err.Error())
		return m, nil
	}
	account, err := m.cfg.AddMicrosoftAccount(*msg.session)
	if err != nil {
		m.ms.Stage = "failed"
		m.ms.Err = err.Error()
		m.addLog("error", "microsoft account: "+err.Error())
		return m, nil
	}
	m.modal = modalNone
	m.ms = MicrosoftLoginState{}
	m.clampAccountCursor()
	for i := range m.cfg.Accounts {
		if m.cfg.Accounts[i].UUID == account.UUID {
			m.accountCursor = i
			break
		}
	}
	m.ensureAccountVisible()
	m.setToast(m.tf("Signed in as %s", account.Username), toastSuccess)
	m.addLog("launcher", "microsoft account: "+account.Username)
	return m, nil
}

func (m *Model) handleMicrosoftRefreshed(msg microsoftRefreshedMsg) (tea.Model, tea.Cmd) {
	ok, err := m.cfg.UpdateMicrosoftAccount(msg.uuid, *msg.session)
	if err != nil {
		m.addLog("error", "microsoft refresh: "+err.Error())
		return m, nil
	}
	if ok {
		m.setToast(m.t("Session refreshed"), toastSuccess)
		m.addLog("launcher", "microsoft session refreshed")
	}
	return m, nil
}

func (m *Model) updateMicrosoft(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.modal = modalNone
		m.ms = MicrosoftLoginState{}
		return m, nil
	case "c":
		if m.ms.Code == nil {
			return m, nil
		}
		if err := clipboard.WriteAll(m.ms.Code.UserCode); err != nil {
			m.setToast(m.t("Could not copy the code"), toastError)
		} else {
			m.setToast(m.t("code copied"), toastSuccess)
		}
		return m, nil
	case "o":
		if m.ms.Code == nil || m.ms.Code.VerificationURI == "" {
			return m, nil
		}
		if err := util.OpenURL(m.ms.Code.VerificationURI); err != nil {
			m.setToast(err.Error(), toastError)
		}
		return m, nil
	case "r":
		if m.ms.Stage == "failed" {
			return m, m.openMicrosoftLogin()
		}
		return m, nil
	}
	return m, nil
}

func (m *Model) microsoftModalBody(contentW int) []string {
	var body []string
	switch m.ms.Stage {
	case "starting":
		body = append(body,
			stylePrimary.Render(spinnerFrame(m.spinner))+" "+styleMuted.Render(m.t("Starting Microsoft sign-in…")),
			"",
		)
	case "failed":
		body = append(body,
			styleRed.Render("✗ "+m.t("Microsoft sign-in failed")),
			"",
		)
		for _, line := range wrapPlain(m.ms.Err, contentW-2) {
			body = append(body, styleMuted.Render(line))
		}
		body = append(body, "", styleDim.Render(m.t("Press r to retry.")))
	case "waiting", "completing":
		code := m.ms.Code
		if code == nil {
			break
		}
		body = append(body,
			styleMuted.Render(m.t("Open this page and enter the code:")),
			"",
			styleBlue.Render(util.Truncate(code.VerificationURI, contentW-2)),
			"",
			lipgloss.NewStyle().
				Foreground(colPrimary).
				Background(colPanelAlt).
				Bold(true).
				Padding(0, 2).
				Render(code.UserCode),
			"",
		)
		if m.ms.Stage == "completing" {
			body = append(body, stylePrimary.Render(spinnerFrame(m.spinner))+" "+styleMuted.Render(m.t("Completing Microsoft sign-in…")))
		} else {
			body = append(body, stylePrimary.Render(spinnerFrame(m.spinner))+" "+styleMuted.Render(m.t("Waiting for you to finish signing in…")))
		}
		if code.ExpiresIn > 0 {
			remaining := time.Until(m.ms.Started.Add(time.Duration(code.ExpiresIn) * time.Second))
			if remaining <= 0 {
				body = append(body, styleRed.Render(m.t("The code expired — press r to try again.")))
			} else {
				body = append(body, styleDim.Render(m.tf("The code expires in %s", util.HumanDuration(remaining))))
			}
		}
	}
	return body
}

func (m *Model) microsoftModalHints() []hintSpec {
	hints := []hintSpec{
		hint("c", m.t("Copy code")),
		hint("o", m.t("Open link")),
	}
	if m.ms.Stage == "failed" {
		hints = append(hints, hint("r", m.t("Retry")))
	}
	return hints
}
