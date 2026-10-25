package homeassistant

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/nchungdev/home-sentinel/agent/pkg/contracts"
	"github.com/nchungdev/home-sentinel/agent/pkg/haclient"
)

type Requester interface {
	RequestJSON(path string, body any, out any) error
}

type Config struct {
	BaseURL           string
	Token             string
	ProtectedEntities []string
}

type Adapter struct {
	requester Requester
	config    Config
}

func NewAdapter(requester Requester, cfg Config) *Adapter {
	if len(cfg.ProtectedEntities) == 0 {
		envProtected := os.Getenv("AGENT_HA_PROTECTED_ENTITIES")
		if envProtected != "" {
			for _, item := range strings.Split(envProtected, ",") {
				trimmed := strings.TrimSpace(item)
				if trimmed != "" {
					cfg.ProtectedEntities = append(cfg.ProtectedEntities, trimmed)
				}
			}
		}
	}
	return &Adapter{
		requester: requester,
		config:    cfg,
	}
}

func (a *Adapter) isProtected(entityID string) bool {
	for _, protected := range a.config.ProtectedEntities {
		if protected == entityID {
			return true
		}
	}
	return false
}

func (a *Adapter) Execute(
	ctx context.Context,
	step *contracts.ActionStep,
) (*contracts.ExecutionResult, error) {
	if step == nil {
		return nil, contracts.ErrInvalidActionStep
	}

	entityID := strings.TrimSpace(step.TargetID)
	if entityID == "" {
		code := "MISSING_TARGET_ENTITY"
		detail := "Target entity ID is missing in action step"
		return &contracts.ExecutionResult{
			StepID:    step.StepID,
			Status:    contracts.ActionStatusFailed,
			ErrorCode: &code,
			Detail:    &detail,
			CanRetry:  false,
		}, nil
	}

	parts := strings.Split(entityID, ".")
	if len(parts) != 2 {
		code := "INVALID_ENTITY_FORMAT"
		detail := fmt.Sprintf("Invalid entity format: %s", entityID)
		return &contracts.ExecutionResult{
			StepID:    step.StepID,
			Status:    contracts.ActionStatusFailed,
			ErrorCode: &code,
			Detail:    &detail,
			CanRetry:  false,
		}, nil
	}

	domain := parts[0]
	supportedDomains := map[string]bool{
		"light":   true,
		"switch":  true,
		"fan":     true,
		"climate": true,
		"cover":   true,
	}
	if !supportedDomains[domain] {
		code := "UNSUPPORTED_DOMAIN"
		detail := fmt.Sprintf("Domain %s is not supported by HomeAssistantAdapter", domain)
		return &contracts.ExecutionResult{
			StepID:    step.StepID,
			Status:    contracts.ActionStatusFailed,
			ErrorCode: &code,
			Detail:    &detail,
			CanRetry:  false,
		}, nil
	}

	if a.isProtected(entityID) {
		code := "PROTECTED_ENTITY"
		detail := "Thiết bị nguồn trọng yếu được bảo vệ, Agent không điều khiển mục này."
		return &contracts.ExecutionResult{
			StepID:    step.StepID,
			Status:    contracts.ActionStatusFailed,
			ErrorCode: &code,
			Detail:    &detail,
			CanRetry:  false,
		}, nil
	}

	var currentState haclient.EntityState
	err := a.requester.RequestJSON("/api/states/"+entityID, nil, &currentState)
	if err != nil {
		code := "STATE_FETCH_FAILED"
		detail := fmt.Sprintf("Failed to query state for %s: %v", entityID, err)
		return &contracts.ExecutionResult{
			StepID:    step.StepID,
			Status:    contracts.ActionStatusFailed,
			ErrorCode: &code,
			Detail:    &detail,
			CanRetry:  true,
		}, nil
	}

	if currentState.State == "unavailable" || currentState.State == "unknown" {
		code := "ENTITY_UNAVAILABLE"
		detail := "Thiết bị hiện không khả dụng, chưa gửi lệnh."
		return &contracts.ExecutionResult{
			StepID:    step.StepID,
			Status:    contracts.ActionStatusFailed,
			ErrorCode: &code,
			Detail:    &detail,
			CanRetry:  true,
		}, nil
	}

	service, serviceData, prepErr := a.prepareServiceCall(domain, step, currentState)
	if prepErr != nil {
		code := "SERVICE_PREPARATION_FAILED"
		detail := prepErr.Error()
		return &contracts.ExecutionResult{
			StepID:    step.StepID,
			Status:    contracts.ActionStatusFailed,
			ErrorCode: &code,
			Detail:    &detail,
			CanRetry:  false,
		}, nil
	}

	if step.IsIdempotent && a.isAlreadyInState(currentState, domain, service, serviceData) {
		detail := fmt.Sprintf("Thiết bị %s đã ở trạng thái yêu cầu.", entityID)
		return &contracts.ExecutionResult{
			StepID:   step.StepID,
			Status:   contracts.ActionStatusSucceeded,
			Detail:   &detail,
			ActualState: map[string]any{
				"state":      currentState.State,
				"attributes": currentState.Attributes,
			},
			CanRetry: false,
		}, nil
	}

	callPath := fmt.Sprintf("/api/services/%s/%s", domain, service)
	if err := a.requester.RequestJSON(callPath, serviceData, nil); err != nil {
		code := "SERVICE_CALL_FAILED"
		detail := fmt.Sprintf("Failed to call service %s: %v", callPath, err)
		return &contracts.ExecutionResult{
			StepID:    step.StepID,
			Status:    contracts.ActionStatusFailed,
			ErrorCode: &code,
			Detail:    &detail,
			CanRetry:  true,
		}, nil
	}

	var updatedState haclient.EntityState
	_ = a.requester.RequestJSON("/api/states/"+entityID, nil, &updatedState)

	name := entityID
	if fn, ok := currentState.Attributes["friendly_name"].(string); ok && fn != "" {
		name = fn
	}

	readbackDetail := fmt.Sprintf(
		"HA đã nhận lệnh cho %s. Trạng thái đọc lại: %s.",
		name,
		updatedState.State,
	)
	return &contracts.ExecutionResult{
		StepID: step.StepID,
		Status: contracts.ActionStatusSucceeded,
		Detail: &readbackDetail,
		ActualState: map[string]any{
			"state":      updatedState.State,
			"attributes": updatedState.Attributes,
		},
		CanRetry: false,
	}, nil
}

