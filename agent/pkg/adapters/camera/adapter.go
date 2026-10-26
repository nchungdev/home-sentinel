package camera

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/nchungdev/home-sentinel/agent/pkg/contracts"
)

type Requester interface {
	RequestJSON(path string, body any, out any) error
}

type Config struct {
	BaseURL        string
	IP             string
	User           string
	Password       string
	DefaultPTZStep int
	MaxSpeakLength int
}

type Adapter struct {
	requester Requester
	config    Config
}

type CameraResponse struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

func NewAdapter(requester Requester, cfg Config) *Adapter {
	if cfg.BaseURL == "" {
		cfg.BaseURL = os.Getenv("AGENT_CAMERA_URL")
	}
	if cfg.IP == "" {
		cfg.IP = os.Getenv("AGENT_CAMERA_IP")
		if cfg.IP == "" {
			cfg.IP = "192.168.1.100"
		}
	}
	if cfg.User == "" {
		cfg.User = os.Getenv("AGENT_CAMERA_USER")
		if cfg.User == "" {
			cfg.User = "admin"
		}
	}
	if cfg.Password == "" {
		cfg.Password = os.Getenv("AGENT_CAMERA_PASSWORD")
	}
	if cfg.DefaultPTZStep <= 0 {
		cfg.DefaultPTZStep = 10
	}
	if cfg.MaxSpeakLength <= 0 {
		cfg.MaxSpeakLength = 500
	}
	return &Adapter{
		requester: requester,
		config:    cfg,
	}
}

func (a *Adapter) Execute(
	ctx context.Context,
	step *contracts.ActionStep,
) (*contracts.ExecutionResult, error) {
	if step == nil {
		return nil, contracts.ErrInvalidActionStep
	}

	if a.config.BaseURL == "" {
		code := "CONFIG_MISSING_CAMERA_URL"
		detail := "Chưa cấu hình Camera service cho Agent."
		return &contracts.ExecutionResult{
			StepID:    step.StepID,
			Status:    contracts.ActionStatusFailed,
			ErrorCode: &code,
			Detail:    &detail,
			CanRetry:  false,
		}, nil
	}

	actionType := strings.TrimSpace(step.ActionType)
	switch actionType {
	case "camera.status":
		return a.executeStatus(step)
	case "camera.ptz":
		return a.executePTZ(step)
	case "camera.privacy":
		return a.executePrivacy(step)
	case "camera.speak":
		return a.executeSpeak(step)
	default:
		code := "UNSUPPORTED_CAMERA_ACTION"
		detail := fmt.Sprintf("Unsupported camera action: %s", actionType)
		return &contracts.ExecutionResult{
			StepID:    step.StepID,
			Status:    contracts.ActionStatusFailed,
			ErrorCode: &code,
			Detail:    &detail,
			CanRetry:  false,
		}, nil
	}
}

func (a *Adapter) baseAuthBody() (map[string]any, error) {
	if a.config.Password == "" {
		return nil, fmt.Errorf("Chưa cấu hình mật khẩu Camera trên máy chủ Agent.")
	}
	return map[string]any{
		"ip":       a.config.IP,
		"user":     a.config.User,
		"password": a.config.Password,
	}, nil
}

func (a *Adapter) executeStatus(step *contracts.ActionStep) (*contracts.ExecutionResult, error) {
	var resp map[string]any
	err := a.requester.RequestJSON("/api/security/status", nil, &resp)
	if err != nil {
		code := "CAMERA_STATUS_FAILED"
		detail := fmt.Sprintf("Failed to get camera status: %v", err)
		return &contracts.ExecutionResult{
			StepID:    step.StepID,
			Status:    contracts.ActionStatusFailed,
			ErrorCode: &code,
			Detail:    &detail,
			CanRetry:  true,
		}, nil
	}

	detail := "Camera service đang hoạt động."
	return &contracts.ExecutionResult{
		StepID:      step.StepID,
		Status:      contracts.ActionStatusSucceeded,
		Detail:      &detail,
		ActualState: resp,
		CanRetry:    false,
	}, nil
}

