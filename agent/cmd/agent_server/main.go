package main

import (
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nchungdev/home-sentinel/agent/pkg/arbitration"
	"github.com/nchungdev/home-sentinel/agent/pkg/haclient"
	"github.com/nchungdev/home-sentinel/agent/pkg/llm"
	"github.com/nchungdev/home-sentinel/agent/pkg/mediabridge"
	"github.com/nchungdev/home-sentinel/agent/pkg/onvifscan"
)

//go:embed web/index.html
var webIndexPage []byte

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
		Token:          token,
		Port:           port,
		NotifyPort:     notifyPort,
		DiscoveryPort:  8789,
		BroadcastIP:    broadcastIP,
		Arbitrator:     arbitration.NewAudioArbitrator(0.15),
		HAClient:       haclient.NewClient("", ""),
		MediaBridge:    mediabridge.NewBridge(""),
		LLMManager:     llm.NewManager("sentinel_llm_config.json"),
		RequestLimiter: make(chan struct{}, 4),
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

func (s *Server) handleAsk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.checkAuth(r) {
		http.Error(w, "unauthorised", http.StatusUnauthorized)
		return
	}

	var req struct {
		Text    string `json:"text"`
		Session string `json:"session"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}

	text := strings.TrimSpace(req.Text)
	if text == "" {
		http.Error(w, "empty question", http.StatusBadRequest)
		return
	}

	// 1. Kiểm tra lệnh Home Assistant trước
	if reply, handled := s.HAClient.HandleCommand(text); handled {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(reply))
		return
	}

	// 2. Chuyển sang LLM Manager với cơ chế Fallback ưu tiên
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	flusher, canFlush := w.(http.Flusher)

	s.LLMManager.StreamChat(text, func(chunk string) {
		_, _ = w.Write([]byte(chunk))
		if canFlush {
			flusher.Flush()
		}
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
		_, _ = w.Write(webIndexPage)
	})

	mux.HandleFunc("/ask", server.handleAsk)
	mux.HandleFunc("/api/llm/engines", server.handleConfigLLM)
	mux.HandleFunc("/api/llm/test", server.handleLLMTest)
	mux.HandleFunc("/api/token/generate", server.handleGenerateToken)
	mux.HandleFunc("/api/camera/scan", server.handleScanCameras)
	mux.HandleFunc("/api/camera/config", server.handleCameraConfig)
	mux.HandleFunc("/api/camera/test-rtsp", server.handleTestCameraRTSP)
	mux.HandleFunc("/api/ha/config", server.handleHAConfig)

	mux.HandleFunc("/notify", func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		_ = server.BroadcastNotification(payload)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
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
