package main

import (
	_ "embed"
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
var testWebPage []byte

type Server struct {
	Token          string
	Port           int
	NotifyPort     int
	DiscoveryPort  int
	BroadcastIP    string
	Arbitrator     *arbitration.AudioArbitrator
	HAClient       *haclient.Client
	MediaBridge    *mediabridge.Bridge
	LLMEngine      *llm.Engine
	UserOnSofa     bool
	StateMutex     sync.RWMutex
	RequestLimiter chan struct{}
}

type NotificationPayload struct {
	Type    string `json:"type"`
	Title   string `json:"title"`
	Message string `json:"message"`
	TTS     bool   `json:"tts"`
}

type AskRequest struct {
	Text    string `json:"text"`
	Session string `json:"session"`
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
		LLMEngine:      llm.NewEngine(),
		RequestLimiter: make(chan struct{}, 4),
	}
}

func (s *Server) BroadcastNotification(payload NotificationPayload) error {
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

func (s *Server) handleNotify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.checkAuth(r) {
		http.Error(w, "unauthorised", http.StatusUnauthorized)
		return
	}

	var payload NotificationPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}

	_ = s.BroadcastNotification(payload)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleArbitration(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var ctx arbitration.Context
	if err := json.NewDecoder(r.Body).Decode(&ctx); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}

	s.StateMutex.RLock()
	if !ctx.UserOnSofa && s.UserOnSofa {
		ctx.UserOnSofa = true
	}
	s.StateMutex.RUnlock()

	decision := s.Arbitrator.Arbitrate(ctx)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(decision)
}

func (s *Server) handleEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var event struct {
		ZoneID    string `json:"zone_id"`
		EventType string `json:"event_type"`
		Timestamp int64  `json:"timestamp"`
	}

	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}

	if event.ZoneID == "sofa" {
		s.StateMutex.Lock()
		s.UserOnSofa = (event.EventType == "enter")
		s.StateMutex.Unlock()

		if event.EventType == "enter" {
			_ = s.BroadcastNotification(NotificationPayload{
				Type:    "interaction",
				Title:   "Phát hiện người",
				Message: "Chào bạn! Chúc bạn xem TV vui vẻ.",
				TTS:     false,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
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

	var req AskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}

	text := strings.TrimSpace(req.Text)
	if text == "" {
		http.Error(w, "empty question", http.StatusBadRequest)
		return
	}

	if reply, handled := s.HAClient.HandleCommand(text); handled {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(reply))
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	flusher, canFlush := w.(http.Flusher)

	_ = s.LLMEngine.StreamChat(text, func(chunk string) {
		_, _ = w.Write([]byte(chunk))
		if canFlush {
			flusher.Flush()
		}
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
		_, _ = w.Write(testWebPage)
	})
	mux.HandleFunc("/notify", server.handleNotify)
	mux.HandleFunc("/api/arbitration/decide", server.handleArbitration)
	mux.HandleFunc("/event", server.handleEvent)
	mux.HandleFunc("/ask", server.handleAsk)

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ok",
			"service": "agent-server-go",
			"uptime":  "running",
		})
	})

	fmt.Printf("Home Sentinel Agent (Go) listening on :%d\n", server.Port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", server.Port), mux); err != nil {
		fmt.Printf("Agent server failed: %v\n", err)
	}
}
