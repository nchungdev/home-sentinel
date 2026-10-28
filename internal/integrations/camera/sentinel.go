package camera

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
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
		"127.0.0.1": true,
		"::1":       true,
		"localhost": true,
	}

	tvHost := os.Getenv("TARGET_TV_HOST")
	if tvHost == "" {
		tvHost = os.Getenv("TV_HOST")
	}
	if tvHost == "" {
		tvHost = "192.168.1.50"
	}

	return &CameraSentinel{
		go2rtcPort:          go2rtcPort,
		targetTvHost:        tvHost,
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
