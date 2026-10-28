package orchestrator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nchungdev/home-sentinel/internal/contracts"
)

type mockUnderstandingEngine struct {
	intentToReturn *contracts.Intent
	errToReturn    error
	calls          int
}

func (m *mockUnderstandingEngine) Parse(
	ctx context.Context,
	input *contracts.InputEvent,
	reqCtx *contracts.RequestContext,
) (*contracts.Intent, error) {
	m.calls++
	if m.errToReturn != nil {
		return nil, m.errToReturn
	}
	return m.intentToReturn, nil
}

type mockResolutionEngine struct {
	planToReturn *contracts.ActionPlan
	errToReturn  error
	calls        int
}

func (m *mockResolutionEngine) Resolve(
	ctx context.Context,
	intent *contracts.Intent,
	reqCtx *contracts.RequestContext,
) (*contracts.ActionPlan, error) {
	m.calls++
	if m.errToReturn != nil {
		return nil, m.errToReturn
	}
	return m.planToReturn, nil
}

type mockExecutionEngine struct {
	stepResults map[string]*contracts.ExecutionResult
	stepErrors  map[string]error
	calledSteps []string
	delay       time.Duration
}

func (m *mockExecutionEngine) Execute(
	ctx context.Context,
	step *contracts.ActionStep,
	reqCtx *contracts.RequestContext,
) (*contracts.ExecutionResult, error) {
	m.calledSteps = append(m.calledSteps, step.StepID)

	if m.delay > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(m.delay):
		}
	}

	if err, exists := m.stepErrors[step.StepID]; exists && err != nil {
		return m.stepResults[step.StepID], err
	}

	if res, exists := m.stepResults[step.StepID]; exists {
		return res, nil
	}

	return &contracts.ExecutionResult{
		StepID:   step.StepID,
		Status:   contracts.ActionStatusSucceeded,
		CanRetry: false,
	}, nil
}

type mockPresentationComposer struct {
	responseToReturn *contracts.PresentationResponse
	errToReturn      error
	capturedResults  []*contracts.ExecutionResult
	calls            int
}

func (m *mockPresentationComposer) Compose(
	ctx context.Context,
	input *contracts.InputEvent,
	intent *contracts.Intent,
	plan *contracts.ActionPlan,
	results []*contracts.ExecutionResult,
	reqCtx *contracts.RequestContext,
) (*contracts.PresentationResponse, error) {
	m.calls++
	m.capturedResults = results
	if m.errToReturn != nil {
		return nil, m.errToReturn
	}
	return m.responseToReturn, nil
}

func TestOrchestratorSuccessSequentialPipeline(t *testing.T) {
	understanding := &mockUnderstandingEngine{
		intentToReturn: &contracts.Intent{
			Name:   "TurnOnLight",
			Domain: "home_assistant",
		},
	}

	resolution := &mockResolutionEngine{
		planToReturn: &contracts.ActionPlan{
			PlanID: "plan_1",
			Steps: []contracts.ActionStep{
				{StepID: "step_1", ActionType: "turn_on"},
				{StepID: "step_2", ActionType: "notify"},
			},
		},
	}

	execution := &mockExecutionEngine{
		stepResults: map[string]*contracts.ExecutionResult{
			"step_1": {
				StepID:   "step_1",
				Status:   contracts.ActionStatusSucceeded,
				CanRetry: false,
			},
			"step_2": {
				StepID:   "step_2",
				Status:   contracts.ActionStatusSucceeded,
				CanRetry: false,
			},
		},
		stepErrors: make(map[string]error),
	}

	presentation := &mockPresentationComposer{
		responseToReturn: &contracts.PresentationResponse{
			SpeechText: "Đã bật đèn và thông báo xong",
			Mood:       "friendly",
		},
	}

	orch := NewOrchestrator(understanding, resolution, execution, presentation)

	inputEvent := &contracts.InputEvent{
		RequestID: "req_123",
		SessionID: "sess_456",
		Text:      "Bật đèn và gửi thông báo",
	}

	reqCtx := &contracts.RequestContext{
		ClientID: "tv_client_living_room",
	}

	resp, err := orch.Process(context.Background(), inputEvent, reqCtx)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if resp.SpeechText != "Đã bật đèn và thông báo xong" {
		t.Errorf("unexpected speech text: %s", resp.SpeechText)
	}

	if understanding.calls != 1 {
		t.Errorf("expected 1 understanding call, got %d", understanding.calls)
	}

	if resolution.calls != 1 {
		t.Errorf("expected 1 resolution call, got %d", resolution.calls)
	}

	if len(execution.calledSteps) != 2 {
		t.Fatalf("expected 2 execution steps called, got %d", len(execution.calledSteps))
	}

	if execution.calledSteps[0] != "step_1" || execution.calledSteps[1] != "step_2" {
		t.Errorf("unexpected execution step order: %v", execution.calledSteps)
	}

	if presentation.calls != 1 {
		t.Errorf("expected 1 presentation call, got %d", presentation.calls)
	}

	if len(presentation.capturedResults) != 2 {
		t.Fatalf("expected 2 execution results, got %d", len(presentation.capturedResults))
	}
}

