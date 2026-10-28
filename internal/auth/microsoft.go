package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	// DefaultMicrosoftClientID is the public client used by the Minecraft
	// launcher's live.com device-code flow. It works without registering an
	// Azure application. Users who registered their own Entra application can
	// supply a GUID through Settings.MicrosoftClientID instead.
	DefaultMicrosoftClientID = "00000000402b5328"

	liveScope  = "service::user.auth.xboxlive.com::MBI_SSL"
	entraScope = "XboxLive.signin offline_access"
)

var (
	liveDeviceCodeEndpoint      = "https://login.live.com/oauth20_connect.srf"
	liveTokenEndpoint           = "https://login.live.com/oauth20_token.srf"
	entraDeviceCodeEndpoint     = "https://login.microsoftonline.com/consumers/oauth2/v2.0/devicecode"
	entraTokenEndpoint          = "https://login.microsoftonline.com/consumers/oauth2/v2.0/token"
	xboxUserAuthEndpoint        = "https://user.auth.xboxlive.com/user/authenticate"
	xstsEndpoint                = "https://xsts.auth.xboxlive.com/xsts/authorize"
	minecraftLoginEndpoint      = "https://api.minecraftservices.com/authentication/login_with_xbox"
	minecraftProfileEndpoint    = "https://api.minecraftservices.com/minecraft/profile"
	guidRE                      = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	ErrAuthorizationPending     = errors.New("the user has not finished signing in yet")
	ErrSlowDown                 = errors.New("Microsoft asked us to poll more slowly")
	ErrDeviceCodeExpired        = errors.New("the Microsoft sign-in code has expired")
	ErrNoMinecraftProfile       = errors.New("this Microsoft account does not own Minecraft: Java Edition")
	ErrMicrosoftAccountNoTokens = errors.New("the Microsoft account has no stored session")
)

// DeviceCode is the short code shown to the user while the launcher polls.
type DeviceCode struct {
	DeviceCode      string
	UserCode        string
	VerificationURI string
	ExpiresIn       int
	Interval        int
	Message         string
}

// TokenResponse is a Microsoft OAuth token response.
type TokenResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	ExpiresIn        int64  `json:"expires_in"`
	TokenType        string `json:"token_type"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// MinecraftSession is the result of the full Microsoft -> Xbox -> Minecraft
// chain. It can be stored directly in the launcher account list.
type MinecraftSession struct {
	Username             string
	UUID                 string
	AccessToken          string
	RefreshToken         string
	MicrosoftAccessToken string
	ExpiresAt            int64
	XUID                 string
	ClientID             string
}

type oauthFlow struct {
	deviceCodeURL string
	tokenURL      string
	scope         string
	ticketPrefix  string
}

type xboxTokenResponse struct {
	Token         string `json:"Token"`
	DisplayClaims struct {
		XUI []struct {
			UHS string `json:"uhs"`
			XID string `json:"xid"`
		} `json:"xui"`
	} `json:"DisplayClaims"`
}

type xstsErrorResponse struct {
	Identity string `json:"Identity"`
	XErr     int    `json:"XErr"`
	Message  string `json:"Message"`
	Redirect string `json:"Redirect"`
}

// NormalizeMicrosoftClientID trims a user supplied client id and falls back to
// the built-in public client when the value is empty.
func NormalizeMicrosoftClientID(clientID string) string {
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		return DefaultMicrosoftClientID
	}
	return clientID
}

func flowFor(clientID string) oauthFlow {
	clientID = NormalizeMicrosoftClientID(clientID)
	if guidRE.MatchString(clientID) {
		return oauthFlow{
			deviceCodeURL: entraDeviceCodeEndpoint,
			tokenURL:      entraTokenEndpoint,
			scope:         entraScope,
			ticketPrefix:  "d=",
		}
	}
	return oauthFlow{
		deviceCodeURL: liveDeviceCodeEndpoint,
		tokenURL:      liveTokenEndpoint,
		scope:         liveScope,
		ticketPrefix:  "t=",
	}
}

// BeginDeviceCode starts the device-code flow. The returned code expires after
// DeviceCode.ExpiresIn seconds and should be polled every Interval seconds.
func BeginDeviceCode(ctx context.Context, client *http.Client, clientID string) (*DeviceCode, error) {
	flow := flowFor(clientID)
	form := url.Values{
		"client_id": {NormalizeMicrosoftClientID(clientID)},
		"scope":     {flow.scope},
	}
	if flow.ticketPrefix == "t=" {
		form.Set("response_type", "device_code")
	}
	body, err := postForm(ctx, client, flow.deviceCodeURL, form)
	if err != nil {
		return nil, fmt.Errorf("request Microsoft sign-in code: %w", err)
	}
	var raw struct {
		DeviceCode       string `json:"device_code"`
		UserCode         string `json:"user_code"`
		VerificationURI  string `json:"verification_uri"`
		ExpiresIn        int    `json:"expires_in"`
		Interval         int    `json:"interval"`
		Message          string `json:"message"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode Microsoft sign-in code: %w", err)
	}
	if raw.Error != "" {
		return nil, fmt.Errorf("Microsoft sign-in: %s", describeOAuthError(raw.Error, raw.ErrorDescription))
	}
	if raw.DeviceCode == "" || raw.UserCode == "" {
		return nil, errors.New("Microsoft returned an incomplete sign-in code")
	}
	if raw.VerificationURI == "" {
		raw.VerificationURI = "https://www.microsoft.com/link"
	}
	if raw.Interval < 1 {
		raw.Interval = 5
	}
	if raw.ExpiresIn < 1 {
		raw.ExpiresIn = 900
	}
	return &DeviceCode{
		DeviceCode:      raw.DeviceCode,
		UserCode:        raw.UserCode,
		VerificationURI: raw.VerificationURI,
		ExpiresIn:       raw.ExpiresIn,
		Interval:        raw.Interval,
		Message:         raw.Message,
	}, nil
}

