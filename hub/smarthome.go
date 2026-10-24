package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sync"
)

type Device struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Room        string `json:"room"`
	Category    string `json:"category"`
	State       string `json:"state"`
	Temperature int    `json:"temperature,omitempty"`
	Speed       int    `json:"speed,omitempty"`
	IsCritical  bool   `json:"is_critical"`
}

type SmarthomeManager struct {
	mutex           sync.RWMutex
	devices         map[string]*Device
	orderedIds      []string
	smarthomeBinary string
	rooms           []string
}

func NewSmarthomeManager() *SmarthomeManager {
	manager := &SmarthomeManager{
		devices:    make(map[string]*Device),
		orderedIds: make([]string, 0, 25),
		rooms: []string{
			"Phòng khách",
			"Phòng ngủ chính",
			"Phòng ngủ nhỏ",
			"Cửa chính & Khách",
		},
		smarthomeBinary: findSmarthomeBinary(),
	}

	manager.initializeDefaultDevices()
	manager.syncFromBinary()
	return manager
}

func findSmarthomeBinary() string {
	candidates := []string{
		"services/smarthome/bin/smarthome",
		"/Volumes/512GB/TV/tv-agent-launcher/services/smarthome/bin/smarthome",
	}
	for _, path := range candidates {
		if _, statErr := os.Stat(path); statErr == nil {
			return path
		}
	}
	return ""
}

func (manager *SmarthomeManager) initializeDefaultDevices() {
	defaultItems := []*Device{
		{
			ID: "ct_pk_1_1", Name: "Đèn trần chính", Room: "Phòng khách",
			Category: "light", State: "off",
		},
		{
			ID: "ct_pk_1_2", Name: "Đèn hắt thạch cao", Room: "Phòng khách",
			Category: "light", State: "off",
		},
		{
			ID: "ct_pk_1_3", Name: "Đèn chùm trang trí", Room: "Phòng khách",
			Category: "light", State: "off",
		},
		{
			ID: "ct_pk_1_4", Name: "Đèn ban công", Room: "Phòng khách",
			Category: "light", State: "off",
		},
		{
			ID: "ct_pk_2_1", Name: "Quạt trần phòng khách", Room: "Phòng khách",
			Category: "fan", State: "off", Speed: 2,
		},
		{
			ID: "ct_pk_2_2", Name: "Máy lạnh phòng khách", Room: "Phòng khách",
			Category: "climate", State: "off", Temperature: 24,
		},
		{
			ID: "ct_pk_2_3", Name: "Đèn tủ rượu", Room: "Phòng khách",
			Category: "light", State: "off",
		},
		{
			ID: "ct_pk_2_4", Name: "Rèm tự động", Room: "Phòng khách",
			Category: "curtain", State: "off",
		},
		{
			ID: "ct_pn_chinh_1", Name: "Đèn trần phòng ngủ", Room: "Phòng ngủ chính",
			Category: "light", State: "off",
		},
		{
			ID: "ct_pn_chinh_2", Name: "Máy lạnh phòng ngủ", Room: "Phòng ngủ chính",
			Category: "climate", State: "off", Temperature: 25,
		},
		{
			ID: "ct_pn_chinh_3", Name: "Quạt treo tường", Room: "Phòng ngủ chính",
			Category: "fan", State: "off", Speed: 1,
		},
		{
			ID: "ct_pn_chinh_4", Name: "Đèn ngủ ấm", Room: "Phòng ngủ chính",
			Category: "light", State: "off",
		},
		{
			ID: "ct_pn_nho_1", Name: "Đèn phòng ngủ nhỏ", Room: "Phòng ngủ nhỏ",
			Category: "light", State: "off",
		},
		{
			ID: "ct_pn_nho_2", Name: "Máy lạnh PN nhỏ", Room: "Phòng ngủ nhỏ",
			Category: "climate", State: "off", Temperature: 26,
		},
		{
			ID: "ct_pn_nho_3", Name: "Quạt PN nhỏ", Room: "Phòng ngủ nhỏ",
			Category: "fan", State: "off", Speed: 2,
		},
		{
			ID: "ct_pn_nho_4", Name: "Đèn bàn học", Room: "Phòng ngủ nhỏ",
			Category: "light", State: "off",
		},
		{
			ID: "ct_cua_chinh_1", Name: "Nguồn Smarthome Gateway", Room: "Cửa chính & Khách",
			Category: "switch", State: "on", IsCritical: true,
		},
		{
			ID: "ct_cua_chinh_2", Name: "Đèn sảnh ra vào", Room: "Cửa chính & Khách",
			Category: "light", State: "off",
		},
		{
			ID: "ct_cua_chinh_3", Name: "Đèn hành lang", Room: "Cửa chính & Khách",
			Category: "light", State: "off",
		},
		{
			ID: "ct_cua_chinh_4", Name: "Cảm biến khóa cửa", Room: "Cửa chính & Khách",
			Category: "lock", State: "on",
		},
	}

	for _, item := range defaultItems {
		manager.devices[item.ID] = item
		manager.orderedIds = append(manager.orderedIds, item.ID)
	}
}

