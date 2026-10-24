package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"
)

type CameraStatus struct {
	Online          bool   `json:"online"`
	Host            string `json:"host"`
	StreamURL       string `json:"streamUrl"`
	PrivacyMode     bool   `json:"privacyMode"`
	ActiveIntrusion bool   `json:"activeIntrusion"`
	LastIntruderIP  string `json:"lastIntruderIp"`
	ShieldText      string `json:"shieldText"`
}

type CameraManager struct {
	mutex           sync.RWMutex
	cameraHost      string
	cameraPort      int
	cameraPassword  string
	backendURL      string
	privacyMode     bool
	activeIntrusion bool
	lastIntruderIP  string
}

func NewCameraManager() *CameraManager {
	initialPass := os.Getenv("CAMERA_PASSWORD")
	if initialPass == "" {
		initialPass = os.Getenv("CAMERA_CLOUD_PASSWORD")
	}

	return &CameraManager{
		cameraHost:     "192.168.1.6",
		cameraPort:     554,
		cameraPassword: initialPass,
		backendURL:     "http://127.0.0.1:8792",
	}
}

func (manager *CameraManager) GetStatus() CameraStatus {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	client := http.Client{Timeout: 800 * time.Millisecond}
	resp, reqErr := client.Get(fmt.Sprintf("%s/api/security/status", manager.backendURL))
	online := reqErr == nil && resp.StatusCode == 200
	if online {
		defer resp.Body.Close()
		var payload struct {
			Status struct {
				ActiveIntrusion bool   `json:"activeIntrusion"`
				LastIntruderIP  string `json:"lastIntruderIp"`
			} `json:"status"`
		}
		if jsonErr := json.NewDecoder(resp.Body).Decode(&payload); jsonErr == nil {
			manager.activeIntrusion = payload.Status.ActiveIntrusion
			manager.lastIntruderIP = payload.Status.LastIntruderIP
		}
	}

	shieldText := "● Đang Bảo Vệ"
	if manager.activeIntrusion {
		shieldText = fmt.Sprintf("🚨 KẾT NỐI LẠ: %s", manager.lastIntruderIP)
	}

	streamURL := "http://localhost:1984/stream.html?src=camera_live"
	if manager.privacyMode {
		streamURL = "about:blank"
	}

	return CameraStatus{
		Online:          online,
		Host:            manager.cameraHost,
		StreamURL:       streamURL,
		PrivacyMode:     manager.privacyMode,
		ActiveIntrusion: manager.activeIntrusion,
		LastIntruderIP:  manager.lastIntruderIP,
		ShieldText:      shieldText,
	}
}

func (manager *CameraManager) SetCredentials(user string, password string) {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	if password != "" {
		manager.cameraPassword = password
	}
}

func (manager *CameraManager) SetPrivacyMode(
	enabled bool,
	user string,
	password string,
) (string, error) {
	manager.mutex.Lock()
	manager.privacyMode = enabled
	effectivePassword := password
	if effectivePassword == "" {
		effectivePassword = manager.cameraPassword
	} else {
		manager.cameraPassword = effectivePassword
	}
	cameraHost := manager.cameraHost
	backendURL := manager.backendURL
	manager.mutex.Unlock()

	payload := map[string]interface{}{
		"ip":            cameraHost,
		"enabled":       enabled,
		"user":          user,
		"password":      effectivePassword,
		"cloudPassword": effectivePassword,
	}
	encoded, _ := json.Marshal(payload)

	client := http.Client{Timeout: 3 * time.Second}
	resp, reqErr := client.Post(
		fmt.Sprintf("%s/api/camera/privacy", backendURL),
		"application/json",
		bytes.NewReader(encoded),
	)
	if reqErr == nil {
		defer resp.Body.Close()
	}

	if enabled {
		return "Đã bật chế độ riêng tư camera (đóng ống kính)", nil
	}
	return "Đã bật camera trực tiếp", nil
}

func (manager *CameraManager) MovePTZ(
	direction string,
	step int,
	user string,
	password string,
) error {
	manager.mutex.Lock()
	effectivePassword := password
	if effectivePassword == "" {
		effectivePassword = manager.cameraPassword
	} else {
		manager.cameraPassword = effectivePassword
	}
	cameraHost := manager.cameraHost
	backendURL := manager.backendURL
	manager.mutex.Unlock()

	payload := map[string]interface{}{
		"ip":            cameraHost,
		"direction":     direction,
		"step":          step,
		"user":          user,
		"password":      effectivePassword,
		"cloudPassword": effectivePassword,
	}
	encoded, jsonErr := json.Marshal(payload)
	if jsonErr != nil {
		return jsonErr
	}

	client := http.Client{Timeout: 3 * time.Second}
	resp, reqErr := client.Post(
		fmt.Sprintf("%s/api/camera/ptz", backendURL),
		"application/json",
		bytes.NewReader(encoded),
	)
	if reqErr != nil {
		return reqErr
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error string `json:"error"`
		}
		if decErr := json.NewDecoder(resp.Body).Decode(&errResp); decErr == nil &&
			errResp.Error != "" {
			return fmt.Errorf("%s", errResp.Error)
		}
		return fmt.Errorf("camera ptz returned status %d", resp.StatusCode)
	}
	return nil
}

func (manager *CameraManager) Speak(
	text string,
	voice string,
	user string,
	password string,
) error {
	manager.mutex.Lock()
	effectivePassword := password
	if effectivePassword == "" {
		effectivePassword = manager.cameraPassword
	} else {
		manager.cameraPassword = effectivePassword
	}
	cameraHost := manager.cameraHost
	backendURL := manager.backendURL
	manager.mutex.Unlock()

	payload := map[string]interface{}{
		"text":          text,
		"voice":         voice,
		"speed":         1.0,
		"ip":            cameraHost,
		"user":          user,
		"password":      effectivePassword,
		"cloudPassword": effectivePassword,
	}
	encoded, jsonErr := json.Marshal(payload)
	if jsonErr != nil {
		return jsonErr
	}

	client := http.Client{Timeout: 6 * time.Second}
	resp, reqErr := client.Post(
		fmt.Sprintf("%s/api/camera/speak", backendURL),
		"application/json",
		bytes.NewReader(encoded),
	)
	if reqErr != nil {
		return reqErr
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error string `json:"error"`
		}
		if decErr := json.NewDecoder(resp.Body).Decode(&errResp); decErr == nil &&
			errResp.Error != "" {
			return fmt.Errorf("%s", errResp.Error)
		}
		return fmt.Errorf("camera speaker returned status %d", resp.StatusCode)
	}
	return nil
}
