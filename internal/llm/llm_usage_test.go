package llm

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func chatWithLocalServer(t *testing.T, handler http.HandlerFunc, wantError bool) *Manager {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	manager := &Manager{config: &Config{
		SelectedEngine: "local_qwen", LocalLLMURL: server.URL,
	}, httpClient: server.Client()}

	if _, used := manager.LastUsage(); used {
		t.Fatal("fresh manager reports usage")
	}
	err := manager.StreamChatContext(context.Background(), "hello", func(string) {})
	if (err != nil) != wantError {
		t.Fatalf("error = %v, wantError = %v", err, wantError)
	}
	return manager
}

func TestLastUsageRecordsAnsweringEngine(t *testing.T) {
	manager := chatWithLocalServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `data: {"choices":[{"delta":{"content":"xin chào"}}]}`)
		fmt.Fprintln(w, "data: [DONE]")
	}, false)

	usage, used := manager.LastUsage()
	if !used || usage.EngineID != "local_qwen" || usage.At.IsZero() {
		t.Fatalf("expected local_qwen usage, got %+v", usage)
	}
}

func TestLastUsageDoesNotRecordFailedProvider(t *testing.T) {
	manager := chatWithLocalServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusInternalServerError)
	}, true)

	usage, used := manager.LastUsage()
	if used {
		t.Fatalf("failed provider recorded usage: %+v", usage)
	}
}
