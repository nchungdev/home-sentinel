package automation

import (
	"fmt"
	"time"
)

var livingRoomLights = []string{
	"light.dqsmart_0201bff7b7_1", // Đèn trần phòng khách
	"light.dqsmart_0201bff7b7_2", // Đèn led viền phòng khách
	"light.dqsmart_0201bff7b7_4", // Đèn chùm phòng khách
}

func (m *Manager) runInactivityLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.checkLivingRoomInactivity()
		}
	}
}

func (m *Manager) checkLivingRoomInactivity() {
	m.activityMutex.Lock()
	idleTime := time.Since(m.lastActivityTime)
	m.activityMutex.Unlock()

	if idleTime < m.inactivity {
		return
	}

	activeLights := m.getActiveLivingRoomLights()
	if len(activeLights) == 0 {
		return
	}

	if m.isTVScreenOn() {
		return
	}

	tNow := time.Now().Format("15:04:05")
	m.turnOffLivingRoom(activeLights, idleTime, tNow)
}

func (m *Manager) getActiveLivingRoomLights() []string {
	var onLights []string
	for _, id := range livingRoomLights {
		if m.isEntityOn(id) {
			onLights = append(onLights, id)
		}
	}
	return onLights
}

func (m *Manager) turnOffLivingRoom(lights []string, idle time.Duration, tNow string) {
	for _, id := range lights {
		payload := map[string]interface{}{"entity_id": id}
		_ = m.haClient.RequestJSON("/api/services/light/turn_off", payload, nil)
	}

	// Gọi script tắt điện phòng khách của HA nếu có
	_ = m.haClient.RequestJSON("/api/services/script/turn_on", map[string]interface{}{
		"entity_id": "script.dqsmart_1792859129663",
	}, nil)

	idleMin := int(idle.Minutes())
	msg := fmt.Sprintf("[%s] 🌙 Phòng khách vắng người (>%d phút): Tự động tắt %d đèn phòng khách để tiết kiệm điện", tNow, idleMin, len(lights))
	if m.broadcast != nil {
		m.broadcast(msg)
	}
	fmt.Println(msg)

	m.activityMutex.Lock()
	m.lastActivityTime = time.Now()
	m.activityMutex.Unlock()
}

func (m *Manager) isTVScreenOn() bool {
	return IsTVScreenOn(m.tvHost)
}
