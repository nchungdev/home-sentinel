package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

type AgentChatMessage struct {
	Role            string   `json:"role"`
	Content         string   `json:"content"`
	ExecutedActions []string `json:"executed_actions,omitempty"`
}

type AgentOrchestrator struct {
	mutex            sync.RWMutex
	smarthomeManager *SmarthomeManager
	tvManager        *TvManager
	cameraManager    *CameraManager
	history          []AgentChatMessage
}

func NewAgentOrchestrator(
	smarthome *SmarthomeManager,
	tv *TvManager,
	camera *CameraManager,
) *AgentOrchestrator {
	return &AgentOrchestrator{
		smarthomeManager: smarthome,
		tvManager:        tv,
		cameraManager:    camera,
		history:          make([]AgentChatMessage, 0, 50),
	}
}

func (agent *AgentOrchestrator) ProcessMessage(userText string) AgentChatMessage {
	cleanedText := strings.TrimSpace(userText)
	if cleanedText == "" {
		return AgentChatMessage{
			Role:    "assistant",
			Content: "Chào bạn! Tôi có thể giúp gì cho ngôi nhà của bạn hôm nay?",
		}
	}

	lowered := strings.ToLower(cleanedText)
	executedActions := make([]string, 0)
	var responseText string

	if strings.Contains(lowered, "xem phim") {
		_, _ = agent.tvManager.ExecuteAction("power")
		_, _ = agent.smarthomeManager.SetState("ct_pk_1_1", "off")
		_, _ = agent.smarthomeManager.SetState("ct_pk_1_2", "on")
		executedActions = append(
			executedActions,
			"Bật TV", "Tắt đèn trần chính", "Bật đèn hắt dịu",
		)
		responseText = "Đã kích hoạt chế độ xem phim: Mở TV và chỉnh ánh sáng dịu phòng khách!"
	} else if strings.Contains(lowered, "đi ngủ") {
		for _, dev := range agent.smarthomeManager.GetAllDevices() {
			if dev.Category == "light" {
				_, _ = agent.smarthomeManager.SetState(dev.ID, "off")
			}
		}
		_, _ = agent.smarthomeManager.SetState("ct_pn_chinh_2", "on")
		_, _ = agent.smarthomeManager.SetTemperature("ct_pn_chinh_2", 25)
		_, _ = agent.cameraManager.SetPrivacyMode(false, "", "")
		executedActions = append(
			executedActions,
			"Tắt toàn bộ đèn",
			"Bật máy lạnh PN 25°C",
			"Kích hoạt khiên an ninh",
		)
		responseText = "Đã bật chế độ đi ngủ: Tắt hết đèn, mở máy lạnh 25°C và bảo vệ camera!"
	} else if strings.Contains(lowered, "tắt hết đèn") ||
		strings.Contains(lowered, "tắt tất cả đèn") {
		for _, dev := range agent.smarthomeManager.GetAllDevices() {
			if dev.Category == "light" {
				_, _ = agent.smarthomeManager.SetState(dev.ID, "off")
			}
		}
		executedActions = append(executedActions, "Tắt toàn bộ đèn trong nhà")
		responseText = "Đã tắt toàn bộ hệ thống đèn trong nhà."
	} else if strings.Contains(lowered, "bật hết đèn") ||
		strings.Contains(lowered, "bật tất cả đèn") {
		for _, dev := range agent.smarthomeManager.GetAllDevices() {
			if dev.Category == "light" {
				_, _ = agent.smarthomeManager.SetState(dev.ID, "on")
			}
		}
		executedActions = append(executedActions, "Bật toàn bộ đèn trong nhà")
		responseText = "Đã bật tất cả đèn trong nhà."
	} else if strings.Contains(lowered, "bật tv") || strings.Contains(lowered, "mở tv") ||
		strings.Contains(lowered, "bật tivi") || strings.Contains(lowered, "mở tivi") {
		msg, _ := agent.tvManager.ExecuteAction("power")
		executedActions = append(executedActions, "Bật nguồn TV")
		responseText = msg
	} else if strings.Contains(lowered, "tắt tv") || strings.Contains(lowered, "tắt tivi") {
		msg, _ := agent.tvManager.ExecuteAction("power")
		executedActions = append(executedActions, "Tắt nguồn TV")
		responseText = msg
	} else if strings.Contains(lowered, "tăng âm lượng") {
		msg, _ := agent.tvManager.ExecuteAction("vol_up")
		executedActions = append(executedActions, "Tăng âm lượng TV")
		responseText = msg
	} else if strings.Contains(lowered, "giảm âm lượng") {
		msg, _ := agent.tvManager.ExecuteAction("vol_down")
		executedActions = append(executedActions, "Giảm âm lượng TV")
		responseText = msg
	} else if strings.Contains(lowered, "tắt tiếng tv") || strings.Contains(lowered, "mute tv") {
		msg, _ := agent.tvManager.ExecuteAction("mute")
		executedActions = append(executedActions, "Tắt tiếng TV")
		responseText = msg
	} else if strings.Contains(lowered, "tắt camera") ||
		strings.Contains(lowered, "bật chế độ riêng tư") ||
		strings.Contains(lowered, "chế độ riêng tư") {
		msg, _ := agent.cameraManager.SetPrivacyMode(true, "", "")
		executedActions = append(executedActions, "Bật Privacy Mode camera")
		responseText = msg
	} else if strings.Contains(lowered, "tắt chế độ riêng tư") ||
		strings.Contains(lowered, "bật camera") ||
		strings.Contains(lowered, "mở camera") {
		msg, _ := agent.cameraManager.SetPrivacyMode(false, "", "")
		executedActions = append(executedActions, "Tắt Privacy Mode camera")
		responseText = msg
	} else if strings.Contains(lowered, "xoay camera sang trái") ||
		strings.Contains(lowered, "xoay camera qua trái") ||
		strings.Contains(lowered, "quay camera sang trái") {
		_ = agent.cameraManager.MovePTZ("left", 20, "", "")
		executedActions = append(executedActions, "Xoay camera sang trái")
		responseText = "Đã xoay camera sang bên trái"
	} else if strings.Contains(lowered, "xoay camera sang phải") ||
		strings.Contains(lowered, "xoay camera qua phải") ||
		strings.Contains(lowered, "quay camera sang phải") {
		_ = agent.cameraManager.MovePTZ("right", 20, "", "")
		executedActions = append(executedActions, "Xoay camera sang phải")
		responseText = "Đã xoay camera sang bên phải"
	} else if strings.Contains(lowered, "xoay camera lên") ||
		strings.Contains(lowered, "quay camera lên") {
		_ = agent.cameraManager.MovePTZ("up", 20, "", "")
		executedActions = append(executedActions, "Xoay camera lên trên")
		responseText = "Đã xoay camera hướng lên trên"
	} else if strings.Contains(lowered, "xoay camera xuống") ||
		strings.Contains(lowered, "quay camera xuống") {
		_ = agent.cameraManager.MovePTZ("down", 20, "", "")
		executedActions = append(executedActions, "Xoay camera xuống dưới")
		responseText = "Đã xoay camera hướng xuống dưới"
	} else if strings.Contains(lowered, "dừng xoay") || strings.Contains(lowered, "dừng camera") {
		_ = agent.cameraManager.MovePTZ("stop", 0, "", "")
		executedActions = append(executedActions, "Dừng xoay camera")
		responseText = "Đã dừng xoay camera"
	} else if strings.Contains(lowered, "phát loa camera:") ||
		strings.Contains(lowered, "phát loa:") {
		parts := strings.SplitN(cleanedText, ":", 2)
		if len(parts) == 2 && strings.TrimSpace(parts[1]) != "" {
			speakText := strings.TrimSpace(parts[1])
			_ = agent.cameraManager.Speak(speakText, "duinch_natural", "", "")
			executedActions = append(
				executedActions,
				fmt.Sprintf("Phát loa camera: \"%s\"", speakText),
			)
			responseText = fmt.Sprintf("Đã phát âm thanh ra loa camera: \"%s\"", speakText)
		} else {
			responseText = "Vui lòng nhập cú pháp: 'phát loa camera: <nội dung>'"
		}
	} else if strings.Contains(lowered, "an ninh") || strings.Contains(lowered, "kết nối lạ") {
		status := agent.cameraManager.GetStatus()
		if status.ActiveIntrusion {
			responseText = fmt.Sprintf(
				"🚨 Cảnh báo! Phát hiện kết nối lạ tới Camera từ IP: %s",
				status.LastIntruderIP,
			)
		} else {
			responseText = "🛡️ Hệ thống an ninh Camera đang hoạt động bình thường, an toàn."
		}
	} else {
		targetState := ""
		if strings.Contains(lowered, "bật") || strings.Contains(lowered, "mở") ||
			strings.Contains(lowered, "on") {
			targetState = "on"
		} else if strings.Contains(lowered, "tắt") || strings.Contains(lowered, "đóng") ||
			strings.Contains(lowered, "off") {
			targetState = "off"
		}

		matchedDevice := agent.findMatchingDevice(lowered)
		if matchedDevice != nil && targetState != "" {
			tempRegex := regexp.MustCompile(`(\d{2})\s*(độ|c|degree)`)
			tempMatch := tempRegex.FindStringSubmatch(lowered)
			if matchedDevice.Category == "climate" && len(tempMatch) > 1 {
				if parsedTemp, err := strconv.Atoi(tempMatch[1]); err == nil {
					_, _ = agent.smarthomeManager.SetTemperature(matchedDevice.ID, parsedTemp)
					executedActions = append(
						executedActions,
						fmt.Sprintf("Đặt %s ở %d°C", matchedDevice.Name, parsedTemp),
					)
					responseText = fmt.Sprintf(
						"Đã bật %s và đặt nhiệt độ %d°C.",
						matchedDevice.Name, parsedTemp,
					)
				}
			}

			if responseText == "" {
				updated, err := agent.smarthomeManager.SetState(matchedDevice.ID, targetState)
				if err != nil {
					responseText = fmt.Sprintf("Lỗi: %v", err)
				} else {
					actionWord := "bật"
					if targetState == "off" {
						actionWord = "tắt"
					}
					executedActions = append(
						executedActions,
						fmt.Sprintf("%s %s", strings.Title(actionWord), updated.Name),
					)
					responseText = fmt.Sprintf(
						"Đã %s %s (%s) thành công.",
						actionWord, updated.Name, updated.Room,
					)
				}
			}
		} else if strings.Contains(lowered, "nhà thế nào") ||
			strings.Contains(lowered, "trạng thái") {
			devices := agent.smarthomeManager.GetAllDevices()
			onCount := 0
			for _, d := range devices {
				if d.State == "on" {
					onCount++
				}
			}
			tvStatus := agent.tvManager.GetStatus()
			camStatus := agent.cameraManager.GetStatus()
			tvText := "Tắt"
			if tvStatus.PowerState == "on" {
				tvText = "Bật"
			}
			responseText = fmt.Sprintf(
				"Báo cáo ngôi nhà: %d thiết bị đang bật. TV đang %s. Camera: %s.",
				onCount, tvText, camStatus.ShieldText,
			)
		} else {
			responseText = fmt.Sprintf(
				"Tôi đã ghi nhận: \"%s\". Bạn có thể ra lệnh: 'Bật đèn phòng khách', 'Mở TV'...",
				cleanedText,
			)
		}
	}

	reply := AgentChatMessage{
		Role:            "assistant",
		Content:         responseText,
		ExecutedActions: executedActions,
	}

	agent.mutex.Lock()
	agent.history = append(agent.history, AgentChatMessage{Role: "user", Content: cleanedText})
	agent.history = append(agent.history, reply)
	if len(agent.history) > 30 {
		agent.history = agent.history[len(agent.history)-30:]
	}
	agent.mutex.Unlock()

	return reply
}

