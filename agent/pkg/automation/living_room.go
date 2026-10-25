package automation

import (
	"fmt"
	"time"
)

func (m *Manager) runInactivityLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.checkLivingRoomInactivity()
			m.EvaluateCustomRules("idle_room")
		}
	}
}

func (m *Manager) checkLivingRoomInactivity() {
	rule := m.store.Get().IdleLivingRoom
	if !rule.Enabled {
		return
	}

	idleLimit := time.Duration(rule.IdleMinutes) * time.Minute
	if idleLimit <= 0 {
		idleLimit = 15 * time.Minute
	}

	m.activityMutex.Lock()
	idleTime := time.Since(m.lastActivityTime)
	m.activityMutex.Unlock()

	if idleTime < idleLimit {
		return
	}

	activeLights := m.getActiveLivingRoomLights(rule.TargetLights)
	if len(activeLights) == 0 {
		return
	}

	if rule.CheckTVPlayback && m.isTVScreenOn() {
		return
	}

	tNow := time.Now().Format("15:04:05")
	m.turnOffLivingRoom(rule, activeLights, idleTime, tNow)
}

func (m *Manager) getActiveLivingRoomLights(targets []string) []string {
	var onLights []string
	for _, id := range targets {
		if m.isEntityOn(id) {
			onLights = append(onLights, id)
		}
	}
	return onLights
}

func (m *Manager) turnOffLivingRoom(rule IdleLivingRoomRule, lights []string, idle time.Duration, tNow string) {
	for _, id := range lights {
		payload := map[string]interface{}{"entity_id": id}
		_ = m.haClient.RequestJSON("/api/services/light/turn_off", payload, nil)
	}

	if rule.TurnOffScript != "" {
		_ = m.haClient.RequestJSON("/api/services/script/turn_on", map[string]interface{}{
			"entity_id": rule.TurnOffScript,
		}, nil)
	}

	idleMin := int(idle.Minutes())
	msg := fmt.Sprintf("[%s] 🌙 Phòng khách vắng người (>%d phút): Tự động tắt %d đèn phòng khách để tiết kiệm điện", tNow, idleMin, len(lights))
	if m.broadcast != nil {
		m.broadcast(msg)
	}
	fmt.Println(msg)

	m.activityMutex.Lock()
	m.lastActivityTime = time.Now()
	m.roomOccupied = false
	m.activityMutex.Unlock()
}

func (m *Manager) isTVScreenOn() bool {
	return IsTVScreenOn(m.tvHost)
}