// PollDeviceCode checks once whether the user finished the browser step. It
// returns ErrAuthorizationPending while the user is still signing in.
func PollDeviceCode(ctx context.Context, client *http.Client, clientID string, code *DeviceCode) (*TokenResponse, error) {
	if code == nil || code.DeviceCode == "" {
		return nil, errors.New("missing Microsoft device code")
	}
	flow := flowFor(clientID)
	form := url.Values{
		"client_id":   {NormalizeMicrosoftClientID(clientID)},
		"device_code": {code.DeviceCode},
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
	}
	body, err := postForm(ctx, client, flow.tokenURL, form)
	var httpErr *HTTPError
	if err != nil {
		if !errors.As(err, &httpErr) {
			return nil, err
		}
		// Device-code polling errors such as authorization_pending and
		// slow_down are reported as HTTP 400 with a JSON OAuth body. Parse
		// that body instead of treating every non-2xx response as fatal.
		body = httpErr.Body
	}
	var token TokenResponse
	if err := json.Unmarshal(body, &token); err != nil {
		if httpErr != nil {
			return nil, httpErr
		}
		return nil, fmt.Errorf("decode Microsoft token: %w", err)
	}
	if token.Error != "" {
		switch token.Error {
		case "authorization_pending":
			return nil, ErrAuthorizationPending
		case "slow_down":
			return nil, ErrSlowDown
		case "expired_token", "code_expired":
			return nil, ErrDeviceCodeExpired
		case "authorization_declined", "access_denied":
			return nil, errors.New("Microsoft sign-in was declined")
		default:
			return nil, fmt.Errorf("Microsoft sign-in: %s", describeOAuthError(token.Error, token.ErrorDescription))
		}
	}
	if httpErr != nil {
		return nil, httpErr
	}
	if token.AccessToken == "" {
		return nil, errors.New("Microsoft returned an empty access token")
	}
	return &token, nil
}

