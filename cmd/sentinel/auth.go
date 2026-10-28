package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/nchungdev/home-sentinel/internal/transport"
)

const (
	authStoreFile     = "sentinel_auth.json"
	sessionCookieName = "sentinel_session"
	sessionMaxAge     = 30 * 24 * 60 * 60
	tokenBytes        = 24
)

var errTokenFromEnv = errors.New("token is managed by AGENT_TOKEN")

type authGate struct {
	mutex      sync.RWMutex
	token      string
	fromEnv    bool
	storePath  string
	allowedIPs []string
}

type authStore struct {
	Token string `json:"token"`
}

func newAuthGate(envToken string, allowedIPs []string, storePath string) *authGate {
	gate := &authGate{storePath: storePath, allowedIPs: allowedIPs}
	if envToken != "" {
		gate.token = envToken
		gate.fromEnv = true
		return gate
	}

	var stored authStore
	if data, err := os.ReadFile(storePath); err == nil && json.Unmarshal(data, &stored) == nil {
		gate.token = stored.Token
	}
	return gate
}

func (g *authGate) Required() bool {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return g.token != ""
}

func (g *authGate) Token() string {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return g.token
}

func (g *authGate) Rotate() (string, error) {
	if g.fromEnv {
		return "", errTokenFromEnv
	}

	raw := make([]byte, tokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := hex.EncodeToString(raw)

	data, err := json.Marshal(authStore{Token: token})
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(g.storePath, data, 0o600); err != nil {
		return "", err
	}

	g.mutex.Lock()
	g.token = token
	g.mutex.Unlock()
	return token, nil
}

func (g *authGate) Matches(candidate string) bool {
	g.mutex.RLock()
	expected := g.token
	g.mutex.RUnlock()

	if expected == "" {
		return true
	}
	if candidate == "" {
		return false
	}
	expectedHash := sha256.Sum256([]byte(expected))
	candidateHash := sha256.Sum256([]byte(candidate))
	return subtle.ConstantTimeCompare(expectedHash[:], candidateHash[:]) == 1
}

func (g *authGate) Authorized(r *http.Request) bool {
	return g.Matches(requestCredential(r))
}

func requestCredential(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if strings.HasPrefix(header, "Bearer ") {
		return strings.TrimPrefix(header, "Bearer ")
	}
	if _, password, ok := r.BasicAuth(); ok {
		return password
	}
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		return cookie.Value
	}
	return ""
}

func (g *authGate) AllowNetwork(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !transport.IsIPAllowed(r.RemoteAddr, g.allowedIPs) {
			writeJSONError(w, http.StatusForbidden, "forbidden")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (g *authGate) Protect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !g.Authorized(r) {
			writeJSONError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   sessionMaxAge,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

func writeJSONError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
}
