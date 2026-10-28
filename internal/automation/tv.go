package automation

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

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

// HandleClapOnSofa kích hoạt khi user ngồi sofa và vỗ tay -> Mở TV và Bật Quạt tức thì
func (m *Manager) HandleClapOnSofa(userOnSofa bool) bool {
	rule := m.store.Get().SofaClapAction
	if !rule.Enabled || !userOnSofa {
		return false
	}

	tNow := time.Now().Format("15:04:05")

	// 1. Mở TV qua ADB
	tvTarget := rule.TargetTV
	if tvTarget == "" {
		tvTarget = m.tvHost
	}
	if tvTarget != "" {
		go func() {
			_ = TurnOnTV(tvTarget)
		}()
	}

	// 2. Bật các thiết bị được chọn qua Home Assistant
	targets := rule.TargetEntities
	if len(targets) == 0 && rule.TargetFan != "" {
		targets = []string{rule.TargetFan}
	}
	if len(targets) > 0 {
		go m.executeEntitiesAction("Vỗ tay trên Sofa", "turn_on", targets, "", "", false)
	}
	go m.EvaluateCustomRules("clap_detected")

	msg := fmt.Sprintf("[%s] 👏📺 [Vỗ tay trên Sofa] Đã mở TV & Bật quạt (%s)!", tNow, rule.TargetFan)
	if m.broadcast != nil {
		m.broadcast(msg)
	}
	fmt.Println(msg)
	return true
}

func TurnOnTV(tvHost string) error {
	if tvHost == "" {
		return nil
	}
	adbBin := "/home/chungnh/bin/adb"
	if _, err := os.Stat(adbBin); err != nil {
		adbBin = "adb"
	}
	target := tvHost
	if !strings.Contains(target, ":") {
		target = target + ":5555"
	}
	_ = exec.Command(adbBin, "connect", target).Run()
	_ = exec.Command(adbBin, "-s", target, "shell", "input keyevent 224").Run()
	_ = exec.Command(adbBin, "-s", target, "shell", "input keyevent 3").Run()
	return nil
}

func TurnOffTV(tvHost string) error {
	if tvHost == "" {
		return nil
	}
	adbBin := "/home/chungnh/bin/adb"
	if _, err := os.Stat(adbBin); err != nil {
		adbBin = "adb"
	}
	target := tvHost
	if !strings.Contains(target, ":") {
		target = target + ":5555"
	}
	_ = exec.Command(adbBin, "connect", target).Run()
	_ = exec.Command(adbBin, "-s", target, "shell", "input keyevent 223").Run()
	return nil
}
