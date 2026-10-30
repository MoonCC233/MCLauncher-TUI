package skin

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Profile is one player exposed by the local Yggdrasil API.
type Profile struct {
	UUID  string
	Name  string
	Model string
	Skin  *Skin
}

type servedProfile struct {
	uuid  string // compact, lowercase
	name  string
	model string
	hash  string
}

// Server is a loopback-only Yggdrasil API. The game is pointed at it through
// authlib-injector, which asks it for the player's profile and skin texture.
type Server struct {
	ln      net.Listener
	http    *http.Server
	baseURL string

	key    *rsa.PrivateKey
	pubPEM string

	mu       sync.RWMutex
	profiles map[string]servedProfile
	byName   map[string]string
	textures map[string][]byte
}

// NewServer binds a random loopback port and starts serving in the background.
func NewServer() (*Server, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generate skin signature key: %w", err)
	}
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return nil, err
	}
	pubPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("start local skin server: %w", err)
	}
	s := &Server{
		ln:       ln,
		baseURL:  "http://" + ln.Addr().String(),
		key:      key,
		pubPEM:   pubPEM,
		profiles: map[string]servedProfile{},
		byName:   map[string]string{},
		textures: map[string][]byte{},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleRoot)
	mux.HandleFunc("GET /status", s.handleStatus)
	mux.HandleFunc("POST /api/profiles/minecraft", s.handleProfiles)
	mux.HandleFunc("GET /sessionserver/session/minecraft/hasJoined", s.handleHasJoined)
	mux.HandleFunc("POST /sessionserver/session/minecraft/join", s.handleJoin)
	mux.HandleFunc("GET /sessionserver/session/minecraft/profile/{uuid}", s.handleProfile)
	mux.HandleFunc("GET /textures/{hash}", s.handleTexture)
	s.http = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ErrorLog:          log.New(io.Discard, "", 0),
	}
	go func() { _ = s.http.Serve(ln) }()
	return s, nil
}

// URL is the API root the game and authlib-injector should use.
func (s *Server) URL() string { return s.baseURL }

// AddProfile registers a player. A nil skin still exposes the profile, just
// without a texture, which is how a plain offline account looks.
func (s *Server) AddProfile(p Profile) error {
	uuid := compactUUID(p.UUID)
	if uuid == "" {
		return fmt.Errorf("invalid profile UUID %q", p.UUID)
	}
	name := strings.TrimSpace(p.Name)
	if name == "" {
		return fmt.Errorf("profile name cannot be empty")
	}
	hash := ""
	if p.Skin != nil {
		data, err := p.Skin.Bytes()
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		hash = hex.EncodeToString(sum[:])
		s.mu.Lock()
		s.textures[hash] = data
		s.mu.Unlock()
	}
	s.mu.Lock()
	s.profiles[uuid] = servedProfile{uuid: uuid, name: name, model: NormalizeModel(p.Model), hash: hash}
	s.byName[strings.ToLower(name)] = uuid
	s.mu.Unlock()
	return nil
}

// Close stops the HTTP server. It is safe to call more than once.
func (s *Server) Close() error {
	if s == nil || s.http == nil {
		return nil
	}
	return s.http.Close()
}

func (s *Server) handleRoot(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"signaturePublickey": s.pubPEM,
		"skinDomains":        []string{"127.0.0.1", "localhost"},
		"meta": map[string]any{
			"serverName":            "mclauncher",
			"implementationName":    "mclauncher",
			"implementationVersion": "1.0",
		},
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	count := len(s.profiles)
	s.mu.RUnlock()
	writeJSON(w, http.StatusOK, map[string]any{
		"user.count":                  count,
		"token.count":                 0,
		"pendingAuthentication.count": 0,
	})
}

func (s *Server) handleProfiles(w http.ResponseWriter, r *http.Request) {
	var names []string
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024)).Decode(&names); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	out := make([]map[string]string, 0, len(names))
	seen := map[string]bool{}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, name := range names {
		uuid, ok := s.byName[strings.ToLower(strings.TrimSpace(name))]
		if !ok || seen[uuid] {
			continue
		}
		seen[uuid] = true
		p := s.profiles[uuid]
		out = append(out, map[string]string{"id": p.uuid, "name": p.name})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleHasJoined(w http.ResponseWriter, r *http.Request) {
	uuid, ok := s.lookupName(r.URL.Query().Get("username"))
	if !ok {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	s.writeProfile(w, uuid)
}

func (s *Server) handleJoin(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request) {
	uuid := compactUUID(r.PathValue("uuid"))
	s.mu.RLock()
	_, ok := s.profiles[uuid]
	s.mu.RUnlock()
	if !ok {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	s.writeProfile(w, uuid)
}

func (s *Server) writeProfile(w http.ResponseWriter, uuid string) {
	s.mu.RLock()
	p, ok := s.profiles[uuid]
	s.mu.RUnlock()
	if !ok {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	textures := map[string]any{}
	if p.hash != "" {
		skin := map[string]any{"url": s.baseURL + "/textures/" + p.hash}
		if p.model == ModelSlim {
			skin["metadata"] = map[string]string{"model": ModelSlim}
		}
		textures["SKIN"] = skin
	}
	payload := map[string]any{
		"timestamp":   time.Now().UnixMilli(),
		"profileId":   p.uuid,
		"profileName": p.name,
		"textures":    textures,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, "encode profile", http.StatusInternalServerError)
		return
	}
	value := base64.StdEncoding.EncodeToString(raw)
	sum := sha1.Sum([]byte(value))
	sig, err := rsa.SignPKCS1v15(rand.Reader, s.key, crypto.SHA1, sum[:])
	if err != nil {
		http.Error(w, "sign profile", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":   p.uuid,
		"name": p.name,
		"properties": []map[string]string{{
			"name":      "textures",
			"value":     value,
			"signature": base64.StdEncoding.EncodeToString(sig),
		}},
	})
}

func (s *Server) handleTexture(w http.ResponseWriter, r *http.Request) {
	hash := strings.ToLower(r.PathValue("hash"))
	s.mu.RLock()
	data, ok := s.textures[hash]
	s.mu.RUnlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Etag", `"`+hash+`"`)
	w.Header().Set("Cache-Control", "max-age=2592000, public")
	_, _ = w.Write(data)
}

func (s *Server) lookupName(name string) (string, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return "", false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	uuid, ok := s.byName[name]
	return uuid, ok
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// compactUUID accepts dashed or compact UUIDs and returns 32 lowercase hex
// characters, or an empty string when the input is not a UUID.
func compactUUID(uuid string) string {
	raw := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(uuid), "-", ""))
	if len(raw) != 32 {
		return ""
	}
	if _, err := hex.DecodeString(raw); err != nil {
		return ""
	}
	return raw
}