func TestOrchestratorStopWhenStepFailsWithStatus(t *testing.T) {
	understanding := &mockUnderstandingEngine{
		intentToReturn: &contracts.Intent{Name: "TurnOnLight"},
	}

	resolution := &mockResolutionEngine{
		planToReturn: &contracts.ActionPlan{
			PlanID: "plan_fail",
			Steps: []contracts.ActionStep{
				{StepID: "step_1", ActionType: "turn_on"},
				{StepID: "step_2", ActionType: "turn_on_second"},
			},
		},
	}

	failDetail := "device unreachable"
	execution := &mockExecutionEngine{
		stepResults: map[string]*contracts.ExecutionResult{
			"step_1": {
				StepID:   "step_1",
				Status:   contracts.ActionStatusFailed,
				Detail:   &failDetail,
				CanRetry: false,
			},
		},
		stepErrors: make(map[string]error),
	}

	presentation := &mockPresentationComposer{
		responseToReturn: &contracts.PresentationResponse{
			SpeechText: "Không thể kết nối tới đèn",
		},
	}

	orch := NewOrchestrator(understanding, resolution, execution, presentation)

	inputEvent := &contracts.InputEvent{
		RequestID: "req_fail_1",
		Text:      "Bật 2 bóng đèn",
	}

	resp, err := orch.Process(context.Background(), inputEvent, nil)
	if err != nil {
		t.Fatalf("expected no process error, got %v", err)
	}

	if resp.SpeechText != "Không thể kết nối tới đèn" {
		t.Errorf("unexpected speech text: %s", resp.SpeechText)
	}

	if len(execution.calledSteps) != 1 {
		t.Fatalf("expected 1 execution step, got %d", len(execution.calledSteps))
	}

	if execution.calledSteps[0] != "step_1" {
		t.Errorf("expected step_1, got %s", execution.calledSteps[0])
	}

	if len(presentation.capturedResults) != 1 {
		t.Fatalf("expected 1 captured result, got %d", len(presentation.capturedResults))
	}

	if presentation.capturedResults[0].Status != contracts.ActionStatusFailed {
		t.Errorf("expected failed status, got %s", presentation.capturedResults[0].Status)
	}
}

func TestOrchestratorStopWhenStepFailsWithError(t *testing.T) {
	understanding := &mockUnderstandingEngine{
		intentToReturn: &contracts.Intent{Name: "TurnOnLight"},
	}

	resolution := &mockResolutionEngine{
		planToReturn: &contracts.ActionPlan{
			PlanID: "plan_error",
			Steps: []contracts.ActionStep{
				{StepID: "step_err", ActionType: "turn_on"},
				{StepID: "step_never", ActionType: "followup"},
			},
		},
	}

	execution := &mockExecutionEngine{
		stepResults: make(map[string]*contracts.ExecutionResult),
		stepErrors: map[string]error{
			"step_err": errors.New("network socket timeout"),
		},
	}

	presentation := &mockPresentationComposer{
		responseToReturn: &contracts.PresentationResponse{
			SpeechText: "Lỗi kết nối mạng",
		},
	}

	orch := NewOrchestrator(understanding, resolution, execution, presentation)

	inputEvent := &contracts.InputEvent{
		RequestID: "req_error_1",
		Text:      "Bật đèn",
	}

	resp, err := orch.Process(context.Background(), inputEvent, nil)
	if err != nil {
		t.Fatalf("expected no process error, got %v", err)
	}

	if resp.SpeechText != "Lỗi kết nối mạng" {
		t.Errorf("unexpected speech text: %s", resp.SpeechText)
	}

	if len(execution.calledSteps) != 1 {
		t.Fatalf("expected only step_err called, got %d steps", len(execution.calledSteps))
	}

	if len(presentation.capturedResults) != 1 {
		t.Fatalf("expected 1 result, got %d", len(presentation.capturedResults))
	}

	if presentation.capturedResults[0].Status != contracts.ActionStatusFailed {
		t.Errorf("expected failed status, got %s", presentation.capturedResults[0].Status)
	}
}

