package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"
	"time"
)

type HubSettings struct {
	CameraIP        string `json:"cameraIp"`
	CameraPort      int    `json:"cameraPort"`
	CameraUser      string `json:"cameraUser"`
	CameraPassword  string `json:"cameraPassword"`
	CameraCloudPass string `json:"cameraCloudPass"`

	SmarthomeHost     string `json:"smarthomeHost"`
	SmarthomeToken    string `json:"smarthomeToken"`
	SmarthomeUser     string `json:"smarthomeUser"`
	SmarthomePassword string `json:"smarthomePassword"`

	TvHost string `json:"tvHost"`
	TvPort int    `json:"tvPort"`

	AgentModel    string `json:"agentModel"`
	AgentApiKey   string `json:"agentApiKey"`
	AgentEndpoint string `json:"agentEndpoint"`
	AgentVoice    string `json:"agentVoice"`
	AgentOutput   string `json:"agentOutput"`
}

type SettingsManager struct {
	mutex        sync.RWMutex
	filePath     string
	currentValue HubSettings
}

func NewSettingsManager(filePath string) *SettingsManager {
	manager := &SettingsManager{
		filePath:     filePath,
		currentValue: defaultHubSettings(),
	}

	manager.loadFromFile()
	return manager
}

func defaultHubSettings() HubSettings {
	return HubSettings{
		CameraIP:      "192.168.1.6",
		CameraPort:    554,
		CameraUser:    "admin",
		SmarthomeHost: "http://192.168.1.1:8123",
		TvHost:        "192.168.1.8",
		TvPort:        5555,
		AgentModel:    "agy",
		AgentVoice:    "Ban Mai (Nữ)",
		AgentOutput:   "browser",
	}
}

func (manager *SettingsManager) loadFromFile() {
	contentBytes, readErr := os.ReadFile(manager.filePath)
	if readErr != nil {
		return
	}

	var parsed HubSettings
	if jsonErr := json.Unmarshal(contentBytes, &parsed); jsonErr == nil {
		manager.currentValue = parsed
	}
}

func (manager *SettingsManager) Get() HubSettings {
	manager.mutex.RLock()
	defer manager.mutex.RUnlock()

	return manager.currentValue
}

func (manager *SettingsManager) Save(updated HubSettings) error {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	manager.currentValue = updated
	encoded, marshalErr := json.MarshalIndent(updated, "", "  ")
	if marshalErr != nil {
		return marshalErr
	}

	return os.WriteFile(manager.filePath, encoded, 0644)
}

func ScanNetworkTargets(targetType string) []string {
	detectedList := make([]string, 0)
	var targetPort int

	switch targetType {
	case "tv":
		targetPort = 5555
	case "camera":
		targetPort = 554
	case "gateway":
		targetPort = 80
	default:
		targetPort = 80
	}

	testHosts := []string{
		"192.168.1.1", "192.168.1.2", "192.168.1.3", "192.168.1.4",
		"192.168.1.5", "192.168.1.6", "192.168.1.7", "192.168.1.8",
		"192.168.1.9", "192.168.1.10", "192.168.1.15", "192.168.1.20",
		"192.168.1.50", "192.168.1.100", "192.168.1.101", "192.168.1.200",
	}

	for _, host := range testHosts {
		addr := fmt.Sprintf("%s:%d", host, targetPort)
		conn, dialErr := net.DialTimeout("tcp", addr, 120*time.Millisecond)
		if dialErr == nil {
			detectedList = append(detectedList, host)
			_ = conn.Close()
		}
	}

	return detectedList
}
