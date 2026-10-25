package automation

import (
	"fmt"
	"time"

	"github.com/nchungdev/home-sentinel/agent/pkg/camstream"
)

const frontDoorLight = "light.dqsmart_020119140c_1"

func (m *Manager) HandleEveningArrival(triggerReason string) {
	if !m.isEveningTime() {
		return
	}

	m.activityMutex.Lock()
	if time.Since(m.lastDoorTrigger) < 5*time.Minute {
		m.activityMutex.Unlock()
		return
	}

	if m.isEntityOn(frontDoorLight) {
		m.activityMutex.Unlock()
		return
	}

	m.lastDoorTrigger = time.Now()
	m.lastActivityTime = time.Now()
	m.activityMutex.Unlock()

	tNow := time.Now().Format("15:04:05")
	if m.haClient != nil && m.haClient.BaseURL != "" {
		payload := map[string]interface{}{"entity_id": frontDoorLight}
		err := m.haClient.RequestJSON("/api/services/light/turn_on", payload, nil)
		if err == nil {
			msg := fmt.Sprintf("[%s] 💡 [Buổi tối] Nhận diện %s: Đã bật Đèn trần cửa chính!", tNow, triggerReason)
			if m.broadcast != nil {
				m.broadcast(msg)
			}
			fmt.Println(msg)
		}
	}

	m.pointCameraToDoor()
}

func (m *Manager) isEveningTime() bool {
	hour := time.Now().Hour()
	if hour >= 18 || hour < 6 {
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
	if m.haClient == nil || m.haClient.BaseURL == "" {
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

func (m *Manager) pointCameraToDoor() {
	if m.getCamera == nil {
		return
	}
	cfg := m.getCamera()
	pwd := cfg.CloudPassword
	if pwd == "" {
		pwd = cfg.Password
	}
	if cfg.IP != "" && pwd != "" {
		session := camstream.GetTapoSession(cfg.IP, cfg.Username, pwd)
		_ = session.GotoPreset("3")
	}
}