func (manager *SmarthomeManager) syncFromBinary() {
	if manager.smarthomeBinary == "" {
		return
	}

	command := exec.Command(manager.smarthomeBinary, "status")
	outputBytes, commandError := command.Output()
	if commandError != nil {
		return
	}

	var parsedList []struct {
		ID    string `json:"id"`
		State string `json:"state"`
	}
	if jsonError := json.Unmarshal(outputBytes, &parsedList); jsonError != nil {
		return
	}

	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	for _, rawItem := range parsedList {
		if existing, found := manager.devices[rawItem.ID]; found {
			existing.State = rawItem.State
		}
	}
}

func (manager *SmarthomeManager) GetAllDevices() []*Device {
	manager.mutex.RLock()
	defer manager.mutex.RUnlock()

	resultList := make([]*Device, 0, len(manager.orderedIds))
	for _, id := range manager.orderedIds {
		deviceCopy := *manager.devices[id]
		resultList = append(resultList, &deviceCopy)
	}
	return resultList
}

func (manager *SmarthomeManager) Toggle(deviceID string) (*Device, error) {
	manager.mutex.Lock()
	targetDevice, found := manager.devices[deviceID]
	if !found {
		manager.mutex.Unlock()
		return nil, fmt.Errorf("không tìm thấy thiết bị: %s", deviceID)
	}

	if targetDevice.IsCritical && targetDevice.State == "on" {
		manager.mutex.Unlock()
		return nil, fmt.Errorf("thiết bị %s là nguồn trọng yếu, không thể tắt", targetDevice.Name)
	}

	nextState := "on"
	if targetDevice.State == "on" {
		nextState = "off"
	}
	targetDevice.State = nextState
	deviceSnapshot := *targetDevice
	manager.mutex.Unlock()

	if manager.smarthomeBinary != "" {
		go func() {
			_ = exec.Command(manager.smarthomeBinary, "toggle", deviceID).Run()
		}()
	}

	return &deviceSnapshot, nil
}

func (manager *SmarthomeManager) SetState(deviceID string, targetState string) (*Device, error) {
	manager.mutex.Lock()
	targetDevice, found := manager.devices[deviceID]
	if !found {
		manager.mutex.Unlock()
		return nil, fmt.Errorf("không tìm thấy thiết bị: %s", deviceID)
	}

	if targetDevice.IsCritical && targetState == "off" {
		manager.mutex.Unlock()
		return nil, fmt.Errorf("thiết bị %s là nguồn trọng yếu, không thể tắt", targetDevice.Name)
	}

	targetDevice.State = targetState
	deviceSnapshot := *targetDevice
	manager.mutex.Unlock()

	if manager.smarthomeBinary != "" {
		go func() {
			_ = exec.Command(manager.smarthomeBinary, "set", deviceID, targetState).Run()
		}()
	}

	return &deviceSnapshot, nil
}

func (manager *SmarthomeManager) SetTemperature(deviceID string, temperature int) (*Device, error) {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	targetDevice, found := manager.devices[deviceID]
	if !found {
		return nil, fmt.Errorf("không tìm thấy thiết bị: %s", deviceID)
	}

	if temperature < 16 {
		temperature = 16
	} else if temperature > 30 {
		temperature = 30
	}

	targetDevice.Temperature = temperature
	targetDevice.State = "on"
	deviceSnapshot := *targetDevice
	return &deviceSnapshot, nil
}
