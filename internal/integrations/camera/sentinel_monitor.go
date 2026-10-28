package camera

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"
)

func (sentinel *CameraSentinel) pollGo2rtcStreams() {
	endpoint := fmt.Sprintf("http://127.0.0.1:%d/api/streams", sentinel.go2rtcPort)
	httpClient := http.Client{Timeout: 1500 * time.Millisecond}

	httpResponse, requestError := httpClient.Get(endpoint)
	if requestError != nil {
		return
	}
	defer httpResponse.Body.Close()

	if httpResponse.StatusCode != http.StatusOK {
		return
	}

	var streamsData map[string]interface{}
	if decodeError := json.NewDecoder(httpResponse.Body).Decode(&streamsData); decodeError != nil {
		return
	}

	cameraLiveEntry, hasCameraLive := streamsData["camera_live"]
	if !hasCameraLive {
		return
	}

	cameraLiveMap, isMap := cameraLiveEntry.(map[string]interface{})
	if !isMap {
		return
	}

	producersArray, _ := cameraLiveMap["producers"].([]interface{})
	consumersArray, _ := cameraLiveMap["consumers"].([]interface{})

	currentTimeString := time.Now().Format("15:04:05")
	detectedIntruder := false
	var intruderIP string

	sentinel.mutex.Lock()
	defer sentinel.mutex.Unlock()

	checkConnection := func(connectionData interface{}, protocolName string) {
		connectionMap, ok := connectionData.(map[string]interface{})
		if !ok {
			return
		}
		rawRemote, _ := connectionMap["remote"].(string)
		if rawRemote == "" {
			rawRemote, _ = connectionMap["url"].(string)
		}
		extractedIP := extractRemoteIP(rawRemote)
		if extractedIP == "" {
			return
		}

		isWhitelisted := sentinel.whitelistIPs[extractedIP]
		if !isWhitelisted {
			detectedIntruder = true
			intruderIP = extractedIP
		}

		logItem := AccessLogItem{
			Time:     currentTimeString,
			IP:       extractedIP,
			Protocol: protocolName,
			IsSafe:   isWhitelisted,
		}
		sentinel.prependLog(logItem)
	}

	for _, consumer := range consumersArray {
		checkConnection(consumer, "WebRTC/RTSP Viewer")
	}
	for _, producer := range producersArray {
		checkConnection(producer, "Stream Producer")
	}

	sentinel.activeIntrusion = detectedIntruder
	if detectedIntruder && intruderIP != "" {
		sentinel.lastIntruderIP = intruderIP
	}
}

func (sentinel *CameraSentinel) prependLog(logItem AccessLogItem) {
	for _, existing := range sentinel.recentLogs {
		if existing.IP == logItem.IP && existing.Protocol == logItem.Protocol {
			return
		}
	}
	sentinel.recentLogs = append([]AccessLogItem{logItem}, sentinel.recentLogs...)
	if len(sentinel.recentLogs) > 20 {
		sentinel.recentLogs = sentinel.recentLogs[:20]
	}
}

func (sentinel *CameraSentinel) sendTvUdpNotification(alertTitle string, alertMessage string) {
	notificationPayload := TvNotificationPayload{
		Type:    "security",
		Title:   alertTitle,
		Message: alertMessage,
		Tts:     true,
	}
	encodedPayload, jsonError := json.Marshal(notificationPayload)
	if jsonError != nil {
		return
	}

	destinationAddress := fmt.Sprintf("%s:%d", sentinel.targetTvHost, sentinel.targetTvPort)
	udpConnection, dialError := net.DialTimeout("udp", destinationAddress, 2*time.Second)
	if dialError != nil {
		return
	}
	defer udpConnection.Close()

	_, _ = udpConnection.Write(encodedPayload)
}
