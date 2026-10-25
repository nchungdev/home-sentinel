package main

import (
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nchungdev/home-sentinel/agent/pkg/arbitration"
	"github.com/nchungdev/home-sentinel/agent/pkg/camstream"
	"github.com/nchungdev/home-sentinel/agent/pkg/haclient"
	"github.com/nchungdev/home-sentinel/agent/pkg/llm"
	"github.com/nchungdev/home-sentinel/agent/pkg/mediabridge"
	"github.com/nchungdev/home-sentinel/agent/pkg/onvifscan"
	"github.com/nchungdev/home-sentinel/agent/pkg/proactive"
	"github.com/nchungdev/home-sentinel/agent/pkg/transport"
	"github.com/nchungdev/home-sentinel/web"
)

type Server struct {
	Token          string
	Port           int
	NotifyPort     int
	DiscoveryPort  int
	BroadcastIP    string
	Arbitrator     *arbitration.AudioArbitrator
	HAClient       *haclient.Client
	MediaBridge    *mediabridge.Bridge
	LLMManager     *llm.Manager
	UserOnSofa     bool
	StateMutex     sync.RWMutex
	RequestLimiter chan struct{}
	ClientLogs     []ClientLogEntry
	LogsMutex      sync.RWMutex
	eventSubscribers map[chan string]struct{}
	eventSubMutex    sync.Mutex
}

type ClientLogEntry struct {
	Timestamp string `json:"timestamp"`
	ClientIP  string `json:"client_ip"`
	Session   string `json:"session"`
	Text      string `json:"text"`
	Source    string `json:"source"`
	Response  string `json:"response"`
}

func (s *Server) AddClientLog(entry ClientLogEntry) {
	s.LogsMutex.Lock()
	defer s.LogsMutex.Unlock()
	s.ClientLogs = append(s.ClientLogs, entry)
	if len(s.ClientLogs) > 100 {
		s.ClientLogs = s.ClientLogs[len(s.ClientLogs)-100:]
	}
}

func NewServer() *Server {
	port := 8788
	if p := os.Getenv("AGENT_PORT"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			port = v
		}
	}

	notifyPort := 8790
	if p := os.Getenv("AGENT_NOTIFY_PORT"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			notifyPort = v
		}
	}

	broadcastIP := os.Getenv("AGENT_BROADCAST_IP")
	if broadcastIP == "" {
		broadcastIP = "255.255.255.255"
	}

	token := os.Getenv("AGENT_TOKEN")

	server := &Server{
		Token:            token,
		Port:             port,
		NotifyPort:       notifyPort,
		DiscoveryPort:    8789,
		BroadcastIP:      broadcastIP,
		Arbitrator:       arbitration.NewAudioArbitrator(0.15),
		HAClient:         haclient.NewClient("", ""),
		MediaBridge:      mediabridge.NewBridge(""),
		LLMManager:       llm.NewManager("sentinel_llm_config.json"),
		RequestLimiter:   make(chan struct{}, 4),
		eventSubscribers: make(map[chan string]struct{}),
	}

	if data, err := os.ReadFile("sentinel_ha_config.json"); err == nil {
		var haCfg struct {
			URL   string `json:"url"`
			Token string `json:"token"`
		}
		if json.Unmarshal(data, &haCfg) == nil {
			server.HAClient.UpdateConfig(haCfg.URL, haCfg.Token)
		}
	}

	return server
}

func (s *Server) BroadcastNotification(payload map[string]interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	conn, err := net.Dial("udp", fmt.Sprintf("%s:%d", s.BroadcastIP, s.NotifyPort))
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = conn.Write(data)
	return err
}

func (s *Server) StartDiscoveryResponder() {
	go func() {
		addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("0.0.0.0:%d", s.DiscoveryPort))
		if err != nil {
			return
		}
		conn, err := net.ListenUDP("udp", addr)
		if err != nil {
			return
		}
		defer conn.Close()

		buf := make([]byte, 128)
		for {
			n, remoteAddr, err := conn.ReadFrom(buf)
			if err != nil {
				continue
			}
			if strings.TrimSpace(string(buf[:n])) == "TV_AGENT_DISCOVER" {
				payload, _ := json.Marshal(map[string]int{"port": s.Port})
				_, _ = conn.WriteTo(payload, remoteAddr)
			}
		}
	}()
}

