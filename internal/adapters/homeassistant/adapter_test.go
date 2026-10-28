package homeassistant

import (
	"context"
	"testing"

	"github.com/nchungdev/home-sentinel/internal/contracts"
	"github.com/nchungdev/home-sentinel/internal/integrations/haclient"
)

type mockRequester struct {
	calls        []string
	requestFunc  func(path string, body any, out any) error
}

func (m *mockRequester) RequestJSON(path string, body any, out any) error {
	m.calls = append(m.calls, path)
	if m.requestFunc != nil {
		return m.requestFunc(path, body, out)
	}
	return nil
}

func TestProtectedEntity(t *testing.T) {
	mock := &mockRequester{}
	adapter := NewAdapter(mock, Config{
		ProtectedEntities: []string{"light.nas_power"},
	})

	step := &contracts.ActionStep{
		StepID:     "step-1",
		ActionType: "ha.control",
		TargetID:   "light.nas_power",
		Parameters: map[string]any{"command": "turn_off"},
	}

	result, err := adapter.Execute(context.Background(), step)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.Status != contracts.ActionStatusFailed {
		t.Errorf("Expected status failed, got %s", result.Status)
	}
	if result.ErrorCode == nil || *result.ErrorCode != "PROTECTED_ENTITY" {
		t.Errorf("Expected code PROTECTED_ENTITY, got %v", result.ErrorCode)
	}
}

func TestUnavailableEntity(t *testing.T) {
	mock := &mockRequester{
		requestFunc: func(path string, body any, out any) error {
			if path == "/api/states/light.bedroom" {
				if s, ok := out.(*haclient.EntityState); ok {
					s.EntityID = "light.bedroom"
					s.State = "unavailable"
				}
			}
			return nil
		},
	}
	adapter := NewAdapter(mock, Config{})

	step := &contracts.ActionStep{
		StepID:     "step-2",
		ActionType: "ha.control",
		TargetID:   "light.bedroom",
		Parameters: map[string]any{"command": "turn_on"},
	}

	result, err := adapter.Execute(context.Background(), step)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.Status != contracts.ActionStatusFailed {
		t.Errorf("Expected status failed, got %s", result.Status)
	}
	if result.ErrorCode == nil || *result.ErrorCode != "ENTITY_UNAVAILABLE" {
		t.Errorf("Expected code ENTITY_UNAVAILABLE, got %v", result.ErrorCode)
	}
}

func TestUnsupportedDomain(t *testing.T) {
	mock := &mockRequester{}
	adapter := NewAdapter(mock, Config{})

	step := &contracts.ActionStep{
		StepID:     "step-3",
		ActionType: "ha.control",
		TargetID:   "camera.front_door",
		Parameters: map[string]any{"command": "turn_on"},
	}

	result, err := adapter.Execute(context.Background(), step)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.Status != contracts.ActionStatusFailed {
		t.Errorf("Expected status failed, got %s", result.Status)
	}
	if result.ErrorCode == nil || *result.ErrorCode != "UNSUPPORTED_DOMAIN" {
		t.Errorf("Expected code UNSUPPORTED_DOMAIN, got %v", result.ErrorCode)
	}
}

func TestTemperatureSafetyLimits(t *testing.T) {
	mock := &mockRequester{
		requestFunc: func(path string, body any, out any) error {
			if path == "/api/states/climate.bedroom" {
				if s, ok := out.(*haclient.EntityState); ok {
					s.EntityID = "climate.bedroom"
					s.State = "cool"
					s.Attributes = map[string]any{
						"min_temp": 16.0,
						"max_temp": 30.0,
					}
				}
			}
			return nil
		},
	}
	adapter := NewAdapter(mock, Config{})

	step := &contracts.ActionStep{
		StepID:     "step-4",
		ActionType: "ha.temperature",
		TargetID:   "climate.bedroom",
		Parameters: map[string]any{"temperature": 10.0},
	}

	result, err := adapter.Execute(context.Background(), step)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.Status != contracts.ActionStatusFailed {
		t.Errorf("Expected status failed, got %s", result.Status)
	}
	if result.ErrorCode == nil || *result.ErrorCode != "SERVICE_PREPARATION_FAILED" {
		t.Errorf("Expected SERVICE_PREPARATION_FAILED, got %v", result.ErrorCode)
	}
}

func TestServiceDomainAndReadback(t *testing.T) {
	readbackCount := 0
	mock := &mockRequester{
		requestFunc: func(path string, body any, out any) error {
			if path == "/api/states/light.living_room" {
				if s, ok := out.(*haclient.EntityState); ok {
					s.EntityID = "light.living_room"
					s.Attributes = map[string]any{"friendly_name": "Đèn phòng khách"}
					if readbackCount == 0 {
						s.State = "off"
						readbackCount++
					} else {
						s.State = "on"
					}
				}
			}
			return nil
		},
	}
	adapter := NewAdapter(mock, Config{})

	step := &contracts.ActionStep{
		StepID:     "step-5",
		ActionType: "ha.control",
		TargetID:   "light.living_room",
		Parameters: map[string]any{"command": "turn_on"},
	}

	result, err := adapter.Execute(context.Background(), step)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.Status != contracts.ActionStatusSucceeded {
		t.Errorf("Expected status succeeded, got %s", result.Status)
	}
	if result.ActualState["state"] != "on" {
		t.Errorf("Expected actual state on, got %v", result.ActualState["state"])
	}

	expectedServiceCall := "/api/services/light/turn_on"
	foundServiceCall := false
	for _, call := range mock.calls {
		if call == expectedServiceCall {
			foundServiceCall = true
			break
		}
	}
	if !foundServiceCall {
		t.Errorf("Expected service call %s, got calls: %v", expectedServiceCall, mock.calls)
	}
}

func TestIdempotencySkipsRedundantCall(t *testing.T) {
	serviceCalls := 0
	mock := &mockRequester{
		requestFunc: func(path string, body any, out any) error {
			if path == "/api/states/fan.living_room" {
				if s, ok := out.(*haclient.EntityState); ok {
					s.EntityID = "fan.living_room"
					s.State = "on"
				}
				return nil
			}
			if path == "/api/services/fan/turn_on" {
				serviceCalls++
			}
			return nil
		},
	}
	adapter := NewAdapter(mock, Config{})

	step := &contracts.ActionStep{
		StepID:       "step-6",
		ActionType:   "ha.control",
		TargetID:     "fan.living_room",
		Parameters:   map[string]any{"command": "turn_on"},
		IsIdempotent: true,
	}

	result, err := adapter.Execute(context.Background(), step)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.Status != contracts.ActionStatusSucceeded {
		t.Errorf("Expected status succeeded, got %s", result.Status)
	}
	if serviceCalls != 0 {
		t.Errorf("Expected 0 service calls due to idempotency, got %d", serviceCalls)
	}
}
