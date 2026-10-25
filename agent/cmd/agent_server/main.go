package main

import (
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
	"github.com/nchungdev/home-sentinel/agent/pkg/automation"
	"github.com/nchungdev/home-sentinel/agent/pkg/haclient"
	"github.com/nchungdev/home-sentinel/agent/pkg/llm"
	"github.com/nchungdev/home-sentinel/agent/pkg/mediabridge"
	"github.com/nchungdev/home-sentinel/agent/pkg/proactive"
	"github.com/nchungdev/home-sentinel/agent/pkg/speech"
	"github.com/nchungdev/home-sentinel/agent/pkg/transport"
	"github.com/nchungdev/home-sentinel/web"
)

type Server struct {
	Token            string
	Port             int
	NotifyPort       int
	DiscoveryPort    int
	BroadcastIP      string
	Arbitrator       *arbitration.AudioArbitrator
	HAClient         *haclient.Client
	MediaBridge      *mediabridge.Bridge
	LLMManager       *llm.Manager
	UserOnSofa       bool
	StateMutex       sync.RWMutex
	RequestLimiter   chan struct{}
	ClientLogs       []ClientLogEntry
	LogsMutex        sync.RWMutex
	eventSubscribers map[chan string]struct{}
	eventSubMutex    sync.Mutex

	AutomationMgr *automation.Manager
	Transcriber   *speech.Transcriber

	transcribeWorkers int
	transcribeLock    sync.Mutex

	lastTVPromptTime time.Time
	promptMutex      sync.Mutex
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
func (s *Server) broadcastTerminal(msg string) {
	s.eventSubMutex.Lock()
	defer s.eventSubMutex.Unlock()
	for ch := range s.eventSubscribers {
		select {
		case ch <- msg:
		default:
		}
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
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ") == s.Token
	}
	return s.Token == ""
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

	server := &Server{
		Token:            os.Getenv("AGENT_TOKEN"),
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

	server.AutomationMgr = automation.NewManager(
		server.HAClient,
		func() automation.CameraConfig {
			var cfg CameraConfigData
			if d, err := os.ReadFile("sentinel_camera_config.json"); err == nil {
				_ = json.Unmarshal(d, &cfg)
			}
			return automation.CameraConfig{
				IP:            cfg.IP,
				Username:      cfg.Username,
				Password:      cfg.Password,
				CloudPassword: cfg.CloudPassword,
			}
		},
		server.broadcastTerminal,
		"192.168.1.8",
		"sentinel_automation_config.json",
	)

	server.Transcriber = speech.NewTranscriber(os.Getenv("GROQ_API_KEY"))
	if data, err := os.ReadFile("sentinel_groq_config.json"); err == nil {
		var groqCfg struct {
			APIKey string `json:"api_key"`
		}
		if json.Unmarshal(data, &groqCfg) == nil && groqCfg.APIKey != "" {
			server.Transcriber.UpdateGroqKey(groqCfg.APIKey)
		}
	}

	return server
}

func main() {
	server := NewServer()
	server.StartDiscoveryResponder()
	startCameraWakeWordListener(server)

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
	mux.HandleFunc("/api/camera/events", server.handleCameraEvents)

	mux.HandleFunc("/api/ha/config", server.handleHAConfig)
	mux.HandleFunc("/api/groq/config", server.handleGroqConfig)
	mux.HandleFunc("/api/automation/config", server.handleAutomationConfig)
	mux.HandleFunc("/notify", server.handleNotify)

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