// CompleteLogin exchanges a Microsoft token for an Xbox Live token, an XSTS
// token and finally a Minecraft access token plus profile.
func CompleteLogin(ctx context.Context, client *http.Client, clientID string, token *TokenResponse) (*MinecraftSession, error) {
	if token == nil || token.AccessToken == "" {
		return nil, errors.New("missing Microsoft access token")
	}
	clientID = NormalizeMicrosoftClientID(clientID)
	flow := flowFor(clientID)

	xbl, err := xboxAuthenticate(ctx, client, flow.ticketPrefix+token.AccessToken)
	if err != nil {
		return nil, err
	}
	if len(xbl.DisplayClaims.XUI) == 0 || xbl.DisplayClaims.XUI[0].UHS == "" {
		return nil, errors.New("Xbox Live returned no user hash")
	}
	uhs := xbl.DisplayClaims.XUI[0].UHS
	xuid := xbl.DisplayClaims.XUI[0].XID

	xsts, err := xstsAuthorize(ctx, client, xbl.Token)
	if err != nil {
		return nil, err
	}
	if len(xsts.DisplayClaims.XUI) > 0 {
		if got := xsts.DisplayClaims.XUI[0].UHS; got != "" && got != uhs {
			return nil, errors.New("Microsoft user hash changed during sign-in")
		}
		if xuid == "" {
			xuid = xsts.DisplayClaims.XUI[0].XID
		}
	}

	mcToken, expiresIn, err := loginWithXbox(ctx, client, uhs, xsts.Token)
	if err != nil {
		return nil, err
	}
	profile, err := fetchProfile(ctx, client, mcToken)
	if err != nil {
		return nil, err
	}
	if expiresIn <= 0 {
		expiresIn = 86400
	}
	return &MinecraftSession{
		Username:             profile.Name,
		UUID:                 NormalizeUUID(profile.ID),
		AccessToken:          mcToken,
		RefreshToken:         token.RefreshToken,
		MicrosoftAccessToken: token.AccessToken,
		ExpiresAt:            time.Now().Add(time.Duration(expiresIn) * time.Second).Unix(),
		XUID:                 xuid,
		ClientID:             clientID,
	}, nil
}

// RefreshMicrosoft obtains a fresh Microsoft token with the stored refresh
// token, then repeats the Xbox/Minecraft exchange.
func RefreshMicrosoft(ctx context.Context, client *http.Client, clientID, refreshToken string) (*MinecraftSession, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return nil, ErrMicrosoftAccountNoTokens
	}
	clientID = NormalizeMicrosoftClientID(clientID)
	flow := flowFor(clientID)
	form := url.Values{
		"client_id":     {clientID},
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"scope":         {flow.scope},
	}
	body, err := postForm(ctx, client, flow.tokenURL, form)
	if err != nil {
		return nil, fmt.Errorf("refresh Microsoft session: %w", err)
	}
	var token TokenResponse
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, fmt.Errorf("decode refreshed Microsoft token: %w", err)
	}
	if token.Error != "" {
		return nil, fmt.Errorf("refresh Microsoft session: %s", describeOAuthError(token.Error, token.ErrorDescription))
	}
	if token.AccessToken == "" {
		return nil, errors.New("Microsoft returned an empty access token while refreshing")
	}
	if token.RefreshToken == "" {
		token.RefreshToken = refreshToken
	}
	return CompleteLogin(ctx, client, clientID, &token)
}

// NeedsRefresh reports whether a Minecraft token is close enough to expiry
// that it should be renewed before launching.
func NeedsRefresh(expiresAt int64) bool {
	if expiresAt <= 0 {
		return true
	}
	return expiresAt <= time.Now().Add(5*time.Minute).Unix()
}

// NormalizeUUID adds hyphens to the 32-character Minecraft profile id.
func NormalizeUUID(id string) string {
	id = strings.TrimSpace(id)
	if guidRE.MatchString(id) {
		return strings.ToLower(id)
	}
	clean := strings.ReplaceAll(id, "-", "")
	if len(clean) != 32 {
		return id
	}
	return strings.ToLower(strings.Join([]string{
		clean[0:8], clean[8:12], clean[12:16], clean[16:20], clean[20:32],
	}, "-"))
}

func xboxAuthenticate(ctx context.Context, client *http.Client, ticket string) (*xboxTokenResponse, error) {
	payload := map[string]any{
		"Properties": map[string]any{
			"AuthMethod": "RPS",
			"SiteName":   "user.auth.xboxlive.com",
			"RpsTicket":  ticket,
		},
		"RelyingParty": "http://auth.xboxlive.com",
		"TokenType":    "JWT",
	}
	var out xboxTokenResponse
	if err := postJSON(ctx, client, xboxUserAuthEndpoint, payload, &out); err != nil {
		return nil, fmt.Errorf("Xbox Live sign-in: %w", err)
	}
	if out.Token == "" {
		return nil, errors.New("Xbox Live returned an empty token")
	}
	return &out, nil
}

