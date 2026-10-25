package transport

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nchungdev/home-sentinel/agent/pkg/contracts"
	"github.com/nchungdev/home-sentinel/agent/pkg/orchestrator"
)

type mockOrchUnderstanding struct{}

func (m *mockOrchUnderstanding) Parse(
	ctx context.Context,
	input *contracts.InputEvent,
	reqCtx *contracts.RequestContext,
) (*contracts.Intent, error) {
	return &contracts.Intent{
		Name:       "media.search",
		Domain:     "media",
		Confidence: 1.0,
		Slots:      map[string]any{"query": "Dune 2"},
	}, nil
}

type mockOrchResolution struct{}

func (m *mockOrchResolution) Resolve(
	ctx context.Context,
	intent *contracts.Intent,
	reqCtx *contracts.RequestContext,
) (*contracts.ActionPlan, error) {
	return &contracts.ActionPlan{
		PlanID:     "plan-123",
		IntentName: intent.Name,
		Steps: []contracts.ActionStep{
			{
				StepID:     "step-1",
				ActionType: "plex.open",
				TargetID:   "12345",
				RunsOn:     "tv_client",
			},
		},
	}, nil
}

type mockOrchExecution struct{}

func (m *mockOrchExecution) Execute(
	ctx context.Context,
	step *contracts.ActionStep,
	reqCtx *contracts.RequestContext,
) (*contracts.ExecutionResult, error) {
	return &contracts.ExecutionResult{
		StepID: step.StepID,
		Status: contracts.ActionStatusSucceeded,
	}, nil
}

type mockOrchPresentation struct{}

func (m *mockOrchPresentation) Compose(
	ctx context.Context,
	input *contracts.InputEvent,
	intent *contracts.Intent,
	plan *contracts.ActionPlan,
	results []*contracts.ExecutionResult,
	reqCtx *contracts.RequestContext,
) (*contracts.PresentationResponse, error) {
	return &contracts.PresentationResponse{
		SpeechText: "Đã tìm thấy phim Dune 2 trên Plex",
		Cards: []map[string]any{
			{"title": "Dune 2", "status": "plex"},
		},
		ClientActions: []map[string]any{
			{
				"step_id":     "step-open-plex",
				"action_type": "plex.open",
				"target_id":   "12345",
			},
		},
		Mood: "excited",
	}, nil
}

type mockErrorUnderstanding struct{}

func (m *mockErrorUnderstanding) Parse(
	ctx context.Context,
	input *contracts.InputEvent,
	reqCtx *contracts.RequestContext,
) (*contracts.Intent, error) {
	return nil, errors.New("simulated pipeline error")
}

func TestV2Handler_FullFlow(t *testing.T) {
	orch := orchestrator.NewOrchestrator(
		&mockOrchUnderstanding{},
		&mockOrchResolution{},
		&mockOrchExecution{},
		&mockOrchPresentation{},
	)
	cfg := &Config{
		Token: "test_secret_token",
	}
	handler := NewV2Handler(orch, cfg)

	input := contracts.InputEvent{
		RequestID:    "req-1",
		SessionID:    "sess-1",
		UtteranceID:  "utt-1",
		Text:         "tìm phim Dune 2",
		Alternatives: []string{},
		Locale:       "vi-VN",
		InputType:    "voice",
	}
	bodyBytes, _ := json.Marshal(input)

	req := httptest.NewRequest(http.MethodPost, "/v2/ask", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer test_secret_token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/x-ndjson; charset=utf-8" {
		t.Fatalf("expected application/x-ndjson, got %s", contentType)
	}

	scanner := bufio.NewScanner(rec.Body)
	var eventTypes []string

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var evt V2StreamEvent
		if err := json.Unmarshal([]byte(line), &evt); err != nil {
			t.Fatalf("failed to parse event json: %v, raw: %s", err, line)
		}
		eventTypes = append(eventTypes, string(evt.Type))
	}

	expectedTypes := []string{"accepted", "progress", "presentation", "client_action", "completed"}
	if len(eventTypes) != len(expectedTypes) {
		t.Fatalf("expected %d events, got %d (%v)", len(expectedTypes), len(eventTypes), eventTypes)
	}

	for i, exp := range expectedTypes {
		if eventTypes[i] != exp {
			t.Errorf("event [%d]: expected %s, got %s", i, exp, eventTypes[i])
		}
	}
}

func TestV2Handler_Unauthorized(t *testing.T) {
	orch := orchestrator.NewOrchestrator(
		&mockOrchUnderstanding{},
		&mockOrchResolution{},
		&mockOrchExecution{},
		&mockOrchPresentation{},
	)
	cfg := &Config{
		Token: "correct_token",
	}
	handler := NewV2Handler(orch, cfg)

	req := httptest.NewRequest(http.MethodPost, "/v2/ask", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Authorization", "Bearer wrong_token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d", rec.Code)
	}
}

func TestV2Handler_MethodNotAllowed(t *testing.T) {
	orch := orchestrator.NewOrchestrator(
		&mockOrchUnderstanding{},
		&mockOrchResolution{},
		&mockOrchExecution{},
		&mockOrchPresentation{},
	)
	cfg := &Config{Token: "test"}
	handler := NewV2Handler(orch, cfg)

	req := httptest.NewRequest(http.MethodGet, "/v2/ask", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 Method Not Allowed, got %d", rec.Code)
	}
}

func TestV2Handler_PipelineError(t *testing.T) {
	orch := orchestrator.NewOrchestrator(
		&mockErrorUnderstanding{},
		&mockOrchResolution{},
		&mockOrchExecution{},
		&mockOrchPresentation{},
	)
	cfg := &Config{Token: "test_token"}
	handler := NewV2Handler(orch, cfg)

	input := contracts.InputEvent{
		RequestID: "req-err",
		Text:      "lỗi mô phỏng",
	}
	bodyBytes, _ := json.Marshal(input)

	req := httptest.NewRequest(http.MethodPost, "/v2/ask", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer test_token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for stream init, got %d", rec.Code)
	}

	scanner := bufio.NewScanner(rec.Body)
	var hasErrorEvent bool

	for scanner.Scan() {
		var evt V2StreamEvent
		if err := json.Unmarshal(scanner.Bytes(), &evt); err == nil {
			if evt.Type == V2EventError {
				hasErrorEvent = true
			}
		}
	}

	if !hasErrorEvent {
		t.Fatal("expected V2EventError event in stream")
	}
}
