package transport

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nchungdev/home-sentinel/agent/pkg/contracts"
	"github.com/nchungdev/home-sentinel/agent/pkg/proactive"
)

func TestV2SuggestionsUnauthorized(t *testing.T) {
	engine := proactive.NewSuggestionEngine(nil, nil)
	cfg := &Config{
		Token:          "valid-token",
		DefaultTimeout: 5 * time.Second,
	}
	handler := NewV2SuggestionsHandler(engine, cfg)

	req := httptest.NewRequest(http.MethodGet, "/v2/suggestions", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rec.Code)
	}
}

func TestV2SuggestionsSuccess(t *testing.T) {
	engine := proactive.NewSuggestionEngine(nil, nil)
	cfg := &Config{
		Token:          "valid-token",
		DefaultTimeout: 5 * time.Second,
	}
	handler := NewV2SuggestionsHandler(engine, cfg)

	req := httptest.NewRequest(http.MethodGet, "/v2/suggestions", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", rec.Code)
	}

	var resp struct {
		Suggestions []*contracts.Suggestion `json:"suggestions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if len(resp.Suggestions) == 0 {
		t.Errorf("Expected suggestions, got empty list")
	}
}
