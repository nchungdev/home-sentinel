package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nchungdev/home-sentinel/internal/catalog"
	"github.com/nchungdev/home-sentinel/internal/playback"
)

func testCatalogServer(t *testing.T) (*Server, *http.ServeMux, *httptest.Server) {
	t.Helper()
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":true,"movie":{"name":"Movie","slug":"movie"},
"episodes":[{"server_name":"Main","server_data":[{"name":"Full","slug":"full",
"link_m3u8":"http://127.0.0.1/private.m3u8"}]}]}`))
	}))
	server := &Server{Playback: playback.New()}
	server.catalogOnce.Do(func() {
		server.catalogService = &catalog.Service{
			Sources: map[string]catalog.Source{"phimapi": catalog.NewPhimAPI(provider.URL)},
		}
	})
	mux := http.NewServeMux()
	registerSystemRoutes(mux, server)

	return server, mux, provider
}

func TestCatalogAPIRejectsPrivatePlaybackTargets(t *testing.T) {
	_, mux, provider := testCatalogServer(t)
	defer provider.Close()
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet,
		"/v2/catalog/titles/phimapi:movie", nil))
	var payload struct {
		Title catalog.Title `json:"title"`
	}
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &payload) != nil ||
		len(payload.Title.Episodes) != 1 {
		t.Fatalf("title API failed: %d %s", response.Code, response.Body.String())
	}
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodPost,
		"/v2/catalog/episodes/"+payload.Title.Episodes[0].ID+"/playback", strings.NewReader(`{}`)))
	if response.Code != http.StatusUnprocessableEntity ||
		strings.Contains(response.Body.String(), "127.0.0.1") {
		t.Fatalf("private stream reached playback proxy: %d %s", response.Code, response.Body)
	}
}

func TestCatalogAPIBoundsAndIDs(t *testing.T) {
	_, mux, provider := testCatalogServer(t)
	defer provider.Close()
	for _, test := range []struct {
		method string
		path   string
		body   string
		status int
	}{
		{http.MethodGet, "/v2/catalog/titles/phimapi:bad%3Fquery", "", 400},
		{http.MethodPost, "/v2/catalog/titles/phimapi:movie", "", 405},
		{http.MethodGet, "/v2/catalog/episodes/phimapi:invalid/playback", "", 405},
		{http.MethodPost, "/v2/catalog/episodes/phimapi:invalid/playback", `{}`, 400},
		{http.MethodPost, "/v2/catalog/episodes/phimapi:invalid/playback",
			`{"url":"http://127.0.0.1/private"}`, 400},
	} {
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequest(test.method, test.path,
			strings.NewReader(test.body)))
		if response.Code != test.status {
			t.Fatalf("%s %s returned %d, expected %d", test.method, test.path,
				response.Code, test.status)
		}
	}
}
