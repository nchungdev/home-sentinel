package llm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestChatCancellationStopsProvider(t *testing.T) {
	started := make(chan struct{})
	stopped := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		close(started)
		select {
		case <-r.Context().Done():
		case <-time.After(3 * time.Second):
		}
		close(stopped)
	}))
	defer server.Close()
	manager := &Manager{config: &Config{
		PriorityOrder: []string{"local_qwen", "rule_fallback"}, LocalLLMURL: server.URL,
	}, httpClient: server.Client()}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		result <- manager.StreamChatContext(ctx, "hello", func(string) {
			t.Error("cancelled request emitted a response")
		})
	}()
	<-started
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("request did not stop after cancellation")
	}
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("provider HTTP request was not cancelled")
	}
}

func TestFailedStreamDoesNotContaminateFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `data: {"choices":[{"delta":{"content":"partial response"}}]}`)
		fmt.Fprintln(w, "data: "+strings.Repeat("x", 1024*1024))
	}))
	defer server.Close()
	manager := &Manager{config: &Config{
		PriorityOrder: []string{"unknown", "local_qwen", "rule_fallback"},
		LocalLLMURL:   server.URL,
	}, httpClient: server.Client()}
	var response strings.Builder
	err := manager.StreamChatContext(context.Background(), "hello", func(chunk string) {
		response.WriteString(chunk)
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(response.String(), "partial response") {
		t.Fatal("failed provider content leaked into fallback")
	}
	if response.Len() == 0 {
		t.Fatal("unknown engine prevented fallback")
	}
}

func TestCLIRespectsDeadline(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	manager := &Manager{}
	err := manager.callGenericCLI(ctx, "/bin/sh", []string{"-c", "echo unexpected"},
		func(string) { t.Error("cancelled CLI emitted output") })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}
