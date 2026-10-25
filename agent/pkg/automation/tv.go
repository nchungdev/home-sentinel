package automation

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

func (m *Manager) HandleSofaArrival() {
	rule := m.store.Get().SofaFan
	if !rule.Enabled || rule.TargetFan == "" {
		return
	}

	cooldown := time.Duration(rule.CooldownSec) * time.Second
	if cooldown <= 0 {
		cooldown = 10 * time.Minute
	}

	m.activityMutex.Lock()
	if time.Since(m.lastFanTrigger) < cooldown {
		m.activityMutex.Unlock()
		return
	}
	if m.isEntityOn(rule.TargetFan) {
		m.activityMutex.Unlock()
		return
	}
	m.lastFanTrigger = time.Now()
	m.lastActivityTime = time.Now()
	m.activityMutex.Unlock()

	if m.haClient != nil && m.haClient.BaseURL != "" {
		payload := map[string]interface{}{"entity_id": rule.TargetFan}
		err := m.haClient.RequestJSON("/api/services/fan/turn_on", payload, nil)
		if err == nil {
			tNow := time.Now().Format("15:04:05")
			msg := fmt.Sprintf("[%s] 🌪️ Phát hiện có người ngồi Sofa: Đã tự động bật quạt (%s)!", tNow, rule.TargetFan)
			if m.broadcast != nil {
				m.broadcast(msg)
			}
			fmt.Println(msg)
		}
	}
}

func IsTVScreenOn(tvHost string) bool {
	if tvHost == "" {
		return false
	}
	adbBin := "/home/chungnh/bin/adb"
	if _, err := os.Stat(adbBin); err != nil {
		adbBin = "adb"
	}
	target := tvHost
	if !strings.Contains(target, ":") {
		target = target + ":5555"
	}

	cmd := exec.Command(adbBin, "-s", target, "shell", "dumpsys power | grep mWakefulness=")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "mWakefulness=Awake")
}
