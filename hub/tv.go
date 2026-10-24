package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"sync"
	"time"
)

type TvStatus struct {
	Online     bool   `json:"online"`
	Host       string `json:"host"`
	DeviceName string `json:"deviceName"`
	PowerState string `json:"powerState"`
	Volume     int    `json:"volume"`
}

type TvManager struct {
	mutex       sync.RWMutex
	tvHost      string
	tvPort      int
	notifyPort  int
	powerState  string
	volume      int
	tvctlScript string
}

func NewTvManager() *TvManager {
	host := os.Getenv("TV_HOST")
	if host == "" {
		host = "192.168.1.8"
	}

	return &TvManager{
		tvHost:      host,
		tvPort:      5555,
		notifyPort:  8790,
		powerState:  "on",
		volume:      25,
		tvctlScript: findTvctlScript(),
	}
}

func findTvctlScript() string {
	candidates := []string{
		"scripts/tvctl",
		"/Volumes/512GB/TV/tv-agent-launcher/scripts/tvctl",
	}
	for _, path := range candidates {
		if _, statErr := os.Stat(path); statErr == nil {
			return path
		}
	}
	return ""
}

func (manager *TvManager) GetStatus() TvStatus {
	manager.mutex.RLock()
	defer manager.mutex.RUnlock()

	targetAddress := fmt.Sprintf("%s:%d", manager.tvHost, manager.tvPort)
	conn, dialErr := net.DialTimeout("tcp", targetAddress, 800*time.Millisecond)
	online := dialErr == nil
	if conn != nil {
		_ = conn.Close()
	}

	return TvStatus{
		Online:     online,
		Host:       manager.tvHost,
		DeviceName: "Xiaomi MiTV-MOOR2",
		PowerState: manager.powerState,
		Volume:     manager.volume,
	}
}

func (manager *TvManager) ExecuteAction(actionName string) (string, error) {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	var keycode int
	var message string

	switch actionName {
	case "power":
		keycode = 26
		if manager.powerState == "on" {
			manager.powerState = "standby"
			message = "Đã tắt nguồn TV (chế độ chờ)"
		} else {
			manager.powerState = "on"
			message = "Đã bật nguồn TV"
		}
	case "home":
		keycode = 3
		message = "Đã bấm phím Home trên TV"
	case "back":
		keycode = 4
		message = "Đã bấm phím Back trên TV"
	case "vol_up":
		keycode = 24
		if manager.volume < 100 {
			manager.volume += 5
		}
		message = fmt.Sprintf("Đã tăng âm lượng TV (%d%%)", manager.volume)
	case "vol_down":
		keycode = 25
		if manager.volume > 0 {
			manager.volume -= 5
		}
		message = fmt.Sprintf("Đã giảm âm lượng TV (%d%%)", manager.volume)
	case "mute":
		keycode = 164
		message = "Đã tắt/bật tiếng TV"
	default:
		return "", fmt.Errorf("hành động không hợp lệ: %s", actionName)
	}

	if manager.tvctlScript != "" {
		command := exec.Command(
			manager.tvctlScript,
			"shell",
			fmt.Sprintf("input keyevent %d", keycode),
		)
		command.Env = append(os.Environ(), fmt.Sprintf("TV_HOST=%s", manager.tvHost))
		go func() {
			_ = command.Run()
		}()
	}

	return message, nil
}

func (manager *TvManager) SendNotification(title string, messageText string) error {
	payload := map[string]interface{}{
		"type":    "message",
		"title":   title,
		"message": messageText,
		"tts":     true,
	}
	encoded, jsonErr := json.Marshal(payload)
	if jsonErr != nil {
		return jsonErr
	}

	destination := fmt.Sprintf("%s:%d", manager.tvHost, manager.notifyPort)
	udpConn, dialErr := net.DialTimeout("udp", destination, 2*time.Second)
	if dialErr != nil {
		return dialErr
	}
	defer udpConn.Close()

	_, writeErr := udpConn.Write(encoded)
	return writeErr
}