func (agent *AgentOrchestrator) findMatchingDevice(lowered string) *Device {
	allDevices := agent.smarthomeManager.GetAllDevices()

	for _, dev := range allDevices {
		devNameLower := strings.ToLower(dev.Name)
		roomLower := strings.ToLower(dev.Room)

		hasDeviceType := false
		if (strings.Contains(lowered, "đèn") && dev.Category == "light") ||
			(strings.Contains(lowered, "quạt") && dev.Category == "fan") ||
			((strings.Contains(lowered, "máy lạnh") || strings.Contains(lowered, "điều hòa")) &&
				dev.Category == "climate") ||
			(strings.Contains(lowered, "rèm") && dev.Category == "curtain") {
			hasDeviceType = true
		}

		if hasDeviceType {
			if strings.Contains(lowered, "khách") && strings.Contains(roomLower, "khách") {
				return dev
			}
			if strings.Contains(lowered, "nhỏ") && strings.Contains(roomLower, "nhỏ") {
				return dev
			}
			if (strings.Contains(lowered, "chính") || strings.Contains(lowered, "master")) &&
				strings.Contains(roomLower, "chính") {
				return dev
			}
			if strings.Contains(lowered, "cửa") && strings.Contains(roomLower, "cửa") {
				return dev
			}
		}

		if strings.Contains(lowered, devNameLower) {
			return dev
		}
	}

	return nil
}

func (agent *AgentOrchestrator) GetHistory() []AgentChatMessage {
	agent.mutex.RLock()
	defer agent.mutex.RUnlock()

	cloned := make([]AgentChatMessage, len(agent.history))
	copy(cloned, agent.history)
	return cloned
}
