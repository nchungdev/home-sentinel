package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/nchungdev/home-sentinel/internal/requestflow"
)

func TestLiveSearchFlowUsesProviderAndPreviewStaysOffline(t *testing.T) {
	var calls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{"results":[{"title":"Windows docs",
"url":"https://learn.microsoft.com/windows","content":"Provider fixture"}]}`))
	}))
	defer provider.Close()
	t.Setenv("SEARXNG_URL", provider.URL)
	t.Setenv("REDIS_URL", "")
	server := &Server{}
	for _, test := range []struct {
		live bool
		want string
	}{
		{true, "completed"}, {true, "completed"}, {false, "unavailable"},
	} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/v2/flow",
			strings.NewReader(`{"text":"Tìm cách sửa lỗi Windows"}`))
		if test.live {
			server.handleLiveRequestFlow(response, request)
		} else {
			server.handleRequestFlow(response, request)
		}
		if response.Code != http.StatusOK {
			t.Fatalf("HTTP %d: %s", response.Code, response.Body.String())
		}
		decoder := json.NewDecoder(response.Body)
		last := requestflow.Event{}
		results := 0
		for decoder.More() {
			last = requestflow.Event{}
			if err := decoder.Decode(&last); err != nil {
				t.Fatal(err)
			}
			if last.Result != nil && last.Result.Status == "ok" {
				results++
			}
		}
		if last.Status != test.want || (test.live && results != 2) {
			t.Fatalf("invalid live/preview result: %+v; successful tasks=%d", last, results)
		}
	}
	if calls.Load() != 2 {
		t.Fatalf("preview or repeated search called provider unexpectedly: %d", calls.Load())
	}
}

func TestLiveSearchDoesNotExecuteUnsupportedActions(t *testing.T) {
	server := &Server{}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v2/flow",
		strings.NewReader(`{"text":"Tắt đèn phòng khách"}`))
	server.handleLiveRequestFlow(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "legacy_route") {
		t.Fatalf("read-only flow tried to execute an action: %s", response.Body.String())
	}
}

func TestLiveSearchRejectsUnknownSourceSelection(t *testing.T) {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v2/flow", strings.NewReader(
		`{"text":"Tìm phim","context":{"sources":["invalid"]}}`,
	))
	(&Server{}).handleLiveRequestFlow(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("unknown source selection accepted: %d", response.Code)
	}
}
