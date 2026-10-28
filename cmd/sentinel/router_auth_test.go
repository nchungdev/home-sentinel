package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nchungdev/home-sentinel/internal/adapters/nas/nastest"
)

func TestProtectedRoutesRejectMissingAndWrongToken(t *testing.T) {
	sentinel := startSentinel(t, testTVToken, nastest.DefaultNAS())
	routes := []struct{ method, path string }{
		{http.MethodPost, "/v2/ask"},
		{http.MethodGet, "/v2/catalog/titles/phimapi:movie"},
		{http.MethodPost, "/v2/catalog/episodes/phimapi:episode/playback"},
		{http.MethodGet, "/v2/suggestions"},
		{http.MethodGet, "/v2/watch/continue"},
		{http.MethodGet, "/ws/duinch"},
		{http.MethodGet, "/api/nas/status"},
		{http.MethodPost, "/api/llm/login"},
		{http.MethodGet, "/api/llm/login"},
		{http.MethodGet, "/api/llm/auth"},
		{http.MethodGet, "/api/ha/config"},
		{http.MethodGet, "/api/camera/config"},
		{http.MethodPost, "/api/token/generate"},
		{http.MethodGet, "/api/automation/config"},
		{http.MethodPost, "/event"},
		{http.MethodGet, "/no/such/route"},
	}

	for _, route := range routes {
		for _, bearer := range []string{"", "wrong-token"} {
			if code := sentinel.status(t, route.method, route.path, bearer); code != 401 {
				t.Errorf("%s %s bearer=%q: expected 401, got %d",
					route.method, route.path, bearer, code)
			}
		}
	}
}

func TestPublicRoutes(t *testing.T) {
	sentinel := startSentinel(t, testTVToken, nastest.DefaultNAS())

	response, payload := sentinel.request(t, nil, http.MethodGet, "/", "", nil)
	if response.StatusCode != 200 || !strings.Contains(string(payload), "Home Sentinel") {
		t.Fatalf("index: status %d", response.StatusCode)
	}
	if code := sentinel.status(t, http.MethodGet, "/health", ""); code != 200 {
		t.Errorf("health without token: expected 200, got %d", code)
	}
	if code := sentinel.status(t, http.MethodGet, "/health", "wrong-token"); code != 401 {
		t.Errorf("health with wrong token: expected 401 for TV pairing, got %d", code)
	}
	if code := sentinel.status(t, http.MethodGet, "/health", testTVToken); code != 200 {
		t.Errorf("health with right token: expected 200, got %d", code)
	}
}

func TestBrowserLoginSetsSessionCookie(t *testing.T) {
	sentinel := startSentinel(t, testTVToken, nastest.DefaultNAS())
	browser := newBrowser(t)

	response, _ := sentinel.request(t, browser, http.MethodGet, "/api/nas/status", "", nil)
	if response.StatusCode != 401 {
		t.Fatalf("before login: expected 401, got %d", response.StatusCode)
	}

	response, _ = sentinel.request(t, browser, http.MethodPost, "/api/auth/login", "",
		map[string]string{"token": "wrong-token"})
	if response.StatusCode != 401 {
		t.Fatalf("wrong login: expected 401, got %d", response.StatusCode)
	}

	response, _ = sentinel.request(t, browser, http.MethodPost, "/api/auth/login", "",
		map[string]string{"token": testTVToken})
	if response.StatusCode != 200 {
		t.Fatalf("login: expected 200, got %d", response.StatusCode)
	}

	response, _ = sentinel.request(t, browser, http.MethodGet, "/api/nas/status", "", nil)
	if response.StatusCode != 200 {
		t.Fatalf("after login: expected 200, got %d", response.StatusCode)
	}

	_, payload := sentinel.request(t, browser, http.MethodGet, "/api/auth/status", "", nil)
	if !strings.Contains(string(payload), `"authorized":true`) {
		t.Errorf("auth status after login: %s", payload)
	}

	sentinel.request(t, browser, http.MethodPost, "/api/auth/logout", "", nil)
	response, _ = sentinel.request(t, browser, http.MethodGet, "/api/nas/status", "", nil)
	if response.StatusCode != 401 {
		t.Errorf("after logout: expected 401, got %d", response.StatusCode)
	}
}

