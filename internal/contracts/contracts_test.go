package contracts_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/nchungdev/home-sentinel/internal/contracts"
)

func readFixture(t *testing.T, filename string) []byte {
	t.Helper()
	path := filepath.Join("fixtures", filename)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", filename, err)
	}
	return data
}

func TestActionStatus(t *testing.T) {
	tests := []struct {
		input    string
		expected contracts.ActionStatus
		valid    bool
	}{
		{"accepted", contracts.ActionStatusAccepted, true},
		{"running", contracts.ActionStatusRunning, true},
		{"needs_input", contracts.ActionStatusNeedsInput, true},
		{"succeeded", contracts.ActionStatusSucceeded, true},
		{"failed", contracts.ActionStatusFailed, true},
		{"cancelled", contracts.ActionStatusCancelled, true},
		{"unknown", contracts.ActionStatusUnknown, true},
		{"ACCEPTED", contracts.ActionStatusAccepted, true},
		{" invalid ", contracts.ActionStatusUnknown, true},
	}

	for _, tt := range tests {
		status := contracts.ParseActionStatus(tt.input)
		if status != tt.expected {
			t.Errorf("ParseActionStatus(%q) = %v; want %v", tt.input, status, tt.expected)
		}
		if status.IsValid() != tt.valid {
			t.Errorf("status %q IsValid() = %v; want %v", tt.input, status.IsValid(), tt.valid)
		}

		rawJSON := `"` + tt.input + `"`
		var parsed contracts.ActionStatus
		if err := json.Unmarshal([]byte(rawJSON), &parsed); err != nil {
			t.Fatalf("json.Unmarshal failed: %v", err)
		}
		if parsed != tt.expected {
			t.Errorf("Unmarshal %s: got %v, want %v", rawJSON, parsed, tt.expected)
		}
	}
}

func TestInputEventFixtures(t *testing.T) {
	validData := readFixture(t, "input_event_valid.json")
	var validEvent contracts.InputEvent
	if err := validEvent.FromJson(validData); err != nil {
		t.Fatalf("failed to parse valid input event: %v", err)
	}
	if err := validEvent.Validate(); err != nil {
		t.Fatalf("expected valid input event to pass validation: %v", err)
	}
	if validEvent.RequestID != "req-1001" {
		t.Errorf("unexpected RequestID: %s", validEvent.RequestID)
	}
	if validEvent.Text != "bật quạt phòng khách" {
		t.Errorf("unexpected Text: %s", validEvent.Text)
	}
	if len(validEvent.Alternatives) != 2 {
		t.Errorf("expected 2 alternatives, got %d", len(validEvent.Alternatives))
	}

	marshaled, err := validEvent.ToJson()
	if err != nil {
		t.Fatalf("failed to marshal valid event: %v", err)
	}
	restored, err := contracts.InputEventFromJSON(marshaled)
	if err != nil {
		t.Fatalf("failed to restore marshaled event: %v", err)
	}
	if restored.RequestID != validEvent.RequestID || restored.Text != validEvent.Text {
		t.Errorf("restored event mismatch: %+v vs %+v", restored, validEvent)
	}

	invalidData := readFixture(t, "input_event_invalid.json")
	var invalidEvent contracts.InputEvent
	if err := invalidEvent.FromJSON(invalidData); err != nil {
		t.Fatalf("failed to parse invalid event JSON: %v", err)
	}
	if err := invalidEvent.Validate(); err != contracts.ErrInvalidInputEvent {
		t.Fatalf("expected ErrInvalidInputEvent, got: %v", err)
	}
}

func TestIntentFixture(t *testing.T) {
	data := readFixture(t, "intent_valid.json")
	intent, err := contracts.IntentFromJSON(data)
	if err != nil {
		t.Fatalf("failed to parse intent: %v", err)
	}
	if err := intent.Validate(); err != nil {
		t.Fatalf("intent validation failed: %v", err)
	}
	if intent.Name != "turn_on" || intent.Domain != "home_assistant" {
		t.Errorf("unexpected intent fields: %+v", intent)
	}
	if intent.Confidence < 0.94 {
		t.Errorf("unexpected confidence: %f", intent.Confidence)
	}
	if intent.Slots["device"] != "fan" {
		t.Errorf("unexpected slot device: %v", intent.Slots["device"])
	}
}

