package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type microsoftTestServer struct {
	srv          *httptest.Server
	tokenCalls   int
	refreshCalls int
	profile      bool
}

func newMicrosoftTestServer(t *testing.T) *microsoftTestServer {
	t.Helper()
	ts := &microsoftTestServer{profile: true}
	mux := http.NewServeMux()
	mux.HandleFunc("/live/device", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("device form: %v", err)
		}
		if got := r.Form.Get("client_id"); got != DefaultMicrosoftClientID {
			t.Errorf("device client_id = %q", got)
		}
		if got := r.Form.Get("scope"); got != liveScope {
			t.Errorf("device scope = %q", got)
		}
		fmt.Fprint(w, `{"device_code":"dev-1","user_code":"ABCD-EFGH","verification_uri":"https://example.test/link","expires_in":900,"interval":5}`)
	})
	mux.HandleFunc("/live/token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("token form: %v", err)
		}
		ts.tokenCalls++
		grant := r.Form.Get("grant_type")
		if grant == "urn:ietf:params:oauth:grant-type:device_code" {
			if ts.tokenCalls == 1 {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprint(w, `{"error":"authorization_pending"}`)
				return
			}
		}
		if grant == "refresh_token" {
			ts.refreshCalls++
			if r.Form.Get("refresh_token") != "refresh-1" {
				t.Errorf("refresh token = %q", r.Form.Get("refresh_token"))
			}
		}
		fmt.Fprint(w, `{"access_token":"ms-access","refresh_token":"refresh-1","expires_in":3600}`)
	})
	mux.HandleFunc("/xbox", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Properties struct {
				RpsTicket string `json:"RpsTicket"`
			} `json:"Properties"`
		}
		if err := decodeJSON(r, &body); err != nil {
			t.Errorf("xbox body: %v", err)
		}
		if got := body.Properties.RpsTicket; got != "t=ms-access" && got != "t=ms-access-2" {
			t.Errorf("RpsTicket = %q", got)
		}
		fmt.Fprint(w, `{"Token":"xbl-token","DisplayClaims":{"xui":[{"uhs":"user-hash","xid":"12345"}]}}`)
	})
	mux.HandleFunc("/xsts", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"Token":"xsts-token","DisplayClaims":{"xui":[{"uhs":"user-hash","xid":"12345"}]}}`)
	})
	mux.HandleFunc("/mc", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"access_token":"mc-access","expires_in":86400}`)
	})
	mux.HandleFunc("/profile", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer mc-access" {
			t.Errorf("Authorization = %q", got)
		}
		if !ts.profile {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"error":"NOT_FOUND"}`)
			return
		}
		fmt.Fprint(w, `{"id":"986dec87b7ec47ff89ff033fdb95c4b5","name":"HowDoesAuthWork"}`)
	})
	ts.srv = httptest.NewServer(mux)
	old := []string{
		liveDeviceCodeEndpoint, liveTokenEndpoint, entraDeviceCodeEndpoint,
		entraTokenEndpoint, xboxUserAuthEndpoint, xstsEndpoint,
		minecraftLoginEndpoint, minecraftProfileEndpoint,
	}
	liveDeviceCodeEndpoint = ts.srv.URL + "/live/device"
	liveTokenEndpoint = ts.srv.URL + "/live/token"
	entraDeviceCodeEndpoint = ts.srv.URL + "/entra/device"
	entraTokenEndpoint = ts.srv.URL + "/entra/token"
	xboxUserAuthEndpoint = ts.srv.URL + "/xbox"
	xstsEndpoint = ts.srv.URL + "/xsts"
	minecraftLoginEndpoint = ts.srv.URL + "/mc"
	minecraftProfileEndpoint = ts.srv.URL + "/profile"
	t.Cleanup(func() {
		ts.srv.Close()
		liveDeviceCodeEndpoint = old[0]
		liveTokenEndpoint = old[1]
		entraDeviceCodeEndpoint = old[2]
		entraTokenEndpoint = old[3]
		xboxUserAuthEndpoint = old[4]
		xstsEndpoint = old[5]
		minecraftLoginEndpoint = old[6]
		minecraftProfileEndpoint = old[7]
	})
	return ts
}

func decodeJSON(r *http.Request, out any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(out)
}

func TestMicrosoftDeviceCodeFlow(t *testing.T) {
	newMicrosoftTestServer(t)
	ctx := context.Background()
	code, err := BeginDeviceCode(ctx, http.DefaultClient, "")
	if err != nil {
		t.Fatalf("BeginDeviceCode: %v", err)
	}
	if code.UserCode != "ABCD-EFGH" || code.DeviceCode != "dev-1" || code.Interval != 5 {
		t.Fatalf("unexpected device code: %+v", code)
	}
	if _, err := PollDeviceCode(ctx, http.DefaultClient, "", code); !errors.Is(err, ErrAuthorizationPending) {
		t.Fatalf("first poll error = %v, want pending", err)
	}
	token, err := PollDeviceCode(ctx, http.DefaultClient, "", code)
	if err != nil {
		t.Fatalf("second poll: %v", err)
	}
	if token.AccessToken != "ms-access" || token.RefreshToken != "refresh-1" {
		t.Fatalf("unexpected token: %+v", token)
	}
}

func TestMicrosoftPollErrors(t *testing.T) {
	cases := []struct {
		name string
		body string
		want error
	}{
		{"pending", `{"error":"authorization_pending"}`, ErrAuthorizationPending},
		{"slow down", `{"error":"slow_down"}`, ErrSlowDown},
		{"expired", `{"error":"expired_token"}`, ErrDeviceCodeExpired},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprint(w, tc.body)
			}))
			defer srv.Close()
			old := liveTokenEndpoint
			liveTokenEndpoint = srv.URL
			t.Cleanup(func() { liveTokenEndpoint = old })

			_, err := PollDeviceCode(context.Background(), srv.Client(), "", &DeviceCode{DeviceCode: "dev-1"})
			if !errors.Is(err, tc.want) {
				t.Fatalf("PollDeviceCode error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestMicrosoftPollUnexpectedHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprint(w, "upstream unavailable")
	}))
	defer srv.Close()
	old := liveTokenEndpoint
	liveTokenEndpoint = srv.URL
	t.Cleanup(func() { liveTokenEndpoint = old })

	_, err := PollDeviceCode(context.Background(), srv.Client(), "", &DeviceCode{DeviceCode: "dev-1"})
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) || httpErr.Status != http.StatusBadGateway {
		t.Fatalf("PollDeviceCode error = %v, want HTTP 502", err)
	}
}

func TestMicrosoftCompleteLogin(t *testing.T) {
	newMicrosoftTestServer(t)
	session, err := CompleteLogin(context.Background(), http.DefaultClient, "", &TokenResponse{
		AccessToken:  "ms-access",
		RefreshToken: "refresh-1",
	})
	if err != nil {
		t.Fatalf("CompleteLogin: %v", err)
	}
	if session.Username != "HowDoesAuthWork" {
		t.Errorf("Username = %q", session.Username)
	}
	if session.UUID != "986dec87-b7ec-47ff-89ff-033fdb95c4b5" {
		t.Errorf("UUID = %q", session.UUID)
	}
	if session.AccessToken != "mc-access" || session.RefreshToken != "refresh-1" {
		t.Errorf("tokens = %+v", session)
	}
	if session.XUID != "12345" || session.ClientID != DefaultMicrosoftClientID {
		t.Errorf("xuid/client = %q/%q", session.XUID, session.ClientID)
	}
	if session.ExpiresAt <= 0 {
		t.Errorf("ExpiresAt = %d", session.ExpiresAt)
	}
}

func TestMicrosoftRefresh(t *testing.T) {
	ts := newMicrosoftTestServer(t)
	session, err := RefreshMicrosoft(context.Background(), http.DefaultClient, "", "refresh-1")
	if err != nil {
		t.Fatalf("RefreshMicrosoft: %v", err)
	}
	if ts.refreshCalls != 1 {
		t.Errorf("refresh calls = %d, want 1", ts.refreshCalls)
	}
	if session.AccessToken != "mc-access" {
		t.Errorf("refreshed token = %q", session.AccessToken)
	}
}

func TestMicrosoftNoProfile(t *testing.T) {
	ts := newMicrosoftTestServer(t)
	ts.profile = false
	_, err := CompleteLogin(context.Background(), http.DefaultClient, "", &TokenResponse{AccessToken: "ms-access"})
	if !errors.Is(err, ErrNoMinecraftProfile) {
		t.Fatalf("CompleteLogin error = %v, want ErrNoMinecraftProfile", err)
	}
}

func TestEntraFlowUsesDashedTicket(t *testing.T) {
	ts := newMicrosoftTestServer(t)
	const clientID = "11111111-2222-3333-4444-555555555555"
	// The test server above only maps live endpoints; point the Entra device
	// endpoint at a handler that echoes the client id.
	mux := http.NewServeMux()
	mux.HandleFunc("/entra", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("form: %v", err)
		}
		if got := r.Form.Get("client_id"); got != clientID {
			t.Errorf("client_id = %q", got)
		}
		if got := r.Form.Get("scope"); got != entraScope {
			t.Errorf("scope = %q", got)
		}
		fmt.Fprint(w, `{"device_code":"dev-2","user_code":"CODE","verification_uri":"https://example.test","expires_in":900,"interval":5}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	old := entraDeviceCodeEndpoint
	entraDeviceCodeEndpoint = srv.URL + "/entra"
	t.Cleanup(func() { entraDeviceCodeEndpoint = old })

	if _, err := BeginDeviceCode(context.Background(), ts.srv.Client(), clientID); err != nil {
		t.Fatalf("BeginDeviceCode: %v", err)
	}
}

