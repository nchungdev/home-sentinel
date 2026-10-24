package camera

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type AccessLogItem struct {
	Time     string `json:"time"`
	IP       string `json:"ip"`
	Protocol string `json:"protocol"`
	IsSafe   bool   `json:"isSafe"`
}

type SentinelStatus struct {
	ActiveIntrusion     bool            `json:"activeIntrusion"`
	LastIntruderIP      string          `json:"lastIntruderIp"`
	WhitelistIPs        []string        `json:"whitelistIps"`
	AlarmSpeakerEnabled bool            `json:"alarmSpeakerEnabled"`
	AlarmTvEnabled      bool            `json:"alarmTvEnabled"`
	RecentLogs          []AccessLogItem `json:"recentLogs"`
}

type TvNotificationPayload struct {
	Type    string `json:"type"`
	Title   string `json:"title"`
	Message string `json:"message"`
	Tts     bool   `json:"tts"`
}

type CameraSentinel struct {
	mutex               sync.RWMutex
	go2rtcPort          int
	targetTvHost        string
	targetTvPort        int
	alarmSpeakerEnabled bool
	alarmTvEnabled      bool
	whitelistIPs        map[string]bool
	recentLogs          []AccessLogItem
	activeIntrusion     bool
	lastIntruderIP      string
	lastAlertTimestamp  time.Time
}

func NewCameraSentinel(go2rtcPort int) *CameraSentinel {
	defaultWhitelist := map[string]bool{
		"127.0.0.1":    true,
		"::1":          true,
		"localhost":    true,
		"192.168.1.1":  true,
		"192.168.1.6":  true,
		"192.168.1.8":  true,
		"192.168.1.11": true,
		"192.168.1.13": true,
	}

	return &CameraSentinel{
		go2rtcPort:          go2rtcPort,
		targetTvHost:        "192.168.1.8",
		targetTvPort:        8790,
		alarmSpeakerEnabled: true,
		alarmTvEnabled:      true,
		whitelistIPs:        defaultWhitelist,
		recentLogs:          make([]AccessLogItem, 0, 30),
	}
}

func (sentinel *CameraSentinel) Start(ctx context.Context) {
	ticker := time.NewTicker(2500 * time.Millisecond)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				sentinel.pollGo2rtcStreams()
			}
		}
	}()
}

func (sentinel *CameraSentinel) GetStatus() SentinelStatus {
	sentinel.mutex.RLock()
	defer sentinel.mutex.RUnlock()

	whitelistList := make([]string, 0, len(sentinel.whitelistIPs))
	for ipAddress := range sentinel.whitelistIPs {
		whitelistList = append(whitelistList, ipAddress)
	}

	clonedLogs := make([]AccessLogItem, len(sentinel.recentLogs))
	copy(clonedLogs, sentinel.recentLogs)

	return SentinelStatus{
		ActiveIntrusion:     sentinel.activeIntrusion,
		LastIntruderIP:      sentinel.lastIntruderIP,
		WhitelistIPs:        whitelistList,
		AlarmSpeakerEnabled: sentinel.alarmSpeakerEnabled,
		AlarmTvEnabled:      sentinel.alarmTvEnabled,
		RecentLogs:          clonedLogs,
	}
}

func (sentinel *CameraSentinel) AddWhitelistIP(ipAddress string) {
	sentinel.mutex.Lock()
	defer sentinel.mutex.Unlock()
	cleaned := strings.TrimSpace(ipAddress)
	if cleaned != "" {
		sentinel.whitelistIPs[cleaned] = true
	}
}

func (sentinel *CameraSentinel) RemoveWhitelistIP(ipAddress string) {
	sentinel.mutex.Lock()
	defer sentinel.mutex.Unlock()
	cleaned := strings.TrimSpace(ipAddress)
	delete(sentinel.whitelistIPs, cleaned)
}

func (sentinel *CameraSentinel) SetAlarms(speaker bool, tv bool) {
	sentinel.mutex.Lock()
	defer sentinel.mutex.Unlock()
	sentinel.alarmSpeakerEnabled = speaker
	sentinel.alarmTvEnabled = tv
}

func (sentinel *CameraSentinel) TriggerSecurityAlert(
	intruderIP string,
	speakerCallback func([]byte) error,
) {
	sentinel.mutex.Lock()
	now := time.Now()
	if now.Sub(sentinel.lastAlertTimestamp) < 15*time.Second {
		sentinel.mutex.Unlock()
		return
	}
	sentinel.lastAlertTimestamp = now
	sentinel.activeIntrusion = true
	sentinel.lastIntruderIP = intruderIP
	tvEnabled := sentinel.alarmTvEnabled
	speakerEnabled := sentinel.alarmSpeakerEnabled
	sentinel.mutex.Unlock()

	alertTitle := "CẢNH BÁO BẢO MẬT CAMERA"
	alertMessage := fmt.Sprintf(
		"Phát hiện kết nối lạ tới Camera từ địa chỉ IP: %s",
		intruderIP,
	)

	if tvEnabled {
		go sentinel.sendTvUdpNotification(alertTitle, alertMessage)
	}

	if speakerEnabled && speakerCallback != nil {
		go func() {
			warningPartOne := "Cảnh báo: Bạn đang xâm nhập camera trái phép! "
			warningPartTwo := "Hệ thống đã ghi lại nhật ký IP và báo động."
			voiceWarningText := warningPartOne + warningPartTwo
			audioData, generateError := GenerateSpeechAudio(
				voiceWarningText, "duinch_deep", 600, 1.0,
			)
			if generateError == nil {
				_ = speakerCallback(audioData)
			}
		}()
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

func extractRemoteIP(rawRemote string) string {
	if strings.Contains(rawRemote, "://") {
		parsedUrl, parseError := url.Parse(rawRemote)
		if parseError == nil {
			hostname := parsedUrl.Hostname()
			if hostname != "" {
				return hostname
			}
		}
	}

	if strings.Contains(rawRemote, ":") {
		host, _, splitError := net.SplitHostPort(rawRemote)
		if splitError == nil {
			return host
		}
	}
	return strings.Trim(rawRemote, "[]")
}