func xstsAuthorize(ctx context.Context, client *http.Client, xblToken string) (*xboxTokenResponse, error) {
	payload := map[string]any{
		"Properties": map[string]any{
			"SandboxId":  "RETAIL",
			"UserTokens": []string{xblToken},
		},
		"RelyingParty": "rp://api.minecraftservices.com/",
		"TokenType":    "JWT",
	}
	var out xboxTokenResponse
	err := postJSON(ctx, client, xstsEndpoint, payload, &out)
	if err != nil {
		var httpErr *HTTPError
		if errors.As(err, &httpErr) {
			var detail xstsErrorResponse
			if json.Unmarshal(httpErr.Body, &detail) == nil && detail.XErr != 0 {
				return nil, fmt.Errorf("Xbox Live rejected the sign-in: %s (XErr %d)", xstsMessage(detail.XErr), detail.XErr)
			}
		}
		return nil, fmt.Errorf("Xbox Live authorization: %w", err)
	}
	if out.Token == "" {
		return nil, errors.New("Xbox Live returned an empty XSTS token")
	}
	return &out, nil
}

func loginWithXbox(ctx context.Context, client *http.Client, uhs, xstsToken string) (string, int64, error) {
	payload := map[string]string{
		"identityToken": "XBL3.0 x=" + uhs + ";" + xstsToken,
	}
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := postJSON(ctx, client, minecraftLoginEndpoint, payload, &out); err != nil {
		return "", 0, fmt.Errorf("Minecraft sign-in: %w", err)
	}
	if out.AccessToken == "" {
		return "", 0, errors.New("Minecraft returned an empty access token")
	}
	return out.AccessToken, out.ExpiresIn, nil
}

type minecraftProfile struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func fetchProfile(ctx context.Context, client *http.Client, accessToken string) (*minecraftProfile, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, minecraftProfileEndpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "mclauncher")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch Minecraft profile: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNoMinecraftProfile
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch Minecraft profile: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var profile minecraftProfile
	if err := json.Unmarshal(body, &profile); err != nil {
		return nil, fmt.Errorf("decode Minecraft profile: %w", err)
	}
	if profile.ID == "" || profile.Name == "" {
		return nil, ErrNoMinecraftProfile
	}
	return &profile, nil
}

// HTTPError carries a non-2xx response so callers can inspect API errors.
type HTTPError struct {
	Status int
	Body   []byte
}

func (e *HTTPError) Error() string {
	detail := strings.TrimSpace(string(e.Body))
	if len(detail) > 300 {
		detail = detail[:300] + "…"
	}
	if detail == "" {
		return fmt.Sprintf("HTTP %d", e.Status)
	}
	return fmt.Sprintf("HTTP %d: %s", e.Status, detail)
}

func postForm(ctx context.Context, client *http.Client, endpoint string, form url.Values) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "mclauncher")
	return doRequest(client, req)
}

func postJSON(ctx context.Context, client *http.Client, endpoint string, payload, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "mclauncher")
	data, err := doRequest(client, req)
	if err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(data, out)
}

func doRequest(client *http.Client, req *http.Request) ([]byte, error) {
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &HTTPError{Status: resp.StatusCode, Body: body}
	}
	return body, nil
}

func describeOAuthError(code, description string) string {
	description = strings.TrimSpace(description)
	if description == "" {
		return code
	}
	if len(description) > 300 {
		description = description[:300] + "…"
	}
	return description
}

func xstsMessage(code int) string {
	switch code {
	case 2148916227:
		return "this account is banned from Xbox Live"
	case 2148916233:
		return "this account has no Xbox profile; sign in at xbox.com once"
	case 2148916235:
		return "Xbox Live is not available in this account's country"
	case 2148916236, 2148916237:
		return "this account needs adult verification"
	case 2148916238:
		return "this is a child account and must be added to a Microsoft family"
	case 2148916262:
		return "Xbox Live requires additional account verification"
	default:
		return "the account could not be authorized"
	}
}