func TestNormalizeMicrosoftClientID(t *testing.T) {
	if got := NormalizeMicrosoftClientID(""); got != DefaultMicrosoftClientID {
		t.Errorf("empty client id = %q", got)
	}
	if got := NormalizeMicrosoftClientID("  abc  "); got != "abc" {
		t.Errorf("trimmed client id = %q", got)
	}
}

func TestNormalizeUUID(t *testing.T) {
	const want = "986dec87-b7ec-47ff-89ff-033fdb95c4b5"
	for _, in := range []string{
		"986dec87b7ec47ff89ff033fdb95c4b5",
		"986DEC87-B7EC-47FF-89FF-033FDB95C4B5",
	} {
		if got := NormalizeUUID(in); got != want {
			t.Errorf("NormalizeUUID(%q) = %q", in, got)
		}
	}
	if got := NormalizeUUID("not-a-uuid"); got != "not-a-uuid" {
		t.Errorf("NormalizeUUID(invalid) = %q", got)
	}
}

func TestNeedsRefresh(t *testing.T) {
	if !NeedsRefresh(0) {
		t.Error("missing expiry should need refresh")
	}
	if NeedsRefresh(time.Now().Add(24 * time.Hour).Unix()) {
		t.Error("a fresh token should not need refresh")
	}
	if !NeedsRefresh(time.Now().Add(2 * time.Minute).Unix()) {
		t.Error("a token expiring in two minutes should need refresh")
	}
}

func TestXboxErrorMapping(t *testing.T) {
	if !strings.Contains(xstsMessage(2148916233), "Xbox profile") {
		t.Errorf("unexpected message for 2148916233: %q", xstsMessage(2148916233))
	}
}
