package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nchungdev/home-sentinel/agent/pkg/watching"
)

type staticLister struct {
	items []watching.Item
}

func (s staticLister) Items(context.Context) []watching.Item {
	return s.items
}

func watchConfig() *Config {
	cfg := ConfigFromEnv()
	cfg.Token = "secret"
	return &cfg
}

func TestWatchEventsRequiresToken(t *testing.T) {
	handler := NewV2WatchEventsHandler(watching.NewHistory(""), watchConfig())
	request := httptest.NewRequest(http.MethodPost, "/v2/watch/events", strings.NewReader("{}"))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", recorder.Code)
	}
}

func TestWatchEventsStoresValidEvents(t *testing.T) {
	history := watching.NewHistory("")
	handler := NewV2WatchEventsHandler(history, watchConfig())
	body := `{"events":[{"package_name":"a.pkg","title":"Show","state":"paused","at":1}]}`
	request := httptest.NewRequest(http.MethodPost, "/v2/watch/events", strings.NewReader(body))
	request.Header.Set(HeaderAuthorization, "Bearer secret")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestWatchEventsRejectsEmptyBatch(t *testing.T) {
	handler := NewV2WatchEventsHandler(watching.NewHistory(""), watchConfig())
	request := httptest.NewRequest(http.MethodPost, "/v2/watch/events",
		strings.NewReader(`{"events":[]}`))
	request.Header.Set(HeaderAuthorization, "Bearer secret")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", recorder.Code)
	}
}

func TestWatchContinueReturnsItems(t *testing.T) {
	lister := staticLister{items: []watching.Item{{ID: "plex:1", Title: "The Expanse"}}}
	handler := NewV2WatchContinueHandler(lister, watchConfig())
	request := httptest.NewRequest(http.MethodGet, "/v2/watch/continue", nil)
	request.Header.Set(HeaderAuthorization, "Bearer secret")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)
	var response struct {
		Items []watching.Item `json:"items"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusOK || len(response.Items) != 1 {
		t.Fatalf("want one item, got %d %s", recorder.Code, recorder.Body.String())
	}
}