func (s *Server) checkAuth(r *http.Request) bool {
	if s.Token == "" {
		return true
	}
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ") == s.Token
	}
	return false
}

func (s *Server) handleClientLogs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if r.Method == http.MethodDelete {
		s.LogsMutex.Lock()
		s.ClientLogs = nil
		s.LogsMutex.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "logs": []ClientLogEntry{}})
		return
	}

	s.LogsMutex.RLock()
	defer s.LogsMutex.RUnlock()
	logs := make([]ClientLogEntry, len(s.ClientLogs))
	copy(logs, s.ClientLogs)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":   true,
		"logs": logs,
	})
}

func (s *Server) handleConfigLLM(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	if r.Method == http.MethodGet {
		engines := s.LLMManager.ListEngines()
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":      true,
			"engines": engines,
		})
		return
	}

	if r.Method == http.MethodPost {
		var payload struct {
			PriorityOrder []string `json:"priority_order"`
			GeminiKey     string   `json:"gemini_key"`
			LocalLLMURL   string   `json:"local_llm_url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}

		err := s.LLMManager.SaveConfig(payload.PriorityOrder, payload.GeminiKey, payload.LocalLLMURL)
		if err != nil {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "message": "Đã lưu cài đặt AI Engines"})
		return
	}

	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handleGenerateToken(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	bytes := make([]byte, 24)
	if _, err := rand.Read(bytes); err != nil {
		http.Error(w, "failed to generate random token", http.StatusInternalServerError)
		return
	}
	newToken := hex.EncodeToString(bytes)

	s.Token = newToken
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":    true,
		"token": newToken,
	})
}

func (s *Server) handleScanCameras(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	devices, err := onvifscan.ScanDevices(2500 * time.Millisecond)
	if err != nil {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":      false,
			"error":   err.Error(),
			"devices": []onvifscan.DiscoveredDevice{},
		})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"devices": devices,
	})
}

type CameraConfigData struct {
	IP            string `json:"ip"`
	Port          int    `json:"port"`
	Username      string `json:"username"`
	Password      string `json:"password"`
	CloudPassword string `json:"cloud_password"`
	Brand         string `json:"brand"`
	Model         string `json:"model"`
	StreamPath    string `json:"stream_path"`
}

func (s *Server) handleCameraConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	configPath := "sentinel_camera_config.json"

	if r.Method == http.MethodGet {
		var cfg CameraConfigData
		if data, err := os.ReadFile(configPath); err == nil {
			_ = json.Unmarshal(data, &cfg)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":     true,
			"config": cfg,
		})
		return
	}

	if r.Method == http.MethodPost {
		var cfg CameraConfigData
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		data, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := os.WriteFile(configPath, data, 0644); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":      true,
			"message": "Đã lưu thông tin cấu hình camera!",
		})
		return
	}

	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handleTestCameraRTSP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		IP       string `json:"ip"`
		Port     int    `json:"port"`
		User     string `json:"user"`
		Password string `json:"password"`
		Path     string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if req.Port <= 0 {
		req.Port = 554
	}
	if req.Path == "" {
		req.Path = "/stream1"
	}
	if !strings.HasPrefix(req.Path, "/") {
		req.Path = "/" + req.Path
	}

	target := fmt.Sprintf("%s:%d", req.IP, req.Port)
	conn, err := net.DialTimeout("tcp", target, 3*time.Second)
	if err != nil {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("Không thể kết nối tới %s: %v", target, err),
		})
		return
	}
	defer conn.Close()

	auth := ""
	if req.User != "" && req.Password != "" {
		raw := fmt.Sprintf("%s:%s", req.User, req.Password)
		encoded := hex.EncodeToString([]byte(raw)) // or base64
		_ = encoded
		// Basic Auth requires base64
		encodedAuth := fmt.Sprintf("Basic %s", base64Encode(raw))
		auth = fmt.Sprintf("Authorization: %s\r\n", encodedAuth)
	}

	probeMsg := fmt.Sprintf("DESCRIBE rtsp://%s:%d%s RTSP/1.0\r\nCSeq: 1\r\n%sAccept: application/sdp\r\n\r\n",
		req.IP, req.Port, req.Path, auth)

	_ = conn.SetDeadline(time.Now().Add(4 * time.Second))
	if _, err := conn.Write([]byte(probeMsg)); err != nil {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("Lỗi gửi RTSP probe: %v", err),
		})
		return
	}

	buf := make([]byte, 2048)
	n, err := conn.Read(buf)
	if err != nil {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("Lỗi nhận RTSP response: %v", err),
		})
		return
	}

	firstLine := strings.Split(string(buf[:n]), "\r\n")[0]
	if strings.Contains(firstLine, "200 OK") {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Kết nối thành công (200 OK - Luồng RTSP sẵn sàng)",
		})
		return
	} else if strings.Contains(firstLine, "401") {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Sai tài khoản hoặc mật khẩu Camera (401 Unauthorized)",
		})
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Camera phản hồi: %s", firstLine),
	})
}

func base64Encode(str string) string {
	bytes := []byte(str)
	const base64Table = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var result []byte
	pad := len(bytes) % 3
	for i := 0; i < len(bytes); i += 3 {
		var n uint32
		switch len(bytes) - i {
		case 1:
			n = uint32(bytes[i]) << 16
			result = append(result, base64Table[(n>>18)&63], base64Table[(n>>12)&63], '=', '=')
		case 2:
			n = (uint32(bytes[i]) << 16) | (uint32(bytes[i+1]) << 8)
			result = append(result, base64Table[(n>>18)&63], base64Table[(n>>12)&63], base64Table[(n>>6)&63], '=')
		default:
			n = (uint32(bytes[i]) << 16) | (uint32(bytes[i+1]) << 8) | uint32(bytes[i+2])
			result = append(result, base64Table[(n>>18)&63], base64Table[(n>>12)&63], base64Table[(n>>6)&63], base64Table[n&63])
		}
	}
	_ = pad
	return string(result)
}

func (s *Server) handleHAConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	configPath := "sentinel_ha_config.json"

	if r.Method == http.MethodGet {
		var cfg struct {
			URL   string `json:"url"`
			Token string `json:"token"`
		}
		if data, err := os.ReadFile(configPath); err == nil {
			_ = json.Unmarshal(data, &cfg)
		}
		if cfg.URL == "" {
			cfg.URL = s.HAClient.BaseURL
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":     true,
			"config": cfg,
		})
		return
	}

	if r.Method == http.MethodPost {
		var cfg struct {
			URL   string `json:"url"`
			Token string `json:"token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		s.HAClient.UpdateConfig(cfg.URL, cfg.Token)
		data, _ := json.MarshalIndent(cfg, "", "  ")
		_ = os.WriteFile(configPath, data, 0644)

		ok, msg := s.HAClient.TestConnection()
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":      ok,
			"message": msg,
		})
		return
	}

	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handleLLMTest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		EngineID string `json:"engine_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	ok, desc := s.LLMManager.TestConnection(req.EngineID)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      ok,
		"message": desc,
	})
}

func (s *Server) handleStreamMjpeg(w http.ResponseWriter, r *http.Request) {
	configPath := "sentinel_camera_config.json"
	var cfg CameraConfigData
	if data, err := os.ReadFile(configPath); err == nil {
		_ = json.Unmarshal(data, &cfg)
	}

	q := r.URL.Query()
	host := q.Get("ip")
	if host == "" {
		host = cfg.IP
	}
	if host == "" {
		http.Error(w, "camera ip not configured", http.StatusBadRequest)
		return
	}

	user := q.Get("user")
	if user == "" {
		user = cfg.Username
	}
	pass := q.Get("password")
	if pass == "" {
		pass = cfg.Password
	}
	path := q.Get("path")
	if path == "" {
		path = cfg.StreamPath
	}
	if path == "" {
		path = "/stream1"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	rtspURL := fmt.Sprintf("rtsp://%s:%s@%s:554%s", user, pass, host, path)

	broadcaster := camstream.GetBroadcaster()
	broadcaster.ServeHTTP(w, r, rtspURL)
}

func (s *Server) handleCameraPTZ(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	configPath := "sentinel_camera_config.json"
	var cfg CameraConfigData
	if data, err := os.ReadFile(configPath); err == nil {
		_ = json.Unmarshal(data, &cfg)
	}

	var req struct {
		Direction string `json:"direction"`
		Step      int    `json:"step"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	effectivePass := cfg.CloudPassword
	if effectivePass == "" {
		effectivePass = cfg.Password
	}

	if cfg.IP == "" || effectivePass == "" {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":    false,
			"error": "Chưa cấu hình IP hoặc mật khẩu Camera",
		})
		return
	}

	session := camstream.GetTapoSession(cfg.IP, cfg.Username, effectivePass)
	var err error
	if req.Direction != "" {
		err = session.MoveDirection(req.Direction, req.Step)
	} else {
		err = fmt.Errorf("thiếu hướng xoay")
	}

	if err != nil {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "message": "Đã thực thi lệnh xoay PTZ"})
}

