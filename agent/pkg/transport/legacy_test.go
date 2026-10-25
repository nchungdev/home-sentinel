package transport

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nchungdev/home-sentinel/agent/pkg/contracts"
	"github.com/nchungdev/home-sentinel/agent/pkg/orchestrator"
)

type dummyUnderstandingEngine struct {
	intent *contracts.Intent
	err    error
}

func (d *dummyUnderstandingEngine) Parse(
	ctx context.Context,
	input *contracts.InputEvent,
	reqCtx *contracts.RequestContext,
) (*contracts.Intent, error) {
	if d.err != nil {
		return nil, d.err
	}
	return d.intent, nil
}

type dummyResolutionEngine struct {
	plan *contracts.ActionPlan
	err  error
}

func (d *dummyResolutionEngine) Resolve(
	ctx context.Context,
	intent *contracts.Intent,
	reqCtx *contracts.RequestContext,
) (*contracts.ActionPlan, error) {
	if d.err != nil {
		return nil, d.err
	}
	return d.plan, nil
}

type dummyExecutionEngine struct {
	result *contracts.ExecutionResult
	err    error
}

func (d *dummyExecutionEngine) Execute(
	ctx context.Context,
	step *contracts.ActionStep,
	reqCtx *contracts.RequestContext,
) (*contracts.ExecutionResult, error) {
	if d.err != nil {
		return d.result, d.err
	}
	return d.result, nil
}

type dummyPresentationComposer struct {
	response *contracts.PresentationResponse
	err      error
}

func (d *dummyPresentationComposer) Compose(
	ctx context.Context,
	input *contracts.InputEvent,
	intent *contracts.Intent,
	plan *contracts.ActionPlan,
	results []*contracts.ExecutionResult,
	reqCtx *contracts.RequestContext,
) (*contracts.PresentationResponse, error) {
	if d.err != nil {
		return nil, d.err
	}
	return d.response, nil
}

func createTestOrchestrator(
	response *contracts.PresentationResponse,
	err error,
) *orchestrator.Orchestrator {
	understanding := &dummyUnderstandingEngine{
		intent: &contracts.Intent{Name: "TurnOnLight"},
	}
	resolution := &dummyResolutionEngine{
		plan: &contracts.ActionPlan{
			PlanID: "plan_test",
			Steps: []contracts.ActionStep{
				{StepID: "step_1", ActionType: "turn_on"},
			},
		},
	}
	execution := &dummyExecutionEngine{
		result: &contracts.ExecutionResult{
			StepID:   "step_1",
			Status:   contracts.ActionStatusSucceeded,
			CanRetry: false,
		},
	}
	presentation := &dummyPresentationComposer{
		response: response,
		err:      err,
	}

	return orchestrator.NewOrchestrator(understanding, resolution, execution, presentation)
}

func TestLegacyAdapterAskSuccess(t *testing.T) {
	orch := createTestOrchestrator(&contracts.PresentationResponse{
		SpeechText: "Đã bật đèn phòng khách thành công",
	}, nil)

	config := Config{
		Token:          "secret_token",
		DefaultTimeout: 5 * time.Second,
	}

	adapter := NewLegacyAdapter(config, orch)

	requestBody := `{"text": "bật đèn", "session": "s1"}`
	httpRequest := httptest.NewRequest(http.MethodPost, "/ask", strings.NewReader(requestBody))
	httpRequest.Header.Set("Authorization", "Bearer secret_token")
	httpRequest.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	adapter.ServeHTTP(recorder, httpRequest)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}

	contentType := recorder.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/plain") {
		t.Errorf("expected Content-Type text/plain, got %s", contentType)
	}

	body := recorder.Body.String()
	if body != "Đã bật đèn phòng khách thành công" {
		t.Errorf("expected speech text, got: %s", body)
	}
}

func TestLegacyAdapterWithCards(t *testing.T) {
	orch := createTestOrchestrator(&contracts.PresentationResponse{
		SpeechText: "Tìm thấy phim:",
		Cards: []map[string]any{
			{
				"title":  "Inception",
				"year":   2010,
				"status": "plex",
				"type":   "Movie",
				"poster": "http://nas:32400/thumb.jpg",
				"link":   "plex://play?id=123",
			},
		},
	}, nil)

	adapter := NewLegacyAdapter(Config{}, orch)

	requestBody := `{"text": "tìm phim inception", "session": "tv_sess"}`
	httpRequest := httptest.NewRequest(http.MethodPost, "/ask", strings.NewReader(requestBody))

	recorder := httptest.NewRecorder()
	adapter.ServeHTTP(recorder, httpRequest)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	body := recorder.Body.String()
	if !strings.Contains(body, "Tìm thấy phim:") {
		t.Errorf("missing speech text in: %s", body)
	}

	if !strings.Contains(body, `<div class="card"`) {
		t.Errorf("missing card html in: %s", body)
	}

	if !strings.Contains(body, `data-title="Inception"`) {
		t.Errorf("missing data-title in: %s", body)
	}

	if !strings.Contains(body, `data-status="plex"`) {
		t.Errorf("missing data-status in: %s", body)
	}

	if !strings.Contains(body, `<div class="grid">`) {
		t.Errorf("missing grid wrapper in: %s", body)
	}
}

