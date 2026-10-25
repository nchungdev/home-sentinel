package homeassistant

import (
	"fmt"

	"github.com/nchungdev/home-sentinel/agent/pkg/contracts"
	"github.com/nchungdev/home-sentinel/agent/pkg/haclient"
)

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
