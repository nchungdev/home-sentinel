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

	"github.com/nchungdev/home-sentinel/internal/adapters/nas"
	"github.com/nchungdev/home-sentinel/internal/arbitration"
	"github.com/nchungdev/home-sentinel/internal/automation"
	"github.com/nchungdev/home-sentinel/internal/catalog"
	"github.com/nchungdev/home-sentinel/internal/integrations/haclient"
	"github.com/nchungdev/home-sentinel/internal/integrations/mediabridge"
	"github.com/nchungdev/home-sentinel/internal/llm"
	"github.com/nchungdev/home-sentinel/internal/mediasearch"
	"github.com/nchungdev/home-sentinel/internal/playback"
	"github.com/nchungdev/home-sentinel/internal/requestflow"
	"github.com/nchungdev/home-sentinel/internal/speech"
	"github.com/nchungdev/home-sentinel/internal/transport"
)

type Server struct {
	catalogOnce      sync.Once
	catalogService   *catalog.Service
	flowOnce         sync.Once
	flowRunner       requestflow.Runner
	Playback         *playback.Service
	MediaSearch      *mediasearch.Service
	Auth             *authGate
	NAS              *nas.Adapter
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
		Auth: newAuthGate(
			os.Getenv("AGENT_TOKEN"),
			transport.ParseAllowedIPs(os.Getenv("AGENT_ALLOW")),
			authStoreFile,
		),
		NAS:              nas.NewAdapter(nas.NewSystemProbeFromEnv()),
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
	server.Playback = playback.New()
	server.MediaSearch = mediasearch.FromEnvironment(server.MediaBridge)
	server.MediaSearch.SetPlayable(server.Playback.Resolver.CanResolve)
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
		defaultTVHost(),
		"configs/automation.json",
	)
	return server
}

func main() {
	if captureLoginBrowser() {
		return
	}
	server := NewServer()
	server.LLMManager.PrewarmAgent()
	server.StartDiscoveryResponder()
	startCameraWakeWordListener(server)

	if !server.Auth.Required() {
		fmt.Println("WARNING: no access token configured; every LAN client has full access")
	}
	address := net.JoinHostPort(os.Getenv("AGENT_BIND"), strconv.Itoa(server.Port))
	fmt.Printf("Home Sentinel Server (Go) listening on %s\n", address)
	if err := http.ListenAndServe(address, newRouter(server)); err != nil {
		fmt.Printf("Sentinel server failed: %v\n", err)
	}
}