func TestOrchestratorContextTimeout(t *testing.T) {
	understanding := &mockUnderstandingEngine{
		intentToReturn: &contracts.Intent{Name: "PlayMovie"},
	}

	resolution := &mockResolutionEngine{
		planToReturn: &contracts.ActionPlan{
			PlanID: "plan_timeout",
			Steps: []contracts.ActionStep{
				{StepID: "step_slow", ActionType: "search"},
			},
		},
	}

	execution := &mockExecutionEngine{
		stepResults: make(map[string]*contracts.ExecutionResult),
		stepErrors:  make(map[string]error),
		delay:       100 * time.Millisecond,
	}

	presentation := &mockPresentationComposer{
		responseToReturn: &contracts.PresentationResponse{SpeechText: "done"},
	}

	orch := NewOrchestrator(understanding, resolution, execution, presentation)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	inputEvent := &contracts.InputEvent{
		RequestID: "req_timeout",
		Text:      "Tìm phim chiếu rạp",
	}

	_, err := orch.Process(ctx, inputEvent, nil)
	if err == nil {
		t.Fatal("expected context timeout error, got nil")
	}

	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Errorf("expected deadline exceeded or canceled, got: %v", err)
	}

	if presentation.calls != 0 {
		t.Errorf("expected no presentation call on timeout, got %d", presentation.calls)
	}
}

func TestOrchestratorUnderstandingError(t *testing.T) {
	expectedErr := errors.New("failed to understand prompt")
	understanding := &mockUnderstandingEngine{
		errToReturn: expectedErr,
	}

	resolution := &mockResolutionEngine{}
	execution := &mockExecutionEngine{
		stepResults: make(map[string]*contracts.ExecutionResult),
		stepErrors:  make(map[string]error),
	}
	presentation := &mockPresentationComposer{}

	orch := NewOrchestrator(understanding, resolution, execution, presentation)

	inputEvent := &contracts.InputEvent{
		RequestID: "req_u_err",
		Text:      "Câu lệnh không rõ nghĩa",
	}

	_, err := orch.Process(context.Background(), inputEvent, nil)
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected %v, got %v", expectedErr, err)
	}

	if resolution.calls != 0 {
		t.Errorf("resolution should not be called")
	}

	if len(execution.calledSteps) != 0 {
		t.Errorf("execution should not be called")
	}

	if presentation.calls != 0 {
		t.Errorf("presentation should not be called")
	}
}

func TestOrchestratorResolutionError(t *testing.T) {
	expectedErr := errors.New("cannot resolve intent target")
	understanding := &mockUnderstandingEngine{
		intentToReturn: &contracts.Intent{Name: "UnknownDevice"},
	}

	resolution := &mockResolutionEngine{
		errToReturn: expectedErr,
	}

	execution := &mockExecutionEngine{
		stepResults: make(map[string]*contracts.ExecutionResult),
		stepErrors:  make(map[string]error),
	}
	presentation := &mockPresentationComposer{}

	orch := NewOrchestrator(understanding, resolution, execution, presentation)

	inputEvent := &contracts.InputEvent{
		RequestID: "req_r_err",
		Text:      "Bật thiết bị lạ",
	}

	_, err := orch.Process(context.Background(), inputEvent, nil)
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected %v, got %v", expectedErr, err)
	}

	if len(execution.calledSteps) != 0 {
		t.Errorf("execution should not be called")
	}

	if presentation.calls != 0 {
		t.Errorf("presentation should not be called")
	}
}

func TestOrchestratorNilEngineError(t *testing.T) {
	orch := NewOrchestrator(nil, nil, nil, nil)

	inputEvent := &contracts.InputEvent{
		RequestID: "req_nil",
		Text:      "test",
	}

	_, err := orch.Process(context.Background(), inputEvent, nil)
	if !errors.Is(err, ErrNilEngine) {
		t.Errorf("expected ErrNilEngine, got %v", err)
	}
}

func TestOrchestratorInvalidInputEvent(t *testing.T) {
	understanding := &mockUnderstandingEngine{}
	resolution := &mockResolutionEngine{}
	execution := &mockExecutionEngine{
		stepResults: make(map[string]*contracts.ExecutionResult),
		stepErrors:  make(map[string]error),
	}
	presentation := &mockPresentationComposer{}

	orch := NewOrchestrator(understanding, resolution, execution, presentation)

	_, err := orch.Process(context.Background(), nil, nil)
	if !errors.Is(err, contracts.ErrInvalidInputEvent) {
		t.Errorf("expected ErrInvalidInputEvent, got %v", err)
	}

	invalidEvent := &contracts.InputEvent{
		RequestID: "",
		Text:      "",
	}

	_, err = orch.Process(context.Background(), invalidEvent, nil)
	if !errors.Is(err, contracts.ErrInvalidInputEvent) {
		t.Errorf("expected ErrInvalidInputEvent, got %v", err)
	}
}