func (s *Server) handleCameraPrivacy(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	configPath := "sentinel_camera_config.json"
	var cfg CameraConfigData
	if data, err := os.ReadFile(configPath); err == nil {
		_ = json.Unmarshal(data, &cfg)
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	effectivePass := cfg.CloudPassword
	if effectivePass == "" {
		effectivePass = cfg.Password
	}

	if cfg.IP == "" || effectivePass == "" {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":    false,
			"error": "Chưa cấu hình IP hoặc mật khẩu Camera",
		})
		return
	}

	session := camstream.GetTapoSession(cfg.IP, cfg.Username, effectivePass)
	err := session.SetPrivacyMode(req.Enabled)
	if err != nil {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"message": fmt.Sprintf("Đã đặt Privacy Mode = %v", req.Enabled),
	})
}

func (s *Server) handleCameraDetection(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	configPath := "sentinel_camera_config.json"
	var cfg CameraConfigData
	if data, err := os.ReadFile(configPath); err == nil {
		_ = json.Unmarshal(data, &cfg)
	}

	effectivePass := cfg.CloudPassword
	if effectivePass == "" {
		effectivePass = cfg.Password
	}

	if cfg.IP == "" || effectivePass == "" {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":    false,
			"error": "Chưa cấu hình IP hoặc mật khẩu Camera",
		})
		return
	}

	session := camstream.GetTapoSession(cfg.IP, cfg.Username, effectivePass)

	if r.Method == http.MethodGet {
		states, err := session.GetDetectionStates()
		if err != nil {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":     true,
			"states": states,
		})
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Feature string `json:"feature"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	err := session.SetDetectionFeature(req.Feature, req.Enabled)
	if err != nil {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":      false,
			"error":   "Camera Tapo C200 chạy firmware mới yêu cầu điều khiển nhận diện thông qua App Tapo",
			"details": err.Error(),
		})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"message": fmt.Sprintf("Đã cập nhật chế độ %s = %v", req.Feature, req.Enabled),
	})
}

func (s *Server) handleCameraSpeak(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Text  string `json:"text"`
		Voice string `json:"voice"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	text := strings.TrimSpace(req.Text)
	if text == "" {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": "Thiếu nội dung câu nói"})
		return
	}

	configPath := "sentinel_camera_config.json"
	var cfg CameraConfigData
	if data, err := os.ReadFile(configPath); err == nil {
		_ = json.Unmarshal(data, &cfg)
	}

	effectivePass := cfg.CloudPassword
	if effectivePass == "" {
		effectivePass = cfg.Password
	}
	if cfg.IP == "" || effectivePass == "" {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":    false,
			"error": "Chưa cấu hình IP hoặc mật khẩu Camera",
		})
		return
	}

	// 1. Tạo âm thanh MP3 (Ưu tiên edge-tts chất lượng cao, tự động thử lại hoặc fallback Google TTS nếu Microsoft ngắt kết nối)
	voice := "vi-VN-HoaiMyNeural"
	if req.Voice == "male" {
		voice = "vi-VN-NamMinhNeural"
	}

	tmpMp3 := fmt.Sprintf("/tmp/tts_%d.mp3", time.Now().UnixNano())
	tmpWav := fmt.Sprintf("/tmp/tts_%d.wav", time.Now().UnixNano())
	defer os.Remove(tmpMp3)
	defer os.Remove(tmpWav)

	ttsSuccess := false
	var lastTTSErr error

	// Thử edge-tts tối đa 2 lần
	for attempt := 0; attempt < 2; attempt++ {
		ttsCmd := exec.Command("edge-tts", "--voice", voice, "--text", text, "--write-media", tmpMp3)
		out, err := ttsCmd.CombinedOutput()
		if err == nil {
			if stat, serr := os.Stat(tmpMp3); serr == nil && stat.Size() > 0 {
				ttsSuccess = true
				break
			}
		}
		lastTTSErr = fmt.Errorf("%v (%s)", err, strings.TrimSpace(string(out)))
		time.Sleep(150 * time.Millisecond)
	}

	// Nếu edge-tts không trả về audio (Microsoft rate limit / NoAudioReceived), fallback sang Google Translate TTS
	if !ttsSuccess {
		encodedText := url.QueryEscape(text)
		googleURL := fmt.Sprintf("https://translate.google.com/translate_tts?ie=UTF-8&client=tw-ob&tl=vi&q=%s", encodedText)
		curlCmd := exec.Command("curl", "-sL", "-A", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)", googleURL, "-o", tmpMp3)
		if err := curlCmd.Run(); err == nil {
			if stat, serr := os.Stat(tmpMp3); serr == nil && stat.Size() > 0 {
				ttsSuccess = true
			}
		}
	}

	if !ttsSuccess {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":    false,
			"error": fmt.Sprintf("Lỗi tạo giọng nói TTS: %v", lastTTSErr),
		})
		return
	}

	// 2. Chuyển đổi sang chuẩn Alaw 8000Hz PCM cho Tapo Speaker, chèn 600ms khoảng lặng ở đầu để tránh mất chữ đầu (do buffer kết nối loa)
	ffmpegCmd := exec.Command("ffmpeg", "-y", "-i", tmpMp3, "-af", "adelay=600|600", "-ar", "8000", "-ac", "1", "-c:a", "pcm_alaw", tmpWav)
	if err := ffmpegCmd.Run(); err != nil {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":    false,
			"error": fmt.Sprintf("Lỗi xử lý audio (ffmpeg): %v", err),
		})
		return
	}

	// 3. Gọi tapo_speaker phát trực tiếp lên loa camera port 8800
	speakerBin := "/home/chungnh/home-sentinel/bin/tapo_speaker"
	tapoURL := fmt.Sprintf("tapo://%s@%s:8800", effectivePass, cfg.IP)
	spkCmd := exec.Command(speakerBin, tapoURL, tmpWav)
	out, err := spkCmd.CombinedOutput()
	if err != nil {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":    false,
			"error": fmt.Sprintf("Lỗi phát loa Camera: %v (%s)", err, strings.TrimSpace(string(out))),
		})
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"message": fmt.Sprintf("Đã phát giọng nói lên loa Camera thành công: \"%s\"", text),
	})
}