func (a *Adapter) executePTZ(step *contracts.ActionStep) (*contracts.ExecutionResult, error) {
	body, authErr := a.baseAuthBody()
	if authErr != nil {
		code := "CAMERA_AUTH_MISSING"
		detail := authErr.Error()
		return &contracts.ExecutionResult{
			StepID:    step.StepID,
			Status:    contracts.ActionStatusFailed,
			ErrorCode: &code,
			Detail:    &detail,
			CanRetry:  false,
		}, nil
	}

	rawDir, _ := step.Parameters["direction"].(string)
	dir := strings.ToLower(strings.TrimSpace(rawDir))
	validDirs := map[string]bool{
		"left": true, "right": true, "up": true, "down": true,
	}
	if !validDirs[dir] {
		code := "INVALID_PTZ_DIRECTION"
		detail := fmt.Sprintf("Invalid PTZ direction: %s", rawDir)
		return &contracts.ExecutionResult{
			StepID:    step.StepID,
			Status:    contracts.ActionStatusFailed,
			ErrorCode: &code,
			Detail:    &detail,
			CanRetry:  false,
		}, nil
	}

	stepVal := a.config.DefaultPTZStep
	if rawStep, ok := step.Parameters["step"].(float64); ok && rawStep > 0 {
		stepVal = int(rawStep)
	} else if rawStepInt, ok := step.Parameters["step"].(int); ok && rawStepInt > 0 {
		stepVal = rawStepInt
	}
	if stepVal > 45 {
		stepVal = 45
	}

	body["direction"] = dir
	body["step"] = stepVal

	return a.sendCameraCommand(step, "/api/camera/ptz", body)
}

func (a *Adapter) executePrivacy(step *contracts.ActionStep) (*contracts.ExecutionResult, error) {
	body, authErr := a.baseAuthBody()
	if authErr != nil {
		code := "CAMERA_AUTH_MISSING"
		detail := authErr.Error()
		return &contracts.ExecutionResult{
			StepID:    step.StepID,
			Status:    contracts.ActionStatusFailed,
			ErrorCode: &code,
			Detail:    &detail,
			CanRetry:  false,
		}, nil
	}

	enabled, ok := step.Parameters["enabled"].(bool)
	if !ok {
		cmd, _ := step.Parameters["command"].(string)
		enabled = cmd == "on" || cmd == "bat" || cmd == "enable"
	}
	body["enabled"] = enabled

	return a.sendCameraCommand(step, "/api/camera/privacy", body)
}

func (a *Adapter) executeSpeak(step *contracts.ActionStep) (*contracts.ExecutionResult, error) {
	body, authErr := a.baseAuthBody()
	if authErr != nil {
		code := "CAMERA_AUTH_MISSING"
		detail := authErr.Error()
		return &contracts.ExecutionResult{
			StepID:    step.StepID,
			Status:    contracts.ActionStatusFailed,
			ErrorCode: &code,
			Detail:    &detail,
			CanRetry:  false,
		}, nil
	}

	text, _ := step.Parameters["text"].(string)
	text = strings.TrimSpace(text)
	if text == "" {
		code := "EMPTY_SPEECH_TEXT"
		detail := "Nội dung phát loa camera không được để trống."
		return &contracts.ExecutionResult{
			StepID:    step.StepID,
			Status:    contracts.ActionStatusFailed,
			ErrorCode: &code,
			Detail:    &detail,
			CanRetry:  false,
		}, nil
	}

	if len(text) > a.config.MaxSpeakLength {
		text = text[:a.config.MaxSpeakLength]
	}

	body["text"] = text
	body["speed"] = 1.0

	return a.sendCameraCommand(step, "/api/camera/speak", body)
}

func (a *Adapter) sendCameraCommand(
	step *contracts.ActionStep,
	path string,
	body map[string]any,
) (*contracts.ExecutionResult, error) {
	var resp CameraResponse
	err := a.requester.RequestJSON(path, body, &resp)
	if err != nil {
		code := "CAMERA_NETWORK_ERROR"
		detail := fmt.Sprintf("Lỗi kết nối tới camera API: %v", err)
		return &contracts.ExecutionResult{
			StepID:    step.StepID,
			Status:    contracts.ActionStatusFailed,
			ErrorCode: &code,
			Detail:    &detail,
			CanRetry:  true,
		}, nil
	}

	if !resp.OK {
		code := "CAMERA_REJECTED"
		detail := "Camera từ chối hoặc không thực hiện được lệnh."
		return &contracts.ExecutionResult{
			StepID:    step.StepID,
			Status:    contracts.ActionStatusFailed,
			ErrorCode: &code,
			Detail:    &detail,
			CanRetry:  false,
		}, nil
	}

	detail := resp.Message
	if detail == "" {
		detail = "Camera API đã nhận lệnh."
	}
	return &contracts.ExecutionResult{
		StepID:   step.StepID,
		Status:   contracts.ActionStatusSucceeded,
		Detail:   &detail,
		ActualState: map[string]any{
			"ok":      true,
			"message": detail,
		},
		CanRetry: false,
	}, nil
}
