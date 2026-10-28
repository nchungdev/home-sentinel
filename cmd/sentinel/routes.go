package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/nchungdev/home-sentinel/internal/arbitration"
	"github.com/nchungdev/home-sentinel/internal/proactive"
	"github.com/nchungdev/home-sentinel/internal/transport"
	"github.com/nchungdev/home-sentinel/web"
)

const bridgeAccountsURL = "http://127.0.0.1:8088/api/v2/accounts"

func newRouter(server *Server) http.Handler {
	protected := http.NewServeMux()
	registerTVRoutes(protected, server)
	registerSystemRoutes(protected, server)
	registerCameraRoutes(protected, server)
	registerHomeRoutes(protected, server)

	public := http.NewServeMux()
	public.HandleFunc("/{$}", serveIndex)
	public.HandleFunc("/index.html", serveIndex)
	public.HandleFunc("/health", server.handleHealth)
	if server.Playback != nil {
		public.Handle("/media/stream/", server.Playback)
	}
	public.HandleFunc("/api/auth/login", server.handleAuthLogin)
	public.HandleFunc("/api/auth/logout", server.handleAuthLogout)
	public.HandleFunc("/api/auth/status", server.handleAuthStatus)
	public.Handle("/", server.Auth.Protect(protected))

	return server.Auth.AllowNetwork(public)
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(web.GetIndexHTML())
}

func registerTVRoutes(mux *http.ServeMux, server *Server) {
	transportCfg := transport.ConfigFromEnv()
	transportCfg.Token = ""
	transportCfg.AllowedIPs = nil

	mux.Handle("/v2/ask", transport.NewV2Handler(buildServerOrchestrator(server), &transportCfg))
	suggestions := proactive.NewSuggestionEngine(nil, nil)
	mux.Handle("/v2/suggestions", transport.NewV2SuggestionsHandler(suggestions, &transportCfg))
	registerWatchRoutes(mux, &transportCfg)

	mux.HandleFunc("/ws/duinch", server.handleDuinchWS)
	mux.HandleFunc("/api/duinch/greeting", server.handleDuinchGreeting)
	mux.HandleFunc("/api/notify", server.handleApiNotify)
	mux.HandleFunc("/notify", server.handleNotify)
	mux.HandleFunc("/api/webhook/radarr", server.handleRadarrWebhook)
	mux.HandleFunc("/api/webhook/sonarr", server.handleSonarrWebhook)
}

func registerSystemRoutes(mux *http.ServeMux, server *Server) {
	mux.HandleFunc("/v2/catalog/titles/{id}", server.handleCatalogTitle)
	mux.HandleFunc("/v2/catalog/episodes/{id}/playback", server.handleCatalogPlayback)
	mux.HandleFunc("/v2/flow/preview", server.handleRequestFlow)
	mux.HandleFunc("/v2/flow", server.handleLiveRequestFlow)
	mux.HandleFunc("/api/media/sources", server.handleMediaSources)
	mux.HandleFunc("/api/media/search", server.handleMediaSearch)
	mux.HandleFunc("/api/media/browse", server.handleMediaBrowse)
	mux.HandleFunc("/api/media/request", server.handleMediaRequest)
	mux.HandleFunc("/api/nas/status", server.handleNASStatus)
	mux.HandleFunc("/api/token/generate", server.handleGenerateToken)
	mux.HandleFunc("/api/logs/client", server.handleClientLogs)
	mux.HandleFunc("/api/llm/engines", server.handleConfigLLM)
	mux.HandleFunc("/api/llm/test", server.handleLLMTest)
	mux.HandleFunc("/api/llm/login", server.handleLLMLogin)
	mux.HandleFunc("/api/llm/auth", server.handleLLMAuth)
	mux.HandleFunc("/api/groq/config", server.handleGroqConfig)
	mux.HandleFunc("/api/bridge/accounts", handleBridgeAccounts)
	mux.HandleFunc("/api/arbitration/decide", server.handleArbitrationDecide)
}

func registerCameraRoutes(mux *http.ServeMux, server *Server) {
	mux.HandleFunc("/api/camera/scan", server.handleScanCameras)
	mux.HandleFunc("/api/camera/config", server.handleCameraConfig)
	mux.HandleFunc("/api/camera/test-rtsp", server.handleTestCameraRTSP)
	mux.HandleFunc("/api/camera/stream.mjpeg", server.handleStreamMjpeg)
	mux.HandleFunc("/api/camera/ptz", server.handleCameraPTZ)
	mux.HandleFunc("/api/camera/privacy", server.handleCameraPrivacy)
	mux.HandleFunc("/api/camera/detection", server.handleCameraDetection)
	mux.HandleFunc("/api/camera/speak", server.handleCameraSpeak)
	mux.HandleFunc("/api/camera/events", server.handleCameraEvents)
}

func registerHomeRoutes(mux *http.ServeMux, server *Server) {
	mux.HandleFunc("/api/ha/config", server.handleHAConfig)
	mux.HandleFunc("/api/ha/entities", server.handleHAEntities)
	mux.HandleFunc("/api/automation/config", server.handleAutomationConfig)
	mux.HandleFunc("/event", server.handlePresenceEvent)
}

func handleBridgeAccounts(w http.ResponseWriter, r *http.Request) {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(bridgeAccountsURL)
	if err != nil {
		writeJSON(w, []any{})
		return
	}
	defer resp.Body.Close()

	var data any
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		writeJSON(w, []any{})
		return
	}
	writeJSON(w, data)
}

func (s *Server) handlePresenceEvent(w http.ResponseWriter, r *http.Request) {
	var event struct {
		ZoneID    string `json:"zone_id"`
		EventType string `json:"event_type"`
	}
	_ = json.NewDecoder(r.Body).Decode(&event)
	if event.ZoneID == "sofa" {
		s.StateMutex.Lock()
		s.UserOnSofa = event.EventType == "enter"
		s.StateMutex.Unlock()
	}
	writeJSON(w, map[string]string{"status": "ok"})
}

func (s *Server) handleArbitrationDecide(w http.ResponseWriter, r *http.Request) {
	var ctx arbitration.Context
	_ = json.NewDecoder(r.Body).Decode(&ctx)
	writeJSON(w, s.Arbitrator.Arbitrate(ctx))
}
