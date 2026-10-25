package automation

import (
	"fmt"
	"strings"
	"time"
)

func (m *Manager) EvaluateCustomRules(triggerType string) {
	rules := m.store.Get().CustomRules
	if len(rules) == 0 {
		return
	}

	m.activityMutex.Lock()
	idleTime := time.Since(m.lastActivityTime)
	m.activityMutex.Unlock()

	for _, rule := range rules {
		if !rule.Enabled || rule.TriggerType != triggerType {
			continue
		}
		if !isWithinRuleHours(rule.StartHour, rule.EndHour) {
			continue
		}
		if rule.CheckTVOff && m.isTVScreenOn() {
			continue
		}
		if triggerType == "idle_room" {
			minWait := rule.IdleMinutes
			if minWait <= 0 {
				minWait = 5
			}
			if idleTime < time.Duration(minWait)*time.Minute {
				continue
			}
		}
		m.executeCustomRule(rule)
	}
}

func isWithinRuleHours(startHour, endHour int) bool {
	if startHour == endHour {
		return true
	}
	hour := time.Now().Hour()
	if startHour > endHour {
		return hour >= startHour || hour < endHour
	}
	return hour >= startHour && hour < endHour
}

func (m *Manager) executeCustomRule(rule CustomRule) {
	serviceAction := "turn_off"
	if rule.ActionType == "turn_on" {
		serviceAction = "turn_on"
	}

	affected := 0
	for _, entityID := range rule.TargetEntities {
		if entityID == "" || m.haClient == nil || m.haClient.BaseURL == "" {
			continue
		}
		isOn := m.isEntityOn(entityID)
		if serviceAction == "turn_off" && !isOn {
			continue
		}
		if serviceAction == "turn_on" && isOn {
			continue
		}
		domain := "light"
		if parts := strings.SplitN(entityID, ".", 2); len(parts) == 2 && parts[0] != "" {
			domain = parts[0]
		}
		endpoint := fmt.Sprintf("/api/services/%s/%s", domain, serviceAction)
		payload := map[string]interface{}{"entity_id": entityID}
		if err := m.haClient.RequestJSON(endpoint, payload, nil); err == nil {
			affected++
		}
	}

	if rule.CameraPreset != "" {
		m.pointCameraToPreset(rule.CameraPreset)
	}

	if affected > 0 {
		tNow := time.Now().Format("15:04:05")
		msg := fmt.Sprintf("[%s] ⚡ Kịch bản %q: Đã %s %d thiết bị", tNow, rule.Name, serviceAction, affected)
		if m.broadcast != nil {
			m.broadcast(msg)
		}
	}
}
