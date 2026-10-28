package main

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/nchungdev/home-sentinel/internal/adapters/nas/nastest"
	"github.com/nchungdev/home-sentinel/internal/tvmodes"
)

func TestTVModesListActivateAndRestore(t *testing.T) {
	sentinel := startSentinel(t, testTVToken, nastest.DefaultNAS())
	for _, path := range []string{"/v2/modes", "/api/tv-modes/config"} {
		if code := sentinel.status(t, http.MethodGet, path, ""); code != 401 {
			t.Fatalf("%s without token: want 401, got %d", path, code)
		}
	}

	state := modeState(t, sentinel, http.MethodGet, "/v2/modes")
	if len(state.Modes) == 0 || state.Active != "" {
		t.Fatalf("want default modes and none active, got %+v", state)
	}

	movie := state.Modes[0].ID
	active := modeState(t, sentinel, http.MethodPost, "/v2/modes/"+movie+"/activate")
	if active.Active != movie {
		t.Fatalf("want %s active, got %+v", movie, active)
	}
	normal := modeState(t, sentinel, http.MethodPost, "/v2/modes/normal/activate")
	if normal.Active != "" {
		t.Fatalf("want normal after restore, got %+v", normal)
	}
	unknown := sentinel.status(t, http.MethodPost, "/v2/modes/party/activate", testTVToken)
	if unknown != 404 {
		t.Fatalf("unknown mode: want 404, got %d", unknown)
	}
}

func modeState(t *testing.T, sentinel *testSentinel, method, path string) tvmodes.State {
	t.Helper()
	response, payload := sentinel.request(t, nil, method, path, testTVToken, nil)
	var state tvmodes.State
	if response.StatusCode != 200 || json.Unmarshal(payload, &state) != nil {
		t.Fatalf("%s %s: status %d %s", method, path, response.StatusCode, payload)
	}
	return state
}