func main() {
	server := NewServer()
	server.StartDiscoveryResponder()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && r.URL.Path != "/index.html" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(web.GetIndexHTML())
	})

	transportCfg := transport.ConfigFromEnv()
	if transportCfg.Token == "" {
		transportCfg.Token = server.Token
	}

	orchestratorInstance := buildServerOrchestrator(server)
	mux.Handle("/v2/ask", transport.NewV2Handler(orchestratorInstance, &transportCfg))

	suggestionEngine := proactive.NewSuggestionEngine(nil, nil)
	mux.Handle("/v2/suggestions", transport.NewV2SuggestionsHandler(suggestionEngine, &transportCfg))

	mux.HandleFunc("/api/logs/client", server.handleClientLogs)
	mux.HandleFunc("/api/llm/engines", server.handleConfigLLM)
	mux.HandleFunc("/api/llm/test", server.handleLLMTest)
	mux.HandleFunc("/api/token/generate", server.handleGenerateToken)
	mux.HandleFunc("/api/bridge/accounts", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		client := &http.Client{Timeout: 3 * time.Second}
		resp, err := client.Get("http://127.0.0.1:8088/api/v2/accounts")
		if err != nil {
			_ = json.NewEncoder(w).Encode([]interface{}{})
			return
		}
		defer resp.Body.Close()
		var data interface{}
		if err := json.NewDecoder(resp.Body).Decode(&data); err == nil {
			_ = json.NewEncoder(w).Encode(data)
		} else {
			_ = json.NewEncoder(w).Encode([]interface{}{})
		}
	})

	mux.HandleFunc("/api/camera/scan", server.handleScanCameras)
	mux.HandleFunc("/api/camera/config", server.handleCameraConfig)
	mux.HandleFunc("/api/camera/test-rtsp", server.handleTestCameraRTSP)
	mux.HandleFunc("/api/camera/stream.mjpeg", server.handleStreamMjpeg)
	mux.HandleFunc("/api/camera/ptz", server.handleCameraPTZ)
	mux.HandleFunc("/api/camera/privacy", server.handleCameraPrivacy)
	mux.HandleFunc("/api/camera/detection", server.handleCameraDetection)
	mux.HandleFunc("/api/camera/speak", server.handleCameraSpeak)
	mux.HandleFunc("/api/ha/config", server.handleHAConfig)

	mux.HandleFunc("/notify", func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		_ = server.BroadcastNotification(payload)

		nowStr := time.Now().Format("15:04:05")
		clientIP := r.RemoteAddr
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			clientIP = strings.Split(fwd, ",")[0]
		}
		summary := "Nhận sự kiện notification"
		terminalLogMsg := ""
		if src, ok := payload["source"].(string); ok && src == "home_assistant" {
			fn, _ := payload["friendly_name"].(string)
			et, _ := payload["event_type"].(string)
			st, _ := payload["state"].(string)
			summary = fmt.Sprintf("📡 HA Event [%s]: %s (trạng thái: %s)", et, fn, st)
			if fn == "" {
				fn = "Camera Phòng Khách"
			}
			terminalLogMsg = fmt.Sprintf("[%s] 🚨 CẢNH BÁO: %s - Phát hiện người/chuyển động! (trạng thái: %s)", nowStr, fn, st)
		} else if ev, ok := payload["event"].(string); ok {
			summary = fmt.Sprintf("📡 Event: %s", ev)
			terminalLogMsg = fmt.Sprintf("[%s] 🔔 Event: %s", nowStr, ev)
		}
		server.AddClientLog(ClientLogEntry{
			Timestamp: nowStr,
			ClientIP:  clientIP,
			Session:   "system_notify",
			Text:      summary,
			Source:    "ha_webhook",
			Response:  "Đã tiếp nhận và broadcast qua UDP :8790",
		})

		if terminalLogMsg != "" {
			server.eventSubMutex.Lock()
			for ch := range server.eventSubscribers {
				select {
				case ch <- terminalLogMsg:
				default:
				}
			}
			server.eventSubMutex.Unlock()
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/api/camera/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		ch := make(chan string, 16)
		server.eventSubMutex.Lock()
		server.eventSubscribers[ch] = struct{}{}
		server.eventSubMutex.Unlock()

		defer func() {
			server.eventSubMutex.Lock()
			delete(server.eventSubscribers, ch)
			close(ch)
			server.eventSubMutex.Unlock()
		}()

		// Gửi ping chào đầu
		fmt.Fprintf(w, "data: %s\n\n", "CONNECTED")
		flusher.Flush()

		ctx := r.Context()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				fmt.Fprintf(w, "data: %s\n\n", msg)
				flusher.Flush()
			}
		}
	})

	mux.HandleFunc("/event", func(w http.ResponseWriter, r *http.Request) {
		var event struct {
			ZoneID    string `json:"zone_id"`
			EventType string `json:"event_type"`
		}
		_ = json.NewDecoder(r.Body).Decode(&event)
		if event.ZoneID == "sofa" {
			server.StateMutex.Lock()
			server.UserOnSofa = (event.EventType == "enter")
			server.StateMutex.Unlock()
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/api/arbitration/decide", func(w http.ResponseWriter, r *http.Request) {
		var ctx arbitration.Context
		_ = json.NewDecoder(r.Body).Decode(&ctx)
		decision := server.Arbitrator.Arbitrate(ctx)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(decision)
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"service": "sentinel-server-go",
			"status":  "ok",
			"token":   server.Token != "",
		})
	})

	fmt.Printf("Home Sentinel Server (Go) listening on :%d\n", server.Port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", server.Port), mux); err != nil {
		fmt.Printf("Sentinel server failed: %v\n", err)
	}
}
