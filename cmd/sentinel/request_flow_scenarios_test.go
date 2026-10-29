package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nchungdev/home-sentinel/internal/requestflow"
)

func TestRequestFlowHTTPScenarios(t *testing.T) {
	server := &Server{Auth: newAuthGate("preview-test-token", nil, "")}
	mux := http.NewServeMux()
	registerSystemRoutes(mux, server)
	endpoint := httptest.NewServer(server.Auth.Protect(mux))
	defer endpoint.Close()

	tests := []struct {
		text   string
		domain string
		intent string
		tasks  int
	}{
		{"Ai đạo diễn Interstellar?", "media", "lookup", 2},
		{"Tìm thông tin về phim Interstellar", "media", "lookup", 2},
		{"Tìm Interstellar để xem", "media", "find_to_watch", 2},
		{"Thêm phim Interstellar vào thư viện", "media", "request_library", 4},
		{"Tìm cách sửa lỗi Windows", "technical", "lookup", 2},
		{"Thời tiết hôm nay", "general", "lookup", 1},
		{"Tóm tắt https://example.com/article", "web", "summarize_url", 1},
		{"Interstellar", "media", "find_to_watch", 2},
		{"bạn nghĩ sao về chuyện này hôm qua nhỉ?", "general", "clarify", 0},
	}
	for _, test := range tests {
		t.Run(test.text, func(t *testing.T) {
			body, _ := json.Marshal(requestflow.Input{Text: test.text})
			request, err := http.NewRequest(http.MethodPost, endpoint.URL+"/v2/flow/preview",
				bytes.NewReader(body))
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("Authorization", "Bearer preview-test-token")
			response, err := endpoint.Client().Do(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != http.StatusOK {
				t.Fatalf("HTTP %d", response.StatusCode)
			}
			decoder := json.NewDecoder(response.Body)
			var first, last requestflow.Event
			if err := decoder.Decode(&first); err != nil {
				t.Fatal(err)
			}
			if first.Plan == nil || first.Type != "plan_ready" {
				t.Fatalf("missing initial plan: %+v", first)
			}
			sequence, results := 1, 0
			for decoder.More() {
				last = requestflow.Event{}
				if err := decoder.Decode(&last); err != nil {
					t.Fatal(err)
				}
				sequence++
				if last.Sequence != sequence || last.RequestID != first.RequestID {
					t.Fatal("invalid stream envelope")
				}
				if last.Result != nil {
					results++
					if last.Result.ErrorCode != "not_implemented" {
						t.Fatalf("unexpected live result: %+v", last.Result)
					}
				}
			}
			expectedStatus := "unavailable"
			if test.tasks == 0 {
				expectedStatus = "needs_input"
			}
			understanding := first.Plan.Understanding
			t.Logf("%s → %s/%s, tasks=%d, results=%d, terminal=%s",
				test.text, understanding.Domain, understanding.Intent,
				len(first.Plan.Tasks), results, last.Status)
			if understanding.Domain != test.domain || understanding.Intent != test.intent ||
				len(first.Plan.Tasks) != test.tasks || results != test.tasks ||
				last.Type != "response_completed" || last.Status != expectedStatus {
				t.Fatalf("unexpected plan/response: %+v; terminal=%+v", first.Plan, last)
			}
		})
	}

	response, err := endpoint.Client().Post(endpoint.URL+"/v2/flow/preview",
		"application/json", bytes.NewBufferString(`{"text":"tìm phim"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated preview accepted: %d", response.StatusCode)
	}
}
