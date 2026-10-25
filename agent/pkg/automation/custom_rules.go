package automation

import (
	"fmt"
	"strings"
	"time"
)

func (m *Manager) EvaluateCustomRules(triggerType string) {
	cfg := m.store.Get()
	m.activityMutex.Lock()
	idleTime := time.Since(m.lastActivityTime)
	m.activityMutex.Unlock()

	for _, rule := range cfg.Rules {
		if !rule.Enabled {
			continue
		}
		if rule.TriggerType != triggerType && !(triggerType == "clap_detected" && rule.TriggerType == "clap_sound") {
			continue
		}
		if rule.TimeCondition == "range" && !isWithinRuleHours(rule.StartHour, rule.EndHour) {
			continue
		}
		if rule.TVCondition == "off" && m.isTVScreenOn() {
			continue
		}
		if rule.TVCondition == "on" && !m.isTVScreenOn() {
			continue
		}
		if triggerType == "idle_room" {
			minWait := rule.IdleMinutes
			if minWait <= 0 {
				minWait = 1
			}
			if idleTime < time.Duration(minWait)*time.Minute {
				continue
			}
		}
		m.executeRule(rule, false)
	}
}

func (m *Manager) executeRule(rule Rule, force bool) int {
	if rule.TargetTV != "" && rule.ActionType == "turn_on" {
		go func(host string) { _ = TurnOnTV(host) }(rule.TargetTV)
	}
	return m.executeEntitiesAction(
		rule.Name,
		rule.ActionType,
		rule.TargetEntities,
		rule.ScriptID,
		rule.CameraPreset,
		force,
	)
}

func (m *Manager) RunRuleNow(ruleID string) int {
	cfg := m.store.Get()
	cleanID := strings.TrimPrefix(ruleID, "custom-")
	for _, r := range cfg.Rules {
		if r.ID == ruleID || r.ID == cleanID {
			return m.executeRule(r, true)
		}
	}
	switch ruleID {
	case "idle":
		return m.executeEntitiesAction(
			"Kịch bản 1 (Chạy thử)",
			"turn_off",
			cfg.IdleLivingRoom.TargetLights,
			cfg.IdleLivingRoom.TurnOffScript,
			"",
			true,
		)
	case "clap":
		targets := cfg.SofaClapAction.TargetEntities
		if len(targets) == 0 && cfg.SofaClapAction.TargetFan != "" {
			targets = []string{cfg.SofaClapAction.TargetFan}
		}
		if cfg.SofaClapAction.TargetTV != "" {
			go func(host string) { _ = TurnOnTV(host) }(cfg.SofaClapAction.TargetTV)
		}
		return m.executeEntitiesAction("Kịch bản 2 (Chạy thử)", "turn_on", targets, "", "1", true)
	case "evening":
		targets := cfg.EveningArrival.TargetEntities
		if len(targets) == 0 && cfg.EveningArrival.TargetLight != "" {
			targets = []string{cfg.EveningArrival.TargetLight}
		}
		return m.executeEntitiesAction(
			"Kịch bản 3 (Chạy thử)",
			"turn_on",
			targets,
			"",
			cfg.EveningArrival.TargetPreset,
			true,
		)
	default:
		for _, r := range cfg.CustomRules {
			if r.ID == cleanID {
				return m.executeCustomRule(r, true)
			}
		}
	}
	return 0
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

func (m *Manager) executeCustomRule(rule CustomRule, force bool) int {
	return m.executeEntitiesAction(
		rule.Name,
		rule.ActionType,
		rule.TargetEntities,
		"",
		rule.CameraPreset,
		force,
	)
}

func (m *Manager) executeEntitiesAction(
	ruleName, actionType string,
	entities []string,
	scriptID, cameraPreset string,
	force bool,
) int {
	serviceAction := "turn_off"
	if actionType == "turn_on" {
		serviceAction = "turn_on"
	}

	affected := 0
	for _, entityID := range entities {
		if entityID == "" || m.haClient == nil || m.haClient.BaseURL == "" {
			continue
		}
		if !force {
			isOn := m.isEntityOn(entityID)
			if serviceAction == "turn_off" && !isOn {
				continue
			}
			if serviceAction == "turn_on" && isOn {
				continue
			}
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

	if scriptID != "" && m.haClient != nil && m.haClient.BaseURL != "" {
		_ = m.haClient.RequestJSON("/api/services/script/turn_on", map[string]interface{}{
			"entity_id": scriptID,
		}, nil)
	}

	if cameraPreset != "" {
		m.pointCameraToPreset(cameraPreset)
	}

	if affected > 0 || force {
		tNow := time.Now().Format("15:04:05")
		msg := fmt.Sprintf("[%s] ⚡ %s: Đã %s %d thiết bị", tNow, ruleName, serviceAction, affected)
		if m.broadcast != nil {
			m.broadcast(msg)
		}
	}
	return affected
}