func (a *Adapter) prepareServiceCall(
	domain string,
	step *contracts.ActionStep,
	state haclient.EntityState,
) (string, map[string]any, error) {
	serviceData := map[string]any{
		"entity_id": step.TargetID,
	}

	val, hasVal := step.Parameters["temperature"]
	if hasVal {
		if domain != "climate" {
			return "", nil, fmt.Errorf("Chỉ điều hòa hỗ trợ lệnh đặt nhiệt độ này.")
		}
		var tempFloat float64
		switch t := val.(type) {
		case float64:
			tempFloat = t
		case int:
			tempFloat = float64(t)
		default:
			return "", nil, fmt.Errorf("Tham số nhiệt độ không hợp lệ")
		}

		minTemp := 7.0
		maxTemp := 35.0
		if minVal, ok := state.Attributes["min_temp"].(float64); ok {
			minTemp = minVal
		}
		if maxVal, ok := state.Attributes["max_temp"].(float64); ok {
			maxTemp = maxVal
		}

		if tempFloat < minTemp || tempFloat > maxTemp {
			return "", nil, fmt.Errorf("Nhiệt độ nằm ngoài giới hạn thiết bị.")
		}

		serviceData["temperature"] = tempFloat
		return "set_temperature", serviceData, nil
	}

	verb, _ := step.Parameters["command"].(string)
	if verb == "" {
		verb = step.ActionType
	}
	normVerb := haclient.Normalize(verb)

	if domain == "cover" {
		switch normVerb {
		case "open", "mo", "bat", "cover.open":
			return "open_cover", serviceData, nil
		case "close", "dong", "tat", "cover.close":
			return "close_cover", serviceData, nil
		case "stop", "dung", "cover.stop":
			return "stop_cover", serviceData, nil
		default:
			return "", nil, fmt.Errorf("Lệnh rèm không được hỗ trợ: %s", verb)
		}
	}

	switch normVerb {
	case "turn_off", "tat", "dong", "off":
		return "turn_off", serviceData, nil
	case "turn_on", "bat", "mo", "on":
		return "turn_on", serviceData, nil
	default:
		return "", nil, fmt.Errorf("Lệnh này không phù hợp với thiết bị: %s", verb)
	}
}

func (a *Adapter) isAlreadyInState(
	state haclient.EntityState,
	domain string,
	service string,
	serviceData map[string]any,
) bool {
	if domain == "climate" && service == "set_temperature" {
		targetTemp, ok := serviceData["temperature"].(float64)
		if !ok {
			return false
		}
		currentTemp, ok := state.Attributes["temperature"].(float64)
		return ok && currentTemp == targetTemp
	}
	if service == "turn_on" && state.State == "on" {
		return true
	}
	if service == "turn_off" && state.State == "off" {
		return true
	}
	if service == "open_cover" && state.State == "open" {
		return true
	}
	if service == "close_cover" && state.State == "closed" {
		return true
	}
	return false
}