func TestLegacyAdapterUnauthorized(t *testing.T) {
	orch := createTestOrchestrator(&contracts.PresentationResponse{SpeechText: "ok"}, nil)
	adapter := NewLegacyAdapter(Config{Token: "correct_token"}, orch)

	requestBody := `{"text": "bật đèn", "session": "s1"}`
	httpRequest := httptest.NewRequest(http.MethodPost, "/ask", strings.NewReader(requestBody))
	httpRequest.Header.Set("Authorization", "Bearer wrong_token")

	recorder := httptest.NewRecorder()
	adapter.ServeHTTP(recorder, httpRequest)

	if recorder.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d", recorder.Code)
	}
}

func TestLegacyAdapterForbiddenIP(t *testing.T) {
	orch := createTestOrchestrator(&contracts.PresentationResponse{SpeechText: "ok"}, nil)
	adapter := NewLegacyAdapter(Config{AllowedIPs: []string{"192.168.1.50"}}, orch)

	requestBody := `{"text": "bật đèn", "session": "s1"}`
	httpRequest := httptest.NewRequest(http.MethodPost, "/ask", strings.NewReader(requestBody))
	httpRequest.RemoteAddr = "10.0.0.1:12345"

	recorder := httptest.NewRecorder()
	adapter.ServeHTTP(recorder, httpRequest)

	if recorder.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden, got %d", recorder.Code)
	}
}

func TestLegacyAdapterMethodNotAllowed(t *testing.T) {
	orch := createTestOrchestrator(&contracts.PresentationResponse{SpeechText: "ok"}, nil)
	adapter := NewLegacyAdapter(Config{}, orch)

	httpRequest := httptest.NewRequest(http.MethodGet, "/ask", nil)
	recorder := httptest.NewRecorder()
	adapter.ServeHTTP(recorder, httpRequest)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 Method Not Allowed, got %d", recorder.Code)
	}
}

func TestLegacyAdapterEmptyQuestion(t *testing.T) {
	orch := createTestOrchestrator(&contracts.PresentationResponse{SpeechText: "ok"}, nil)
	adapter := NewLegacyAdapter(Config{}, orch)

	requestBody := `{"text": "   ", "session": "s1"}`
	httpRequest := httptest.NewRequest(http.MethodPost, "/ask", strings.NewReader(requestBody))

	recorder := httptest.NewRecorder()
	adapter.ServeHTTP(recorder, httpRequest)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d", recorder.Code)
	}
}

func TestLegacyAdapterBadJSON(t *testing.T) {
	orch := createTestOrchestrator(&contracts.PresentationResponse{SpeechText: "ok"}, nil)
	adapter := NewLegacyAdapter(Config{}, orch)

	httpRequest := httptest.NewRequest(http.MethodPost, "/ask", strings.NewReader("invalid json"))
	recorder := httptest.NewRecorder()
	adapter.ServeHTTP(recorder, httpRequest)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d", recorder.Code)
	}
}

func TestLegacyAdapterOrchestratorError(t *testing.T) {
	orch := createTestOrchestrator(nil, errors.New("backend failed"))
	adapter := NewLegacyAdapter(Config{}, orch)

	requestBody := `{"text": "hỏng backend", "session": "s1"}`
	httpRequest := httptest.NewRequest(http.MethodPost, "/ask", strings.NewReader(requestBody))

	recorder := httptest.NewRecorder()
	adapter.ServeHTTP(recorder, httpRequest)

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 Internal Server Error, got %d", recorder.Code)
	}
}

func TestFormatLegacyResponse(t *testing.T) {
	respSpeechOnly := &contracts.PresentationResponse{
		SpeechText: "Xin chào",
	}
	if FormatLegacyResponse(respSpeechOnly) != "Xin chào" {
		t.Errorf("unexpected output: %s", FormatLegacyResponse(respSpeechOnly))
	}

	respWithCard := &contracts.PresentationResponse{
		Cards: []map[string]any{
			{"title": "Test Device", "state": "on"},
		},
	}
	cardOutput := FormatLegacyResponse(respWithCard)
	if !strings.Contains(cardOutput, `<div class="card"`) {
		t.Errorf("expected card output, got: %s", cardOutput)
	}

	if FormatLegacyResponse(nil) != "" {
		t.Errorf("expected empty string for nil response")
	}
}

func TestStreamPresentationResponseNil(t *testing.T) {
	var buffer bytes.Buffer
	if err := StreamPresentationResponse(&buffer, nil); err != nil {
		t.Errorf("expected no error for nil, got %v", err)
	}
	if buffer.Len() != 0 {
		t.Errorf("expected empty buffer")
	}
}

func TestSplitStreamChunksReconstructsExactText(t *testing.T) {
	longText := "Hôm nay thời tiết Hà Nội rất đẹp và trời mát mẻ"
	chunks := splitStreamChunks(longText)

	if len(chunks) <= 1 {
		t.Fatalf("expected multiple chunks for long sentence, got %d", len(chunks))
	}

	reconstructed := strings.Join(chunks, "")
	if reconstructed != longText {
		t.Errorf("reconstructed text does not match original: %q vs %q", reconstructed, longText)
	}
}

func TestRenderCardWithRawHTML(t *testing.T) {
	card := map[string]any{
		"raw_html": `<div class="card custom">Raw</div>`,
	}
	rendered := RenderCardHTML(card)
	if rendered != `<div class="card custom">Raw</div>` {
		t.Errorf("unexpected rendered output: %s", rendered)
	}
}
