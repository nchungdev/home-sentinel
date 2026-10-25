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

	"github.com/nchungdev/home-sentinel/agent/pkg/arbitration"
	"github.com/nchungdev/home-sentinel/agent/pkg/haclient"
	"github.com/nchungdev/home-sentinel/agent/pkg/llm"
	"github.com/nchungdev/home-sentinel/agent/pkg/mediabridge"
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

	return &Server{
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
	mux.HandleFunc("/api/token/generate", server.handleGenerateToken)

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