func TestRotatedTokenReplacesOldEverywhereAndPersists(t *testing.T) {
	sentinel := startSentinel(t, "", nastest.DefaultNAS())
	initial, _ := sentinel.server.Auth.Rotate()

	response, payload := sentinel.request(
		t, nil, http.MethodPost, "/api/token/generate", initial, nil,
	)
	if response.StatusCode != 200 {
		t.Fatalf("generate: status %d %s", response.StatusCode, payload)
	}
	var body struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(payload, &body)

	if code := sentinel.status(t, http.MethodPost, "/v2/ask", initial); code != 401 {
		t.Errorf("old token on /v2/ask: expected 401, got %d", code)
	}
	if code := sentinel.status(t, http.MethodGet, "/api/nas/status", body.Token); code != 200 {
		t.Errorf("new token: expected 200, got %d", code)
	}

	storePath := filepath.Join(sentinel.dir, authStoreFile)
	info, err := os.Stat(storePath)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("token store: %v mode %v", err, info)
	}
	if !newAuthGate("", nil, storePath).Matches(body.Token) {
		t.Error("restarted gate does not accept persisted token")
	}
}

func TestEnvTokenCannotBeRotated(t *testing.T) {
	sentinel := startSentinel(t, testTVToken, nastest.DefaultNAS())

	code := sentinel.status(t, http.MethodPost, "/api/token/generate", testTVToken)
	if code != 409 {
		t.Errorf("expected 409 when AGENT_TOKEN manages the token, got %d", code)
	}
}

func TestAllowListBlocksOtherClients(t *testing.T) {
	sentinel := startSentinel(t, testTVToken, nastest.DefaultNAS())
	sentinel.server.Auth.allowedIPs = []string{"10.9.8.7"}

	if code := sentinel.status(t, http.MethodGet, "/health", testTVToken); code != 403 {
		t.Errorf("expected 403 outside AGENT_ALLOW, got %d", code)
	}
}

func TestWebhookAcceptsBasicAuthPassword(t *testing.T) {
	sentinel := startSentinel(t, testTVToken, nastest.DefaultNAS())

	request, _ := http.NewRequest(http.MethodPost,
		sentinel.baseURL+"/api/webhook/radarr", strings.NewReader(`{"eventType":"Test"}`))
	request.SetBasicAuth("radarr", testTVToken)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("webhook: %v", err)
	}
	response.Body.Close()
	if response.StatusCode == 401 {
		t.Error("webhook with basic auth password was rejected")
	}
}

func TestHAConfigNeverReturnsToken(t *testing.T) {
	sentinel := startSentinel(t, testTVToken, nastest.DefaultNAS())
	haToken := "ha-long-lived-secret"

	sentinel.request(t, nil, http.MethodPost, "/api/ha/config", testTVToken,
		map[string]string{"url": "http://127.0.0.1:1", "token": haToken})
	_, payload := sentinel.request(t, nil, http.MethodGet, "/api/ha/config", testTVToken, nil)
	if strings.Contains(string(payload), haToken) {
		t.Fatalf("HA token leaked: %s", payload)
	}
	if !strings.Contains(string(payload), `"token_set":true`) {
		t.Errorf("expected token_set: %s", payload)
	}

	sentinel.request(t, nil, http.MethodPost, "/api/ha/config", testTVToken,
		map[string]string{"url": "http://127.0.0.1:2", "token": ""})
	if saved := loadHAConfig("sentinel_ha_config.json"); saved.Token != haToken {
		t.Errorf("empty token should keep the saved one, got %q", saved.Token)
	}
}

func TestOpenModeKeepsWorkingWithoutToken(t *testing.T) {
	sentinel := startSentinel(t, "", nastest.DefaultNAS())

	speech := sentinel.ask(t, "", "ổ cứng còn bao nhiêu")
	if !strings.Contains(speech, "Ổ /srv/media") {
		t.Errorf("open mode ask: %s", speech)
	}
}
