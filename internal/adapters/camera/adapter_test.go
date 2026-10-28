package camera

import (
	"context"
	"testing"

	"github.com/nchungdev/home-sentinel/internal/contracts"
)

type mockCameraRequester struct {
	lastPath string
	lastBody map[string]any
	respOK   bool
	respMsg  string
}

func (m *mockCameraRequester) RequestJSON(path string, body any, out any) error {
	m.lastPath = path
	if b, ok := body.(map[string]any); ok {
		m.lastBody = b
	}
	if resp, ok := out.(*CameraResponse); ok {
		resp.OK = m.respOK
		resp.Message = m.respMsg
	}
	return nil
}

func TestMissingCameraURL(t *testing.T) {
	mock := &mockCameraRequester{}
	adapter := NewAdapter(mock, Config{})

	step := &contracts.ActionStep{
		StepID:     "cam-1",
		ActionType: "camera.status",
	}

	result, err := adapter.Execute(context.Background(), step)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.Status != contracts.ActionStatusFailed {
		t.Errorf("Expected status failed, got %s", result.Status)
	}
	if result.ErrorCode == nil || *result.ErrorCode != "CONFIG_MISSING_CAMERA_URL" {
		t.Errorf("Expected CONFIG_MISSING_CAMERA_URL, got %v", result.ErrorCode)
	}
}

func TestMissingCameraPassword(t *testing.T) {
	mock := &mockCameraRequester{}
	adapter := NewAdapter(mock, Config{
		BaseURL: "http://127.0.0.1:8792",
	})

	step := &contracts.ActionStep{
		StepID:     "cam-2",
		ActionType: "camera.ptz",
		Parameters: map[string]any{"direction": "left"},
	}

	result, err := adapter.Execute(context.Background(), step)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.Status != contracts.ActionStatusFailed {
		t.Errorf("Expected status failed, got %s", result.Status)
	}
	if result.ErrorCode == nil || *result.ErrorCode != "CAMERA_AUTH_MISSING" {
		t.Errorf("Expected CAMERA_AUTH_MISSING, got %v", result.ErrorCode)
	}
}

func TestPTZExecutionAndClamping(t *testing.T) {
	mock := &mockCameraRequester{
		respOK:  true,
		respMsg: "Moved left",
	}
	adapter := NewAdapter(mock, Config{
		BaseURL:  "http://127.0.0.1:8792",
		Password: "secret-password",
	})

	step := &contracts.ActionStep{
		StepID:     "cam-3",
		ActionType: "camera.ptz",
		Parameters: map[string]any{"direction": "left", "step": 90},
	}

	result, err := adapter.Execute(context.Background(), step)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.Status != contracts.ActionStatusSucceeded {
		t.Errorf("Expected status succeeded, got %s", result.Status)
	}
	if mock.lastPath != "/api/camera/ptz" {
		t.Errorf("Expected path /api/camera/ptz, got %s", mock.lastPath)
	}
	if mock.lastBody["step"] != 45 {
		t.Errorf("Expected step clamped to 45, got %v", mock.lastBody["step"])
	}
	if mock.lastBody["direction"] != "left" {
		t.Errorf("Expected direction left, got %v", mock.lastBody["direction"])
	}
}

func TestPTZInvalidDirection(t *testing.T) {
	mock := &mockCameraRequester{}
	adapter := NewAdapter(mock, Config{
		BaseURL:  "http://127.0.0.1:8792",
		Password: "secret-password",
	})

	step := &contracts.ActionStep{
		StepID:     "cam-4",
		ActionType: "camera.ptz",
		Parameters: map[string]any{"direction": "diagonal"},
	}

	result, err := adapter.Execute(context.Background(), step)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.Status != contracts.ActionStatusFailed {
		t.Errorf("Expected status failed, got %s", result.Status)
	}
	if result.ErrorCode == nil || *result.ErrorCode != "INVALID_PTZ_DIRECTION" {
		t.Errorf("Expected INVALID_PTZ_DIRECTION, got %v", result.ErrorCode)
	}
}

func TestFalseSuccessIsRejected(t *testing.T) {
	mock := &mockCameraRequester{
		respOK:  false,
		respMsg: "Device busy",
	}
	adapter := NewAdapter(mock, Config{
		BaseURL:  "http://127.0.0.1:8792",
		Password: "secret-password",
	})

	step := &contracts.ActionStep{
		StepID:     "cam-5",
		ActionType: "camera.privacy",
		Parameters: map[string]any{"enabled": true},
	}

	result, err := adapter.Execute(context.Background(), step)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.Status != contracts.ActionStatusFailed {
		t.Errorf("Expected status failed, got %s", result.Status)
	}
	if result.ErrorCode == nil || *result.ErrorCode != "CAMERA_REJECTED" {
		t.Errorf("Expected CAMERA_REJECTED, got %v", result.ErrorCode)
	}
}

func TestCameraSpeak(t *testing.T) {
	mock := &mockCameraRequester{
		respOK:  true,
		respMsg: "Audio delivered",
	}
	adapter := NewAdapter(mock, Config{
		BaseURL:  "http://127.0.0.1:8792",
		Password: "secret-password",
	})

	step := &contracts.ActionStep{
		StepID:     "cam-6",
		ActionType: "camera.speak",
		Parameters: map[string]any{"text": "Chào bạn, có người ở cửa."},
	}

	result, err := adapter.Execute(context.Background(), step)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.Status != contracts.ActionStatusSucceeded {
		t.Errorf("Expected status succeeded, got %s", result.Status)
	}
	if mock.lastPath != "/api/camera/speak" {
		t.Errorf("Expected path /api/camera/speak, got %s", mock.lastPath)
	}
	if mock.lastBody["text"] != "Chào bạn, có người ở cửa." {
		t.Errorf("Unexpected text: %v", mock.lastBody["text"])
	}
}
