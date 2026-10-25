package automation

import (
	"fmt"
	"strings"
	"time"

	"github.com/nchungdev/home-sentinel/agent/pkg/camstream"
)

func (m *Manager) HandleEveningArrival(triggerReason string) {
	rule := m.store.Get().EveningArrival
	if !rule.Enabled || !m.isEveningTime(rule) {
		return
	}

	cooldown := time.Duration(rule.CooldownSec) * time.Second
	if cooldown <= 0 {
		cooldown = 5 * time.Minute
	}

	m.activityMutex.Lock()
	if time.Since(m.lastDoorTrigger) < cooldown {
		m.activityMutex.Unlock()
		return
	}

	targets := rule.TargetEntities
	if len(targets) == 0 && rule.TargetLight != "" {
		targets = []string{rule.TargetLight}
	}

	allOn := len(targets) > 0
	for _, eid := range targets {
		if !m.isEntityOn(eid) {
			allOn = false
			break
		}
	}
	if allOn {
		m.activityMutex.Unlock()
		return
	}

	m.lastDoorTrigger = time.Now()
	m.lastActivityTime = time.Now()
	m.activityMutex.Unlock()

	tNow := time.Now().Format("15:04:05")
	if m.haClient != nil && m.haClient.BaseURL != "" && len(targets) > 0 {
		for _, eid := range targets {
			payload := map[string]interface{}{"entity_id": eid}
			_ = m.haClient.RequestJSON("/api/services/light/turn_on", payload, nil)
		}
		msg := fmt.Sprintf("[%s] 💡 [Buổi tối] Nhận diện %s: Đã bật %d đèn đón!", tNow, triggerReason, len(targets))
		if m.broadcast != nil {
			m.broadcast(msg)
		}
		fmt.Println(msg)
	}

	if rule.TargetPreset != "" {
		m.pointCameraToPreset(rule.TargetPreset)
	}
}

func (m *Manager) MatchesDoorSound(text string) bool {
	rule := m.store.Get().EveningArrival
	if !rule.Enabled {
		return false
	}
	lower := strings.ToLower(text)
	for _, kw := range rule.SoundKeywords {
		if strings.Contains(lower, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

func (m *Manager) isEveningTime(rule EveningArrivalRule) bool {
	hour := time.Now().Hour()
	if rule.StartHour > rule.EndHour {
		if hour >= rule.StartHour || hour < rule.EndHour {
			return true
		}
	} else if hour >= rule.StartHour && hour < rule.EndHour {
		return true
	}

	if m.haClient != nil && m.haClient.BaseURL != "" {
		var sunState struct {
			State string `json:"state"`
		}
		if err := m.haClient.RequestJSON("/api/states/sun.sun", nil, &sunState); err == nil {
			return sunState.State == "below_horizon"
		}
	}
	return false
}

func (m *Manager) isEntityOn(entityID string) bool {
	if m.haClient == nil || m.haClient.BaseURL == "" || entityID == "" {
		return false
	}
	var state struct {
		State string `json:"state"`
	}
	if err := m.haClient.RequestJSON("/api/states/"+entityID, nil, &state); err == nil {
		return state.State == "on"
	}
	return false
}

func (m *Manager) pointCameraToPreset(presetToken string) {
	if m.getCamera == nil || presetToken == "" {
		return
	}
	cfg := m.getCamera()
	pwd := cfg.CloudPassword
	if pwd == "" {
		pwd = cfg.Password
	}
	if cfg.IP != "" && pwd != "" {
		session := camstream.GetTapoSession(cfg.IP, cfg.Username, pwd)
		_ = session.GotoPreset(presetToken)
	}
}
