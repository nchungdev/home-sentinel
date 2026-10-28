package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nchungdev/home-sentinel/internal/llm"
)

func TestEngineSettingsKeepKeysPrivate(t *testing.T) {
	manager := llm.NewManager(filepath.Join(t.TempDir(), "llm.json"))
	server := &Server{LLMManager: manager}
	save := httptest.NewRequest(http.MethodPost, "/api/llm/engines", strings.NewReader(
		`{"selected_engine":"gemini","gemini_key":"private-test-key"}`,
	))
	saved := httptest.NewRecorder()
	server.handleConfigLLM(saved, save)
	if !strings.Contains(saved.Body.String(), `"ok":true`) {
		t.Fatal(saved.Body.String())
	}
	response := httptest.NewRecorder()
	server.handleConfigLLM(response, httptest.NewRequest(http.MethodGet, "/api/llm/engines", nil))
	if strings.Contains(response.Body.String(), "private-test-key") {
		t.Fatal("GET exposed key")
	}
	if !strings.Contains(response.Body.String(), `"gemini_key_set":true`) {
		t.Fatal("GET omitted configured key status")
	}
	invalid := httptest.NewRecorder()
	server.handleConfigLLM(invalid, httptest.NewRequest(http.MethodPost, "/api/llm/engines",
		strings.NewReader(`{"selected_engine":"unknown"}`)))
	if !strings.Contains(invalid.Body.String(), `"ok":false`) {
		t.Fatal("invalid engine accepted")
	}
	if manager.Settings().SelectedEngine != "gemini" {
		t.Fatal("invalid request changed selection")
	}
}

func TestEngineTestDoesNotSaveSelection(t *testing.T) {
	manager := llm.NewManager(filepath.Join(t.TempDir(), "llm.json"))
	if err := manager.SaveSettings(llm.Settings{
		SelectedEngine: "gemini", GeminiKey: "private-test-key",
	}); err != nil {
		t.Fatal(err)
	}
	server := &Server{LLMManager: manager}
	response := httptest.NewRecorder()
	server.handleLLMTest(response, httptest.NewRequest(http.MethodPost, "/api/llm/test",
		strings.NewReader(`{"selected_engine":"rule_fallback"}`)))
	if !strings.Contains(response.Body.String(), `"ok":true`) {
		t.Fatal(response.Body.String())
	}
	if manager.Settings().SelectedEngine != "gemini" {
		t.Fatal("test saved draft selection")
	}
}
