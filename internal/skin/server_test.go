package skin

import (
	"bytes"
	"crypto"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestServerServesProfileAndTexture(t *testing.T) {
	srv, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	uuid := "b50ad385-829d-3141-a216-7e7d7539ba7f"
	sk := Default(ModelClassic)
	if err := srv.AddProfile(Profile{UUID: uuid, Name: "MoonCC", Model: ModelSlim, Skin: sk}); err != nil {
		t.Fatal(err)
	}
	hash, err := sk.Hash()
	if err != nil {
		t.Fatal(err)
	}

	root := getJSON(t, srv.URL()+"/")
	pubPEM, _ := root["signaturePublickey"].(string)
	if pubPEM == "" {
		t.Fatal("root has no signaturePublickey")
	}
	domains, _ := root["skinDomains"].([]any)
	if len(domains) == 0 {
		t.Fatal("root has no skinDomains")
	}

	profile := getJSON(t, srv.URL()+"/sessionserver/session/minecraft/profile/"+strings.ReplaceAll(uuid, "-", ""))
	if profile["name"] != "MoonCC" {
		t.Errorf("name = %v", profile["name"])
	}
	props, _ := profile["properties"].([]any)
	if len(props) != 1 {
		t.Fatalf("properties = %v", props)
	}
	prop, _ := props[0].(map[string]any)
	value, _ := prop["value"].(string)
	signature, _ := prop["signature"].(string)
	if value == "" || signature == "" {
		t.Fatal("textures property is not signed")
	}
	verifySignature(t, pubPEM, value, signature)

	raw, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Textures map[string]struct {
			URL      string `json:"url"`
			Metadata struct {
				Model string `json:"model"`
			} `json:"metadata"`
		} `json:"textures"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	skinEntry, ok := payload.Textures["SKIN"]
	if !ok {
		t.Fatalf("textures = %s", raw)
	}
	if want := srv.URL() + "/textures/" + hash; skinEntry.URL != want {
		t.Errorf("skin URL = %q, want %q", skinEntry.URL, want)
	}
	if skinEntry.Metadata.Model != ModelSlim {
		t.Errorf("model = %q, want slim", skinEntry.Metadata.Model)
	}

	resp, err := http.Get(skinEntry.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("texture status = %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "image/png" {
		t.Errorf("content type = %q", ct)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decode(bytes.NewReader(data)); err != nil {
		t.Errorf("served texture is not a valid skin: %v", err)
	}

	hasJoined := getJSON(t, srv.URL()+"/sessionserver/session/minecraft/hasJoined?username=MoonCC&serverId=abc")
	if hasJoined["id"] != strings.ReplaceAll(uuid, "-", "") {
		t.Errorf("hasJoined id = %v", hasJoined["id"])
	}
}

func TestServerUnknownProfileIsNoContent(t *testing.T) {
	srv, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	resp, err := http.Get(srv.URL() + "/sessionserver/session/minecraft/profile/00000000000000000000000000000000")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want 204", resp.StatusCode)
	}
}

func TestServerRejectsInvalidProfile(t *testing.T) {
	srv, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	if err := srv.AddProfile(Profile{UUID: "not-a-uuid", Name: "MoonCC"}); err == nil {
		t.Error("invalid UUID was accepted")
	}
	if err := srv.AddProfile(Profile{UUID: "b50ad385-829d-3141-a216-7e7d7539ba7f"}); err == nil {
		t.Error("empty name was accepted")
	}
}

func getJSON(t *testing.T, url string) map[string]any {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: %d", url, resp.StatusCode)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out
}

func verifySignature(t *testing.T, pubPEM, value, signature string) {
	t.Helper()
	block, _ := pem.Decode([]byte(pubPEM))
	if block == nil {
		t.Fatal("cannot parse public key PEM")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	pub, ok := parsed.(*rsa.PublicKey)
	if !ok {
		t.Fatalf("public key is %T", parsed)
	}
	sig, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha1.Sum([]byte(value))
	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA1, sum[:], sig); err != nil {
		t.Errorf("signature does not verify: %v", err)
	}
}