func TestActionPlanFixture(t *testing.T) {
	data := readFixture(t, "action_plan_valid.json")
	plan, err := contracts.ActionPlanFromJSON(data)
	if err != nil {
		t.Fatalf("failed to parse action plan: %v", err)
	}
	if err := plan.Validate(); err != nil {
		t.Fatalf("action plan validation failed: %v", err)
	}
	if plan.PlanID != "plan-101" || plan.IntentName != "turn_on" {
		t.Errorf("unexpected plan id/intent: %+v", plan)
	}
	if len(plan.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(plan.Steps))
	}
	step := plan.Steps[0]
	if step.StepID != "step-1" ||
		step.ActionType != "service_call" ||
		step.TargetID != "fan.living_room" {
		t.Errorf("unexpected step contents: %+v", step)
	}
	if !step.IsIdempotent {
		t.Errorf("expected is_idempotent to be true")
	}
}

func TestPresentationResponseFixture(t *testing.T) {
	data := readFixture(t, "presentation_response_valid.json")
	response, err := contracts.PresentationResponseFromJSON(data)
	if err != nil {
		t.Fatalf("failed to parse presentation response: %v", err)
	}
	if err := response.Validate(); err != nil {
		t.Fatalf("presentation response validation failed: %v", err)
	}
	if response.SpeechText != "Đã bật quạt phòng khách" {
		t.Errorf("unexpected speech text: %s", response.SpeechText)
	}
	if len(response.Cards) != 1 ||
		len(response.QuickChips) != 2 ||
		len(response.ClientActions) != 1 {
		t.Errorf("unexpected collections lengths: %+v", response)
	}
	if response.Mood != "friendly" {
		t.Errorf("unexpected mood: %s", response.Mood)
	}
}

func TestSuggestionFixture(t *testing.T) {
	data := readFixture(t, "suggestion_valid.json")
	suggestion, err := contracts.SuggestionFromJSON(data)
	if err != nil {
		t.Fatalf("failed to parse suggestion: %v", err)
	}
	if err := suggestion.Validate(); err != nil {
		t.Fatalf("suggestion validation failed: %v", err)
	}
	if suggestion.ID != "sug-901" || suggestion.Kind != "turn_off_reminder" {
		t.Errorf("unexpected suggestion: %+v", suggestion)
	}
	if suggestion.Action.StepID != "step-sug-1" {
		t.Errorf("unexpected nested action step: %+v", suggestion.Action)
	}
	if suggestion.GeneratedAt != 1729850000000 || suggestion.ExpiresAt != 1729853600000 {
		t.Errorf("unexpected timestamps in suggestion")
	}
}

func TestContextSnapshotFixture(t *testing.T) {
	data := readFixture(t, "context_snapshot_valid.json")
	snapshot, err := contracts.ContextSnapshotFromJSON(data)
	if err != nil {
		t.Fatalf("failed to parse context snapshot: %v", err)
	}
	if err := snapshot.Validate(); err != nil {
		t.Fatalf("context snapshot validation failed: %v", err)
	}
	if snapshot.TimestampEpochMs != 1729850000000 || snapshot.TimeOfDay != "evening" {
		t.Errorf("unexpected snapshot fields: %+v", snapshot)
	}
	if snapshot.ViewerProfile == nil || *snapshot.ViewerProfile != "husband" {
		t.Errorf("unexpected viewer profile: %v", snapshot.ViewerProfile)
	}
	if len(snapshot.RecentMedia) != 1 {
		t.Errorf("expected 1 recent media item")
	}
}

func TestProactiveChipFixture(t *testing.T) {
	data := readFixture(t, "proactive_chip_valid.json")
	chip, err := contracts.ProactiveChipFromJSON(data)
	if err != nil {
		t.Fatalf("failed to parse proactive chip: %v", err)
	}
	if err := chip.Validate(); err != nil {
		t.Fatalf("proactive chip validation failed: %v", err)
	}
	if chip.ChipID != "chip-quick-1" || chip.Label != "Tắt đèn phòng khách" {
		t.Errorf("unexpected chip fields: %+v", chip)
	}
}

func TestExecutionResultFixture(t *testing.T) {
	data := readFixture(t, "execution_result_valid.json")
	result, err := contracts.ExecutionResultFromJSON(data)
	if err != nil {
		t.Fatalf("failed to parse execution result: %v", err)
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("execution result validation failed: %v", err)
	}
	if result.StepID != "step-1" || result.Status != contracts.ActionStatusSucceeded {
		t.Errorf("unexpected result: %+v", result)
	}
	if result.Detail == nil || *result.Detail != "Command executed successfully" {
		t.Errorf("unexpected detail: %v", result.Detail)
	}
}
