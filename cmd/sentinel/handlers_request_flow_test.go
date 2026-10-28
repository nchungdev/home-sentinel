package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nchungdev/home-sentinel/internal/requestflow"
)

func TestRequestFlowPreviewStreamsPlanAndPlaceholderResults(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/v2/flow/preview",
		strings.NewReader(`{"text":"Tìm cách sửa lỗi Windows"}`))
	response := httptest.NewRecorder()
	server := &Server{}
	server.handleRequestFlow(response, request)
	if response.Code != http.StatusOK ||
		!strings.HasPrefix(response.Header().Get("Content-Type"), "application/x-ndjson") {
		t.Fatalf("unexpected response: %d %s", response.Code, response.Body.String())
	}

	decoder := json.NewDecoder(response.Body)
	var events []requestflow.Event
	for decoder.More() {
		var event requestflow.Event
		if err := decoder.Decode(&event); err != nil {
			t.Fatal(err)
		}
		events = append(events, event)
	}
	if len(events) != 6 || events[0].Type != "plan_ready" ||
		events[len(events)-1].Status != "unavailable" {
		t.Fatalf("invalid placeholder stream: %+v", events)
	}
	for _, task := range events[0].Plan.Tasks {
		if task.Skill != "web.search" && task.Skill != "documentation.search" {
			t.Fatalf("technical request triggered media source: %+v", task)
		}
	}
}

func TestRequestFlowPreviewRejectsInvalidRequests(t *testing.T) {
	for _, body := range []string{
		`{}`, `{"text":" "}`, `{"text":"hi","unexpected":true}`,
		`{"text":"hi"} {}`, `{"text":`,
	} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/v2/flow/preview",
			strings.NewReader(body))
		(&Server{}).handleRequestFlow(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("accepted invalid body %q: %d", body, response.Code)
		}
	}
}
