package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strconv"
)

//go:embed web/index.html
var EmbeddedIndexHTML []byte

//go:embed web/icons/*
var EmbeddedIcons embed.FS

type HubServer struct {
	port              int
	smarthomeManager  *SmarthomeManager
	tvManager         *TvManager
	cameraManager     *CameraManager
	agentOrchestrator *AgentOrchestrator
	settingsManager   *SettingsManager
	httpServer        *http.Server
}

type DeviceToggleRequest struct {
	ID string `json:"id"`
}

type DeviceSetRequest struct {
	ID          string `json:"id"`
	State       string `json:"state"`
	Temperature int    `json:"temperature"`
}

type TvActionRequest struct {
	Action string `json:"action"`
}

type TvNotifyRequest struct {
	Title   string `json:"title"`
	Message string `json:"message"`
}

type CameraPrivacyRequest struct {
	Privacy       bool   `json:"privacy"`
	User          string `json:"user"`
	Password      string `json:"password"`
	CloudPassword string `json:"cloudPassword"`
}

type CameraPTZRequest struct {
	Direction     string `json:"direction"`
	Step          int    `json:"step"`
	User          string `json:"user"`
	Password      string `json:"password"`
	CloudPassword string `json:"cloudPassword"`
}

type CameraSpeakRequest struct {
	Text          string `json:"text"`
	Voice         string `json:"voice"`
	User          string `json:"user"`
	Password      string `json:"password"`
	CloudPassword string `json:"cloudPassword"`
}

type AgentChatRequest struct {
	Text string `json:"text"`
}

func NewHubServer(port int) *HubServer {
	smarthome := NewSmarthomeManager()
	tv := NewTvManager()
	camera := NewCameraManager()
	agent := NewAgentOrchestrator(smarthome, tv, camera)
	settings := NewSettingsManager("settings.json")

	return &HubServer{
		port:              port,
		smarthomeManager:  smarthome,
		tvManager:         tv,
		cameraManager:     camera,
		agentOrchestrator: agent,
		settingsManager:   settings,
	}
}

func (server *HubServer) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", server.handleIndex)

	if iconsSub, err := fs.Sub(EmbeddedIcons, "web/icons"); err == nil {
		mux.Handle("/icons/", http.StripPrefix("/icons/", http.FileServer(http.FS(iconsSub))))
	}

	mux.HandleFunc("/api/hub/overview", server.handleOverview)
	mux.HandleFunc("/api/hub/settings", server.handleSettings)
	mux.HandleFunc("/api/hub/scan", server.handleScan)
	mux.HandleFunc("/api/hub/device/toggle", server.handleDeviceToggle)
	mux.HandleFunc("/api/hub/device/set", server.handleDeviceSet)
	mux.HandleFunc("/api/hub/tv/action", server.handleTvAction)
	mux.HandleFunc("/api/hub/tv/notify", server.handleTvNotify)
	mux.HandleFunc("/api/hub/camera/privacy", server.handleCameraPrivacy)
	mux.HandleFunc("/api/hub/camera/ptz", server.handleCameraPTZ)
	mux.HandleFunc("/api/hub/camera/speak", server.handleCameraSpeak)
	mux.HandleFunc("/api/hub/agent/chat", server.handleAgentChat)

	server.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", server.port),
		Handler: enableCORS(mux),
	}

	fmt.Printf("FreeTV Unified Home Hub running at http://localhost:%d\n", server.port)
	return server.httpServer.ListenAndServe()
}

func (server *HubServer) handleIndex(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Path != "/" && request.URL.Path != "/index.html" {
		http.NotFound(writer, request)
		return
	}
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.Header().Set("Content-Length", strconv.Itoa(len(EmbeddedIndexHTML)))
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(EmbeddedIndexHTML)
}

func (server *HubServer) handleOverview(writer http.ResponseWriter, request *http.Request) {
	devices := server.smarthomeManager.GetAllDevices()
	tvStatus := server.tvManager.GetStatus()
	cameraStatus := server.cameraManager.GetStatus()
	history := server.agentOrchestrator.GetHistory()

	response := map[string]interface{}{
		"ok":           true,
		"devices":      devices,
		"rooms":        server.smarthomeManager.rooms,
		"tv":           tvStatus,
		"camera":       cameraStatus,
		"agentHistory": history,
	}
	server.writeJSON(writer, response, http.StatusOK)
}

func (server *HubServer) handleDeviceToggle(
	writer http.ResponseWriter,
	request *http.Request,
) {
	if request.Method != http.MethodPost {
		http.Error(writer, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload DeviceToggleRequest
	if decodeErr := json.NewDecoder(request.Body).Decode(&payload); decodeErr != nil {
		server.writeJSON(writer, map[string]interface{}{"ok": false, "error": "Invalid JSON"}, 400)
		return
	}

	updatedDevice, toggleErr := server.smarthomeManager.Toggle(payload.ID)
	if toggleErr != nil {
		server.writeJSON(
			writer,
			map[string]interface{}{"ok": false, "error": toggleErr.Error()},
			400,
		)
		return
	}

	server.writeJSON(
		writer,
		map[string]interface{}{"ok": true, "device": updatedDevice},
		http.StatusOK,
	)
}

func (server *HubServer) handleDeviceSet(
	writer http.ResponseWriter,
	request *http.Request,
) {
	if request.Method != http.MethodPost {
		http.Error(writer, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload DeviceSetRequest
	_ = json.NewDecoder(request.Body).Decode(&payload)

	if payload.Temperature > 0 {
		_, _ = server.smarthomeManager.SetTemperature(payload.ID, payload.Temperature)
	}

	updatedDevice, setErr := server.smarthomeManager.SetState(payload.ID, payload.State)
	if setErr != nil {
		server.writeJSON(
			writer,
			map[string]interface{}{"ok": false, "error": setErr.Error()},
			400,
		)
		return
	}

	server.writeJSON(
		writer,
		map[string]interface{}{"ok": true, "device": updatedDevice},
		http.StatusOK,
	)
}

func (server *HubServer) handleTvAction(
	writer http.ResponseWriter,
	request *http.Request,
) {
	if request.Method != http.MethodPost {
		http.Error(writer, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload TvActionRequest
	_ = json.NewDecoder(request.Body).Decode(&payload)

	message, actionErr := server.tvManager.ExecuteAction(payload.Action)
	if actionErr != nil {
		server.writeJSON(
			writer,
			map[string]interface{}{"ok": false, "error": actionErr.Error()},
			400,
		)
		return
	}

	server.writeJSON(
		writer,
		map[string]interface{}{
			"ok":      true,
			"message": message,
			"status":  server.tvManager.GetStatus(),
		},
		http.StatusOK,
	)
}

func (server *HubServer) handleTvNotify(
	writer http.ResponseWriter,
	request *http.Request,
) {
	var payload TvNotifyRequest
	_ = json.NewDecoder(request.Body).Decode(&payload)

	title := payload.Title
	if title == "" {
		title = "Thông báo Smart Home"
	}

	notifyErr := server.tvManager.SendNotification(title, payload.Message)
	if notifyErr != nil {
		server.writeJSON(
			writer,
			map[string]interface{}{"ok": false, "error": notifyErr.Error()},
			500,
		)
		return
	}

	server.writeJSON(
		writer,
		map[string]interface{}{"ok": true, "message": "Đã gửi thông báo lên TV"},
		http.StatusOK,
	)
}

func (server *HubServer) handleCameraPrivacy(
	writer http.ResponseWriter,
	request *http.Request,
) {
	var payload CameraPrivacyRequest
	_ = json.NewDecoder(request.Body).Decode(&payload)

	effectivePassword := payload.CloudPassword
	if effectivePassword == "" {
		effectivePassword = payload.Password
	}

	message, privacyErr := server.cameraManager.SetPrivacyMode(
		payload.Privacy, payload.User, effectivePassword,
	)
	if privacyErr != nil {
		server.writeJSON(
			writer,
			map[string]interface{}{"ok": false, "error": privacyErr.Error()},
			http.StatusBadRequest,
		)
		return
	}

	server.writeJSON(
		writer,
		map[string]interface{}{
			"ok":      true,
			"message": message,
			"status":  server.cameraManager.GetStatus(),
		},
		http.StatusOK,
	)
}

func (server *HubServer) handleCameraPTZ(
	writer http.ResponseWriter,
	request *http.Request,
) {
	if request.Method != http.MethodPost {
		server.writeJSON(
			writer,
			map[string]interface{}{"ok": false, "error": "Chỉ hỗ trợ POST"},
			http.StatusMethodNotAllowed,
		)
		return
	}

	var payload CameraPTZRequest
	_ = json.NewDecoder(request.Body).Decode(&payload)

	if payload.Direction == "" {
		server.writeJSON(
			writer,
			map[string]interface{}{"ok": false, "error": "Thiếu hướng xoay camera"},
			http.StatusBadRequest,
		)
		return
	}

	effectivePassword := payload.CloudPassword
	if effectivePassword == "" {
		effectivePassword = payload.Password
	}

	moveErr := server.cameraManager.MovePTZ(
		payload.Direction, payload.Step, payload.User, effectivePassword,
	)
	if moveErr != nil {
		server.writeJSON(
			writer,
			map[string]interface{}{"ok": false, "error": moveErr.Error()},
			http.StatusBadRequest,
		)
		return
	}

	server.writeJSON(
		writer,
		map[string]interface{}{
			"ok":        true,
			"message":   fmt.Sprintf("Đã xoay camera: %s", payload.Direction),
			"direction": payload.Direction,
		},
		http.StatusOK,
	)
}

func (server *HubServer) handleCameraSpeak(
	writer http.ResponseWriter,
	request *http.Request,
) {
	var payload CameraSpeakRequest
	_ = json.NewDecoder(request.Body).Decode(&payload)

	voice := payload.Voice
	if voice == "" {
		voice = "duinch_natural"
	}

	effectivePassword := payload.CloudPassword
	if effectivePassword == "" {
		effectivePassword = payload.Password
	}

	speakErr := server.cameraManager.Speak(
		payload.Text, voice, payload.User, effectivePassword,
	)
	if speakErr != nil {
		server.writeJSON(
			writer,
			map[string]interface{}{"ok": false, "error": speakErr.Error()},
			http.StatusBadRequest,
		)
		return
	}

	server.writeJSON(
		writer,
		map[string]interface{}{"ok": true, "message": "Đã phát âm thanh ra camera"},
		http.StatusOK,
	)
}

func (server *HubServer) handleAgentChat(
	writer http.ResponseWriter,
	request *http.Request,
) {
	var payload AgentChatRequest
	_ = json.NewDecoder(request.Body).Decode(&payload)

	reply := server.agentOrchestrator.ProcessMessage(payload.Text)

	overview := map[string]interface{}{
		"devices": server.smarthomeManager.GetAllDevices(),
		"tv":      server.tvManager.GetStatus(),
		"camera":  server.cameraManager.GetStatus(),
	}

	server.writeJSON(
		writer,
		map[string]interface{}{
			"ok":       true,
			"reply":    reply,
			"overview": overview,
		},
		http.StatusOK,
	)
}

func (server *HubServer) writeJSON(
	writer http.ResponseWriter,
	payload interface{},
	statusCode int,
) {
	encoded, jsonErr := json.Marshal(payload)
	if jsonErr != nil {
		http.Error(writer, jsonErr.Error(), 500)
		return
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("Content-Length", strconv.Itoa(len(encoded)))
	writer.WriteHeader(statusCode)
	_, _ = writer.Write(encoded)
}

func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Access-Control-Allow-Origin", "*")
		writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if request.Method == http.MethodOptions {
			writer.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(writer, request)
	})
}

type ScanRequest struct {
	Type string `json:"type"`
}

func (server *HubServer) handleSettings(
	writer http.ResponseWriter,
	request *http.Request,
) {
	if request.Method == http.MethodGet {
		currentSettings := server.settingsManager.Get()
		server.writeJSON(writer, map[string]interface{}{
			"ok":       true,
			"settings": currentSettings,
		}, http.StatusOK)
		return
	}

	if request.Method == http.MethodPost {
		var incoming HubSettings
		if parseErr := json.NewDecoder(request.Body).Decode(&incoming); parseErr != nil {
			server.writeJSON(
				writer,
				map[string]interface{}{"ok": false, "error": "Invalid settings payload"},
				http.StatusBadRequest,
			)
			return
		}

		if saveErr := server.settingsManager.Save(incoming); saveErr != nil {
			server.writeJSON(
				writer,
				map[string]interface{}{"ok": false, "error": saveErr.Error()},
				http.StatusInternalServerError,
			)
			return
		}

		server.writeJSON(writer, map[string]interface{}{
			"ok":       true,
			"settings": incoming,
		}, http.StatusOK)
		return
	}

	http.Error(writer, "Method not allowed", http.StatusMethodNotAllowed)
}

func (server *HubServer) handleScan(
	writer http.ResponseWriter,
	request *http.Request,
) {
	if request.Method != http.MethodPost {
		http.Error(writer, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ScanRequest
	_ = json.NewDecoder(request.Body).Decode(&req)
	targetType := req.Type
	if targetType == "" {
		targetType = "tv"
	}

	results := ScanNetworkTargets(targetType)
	server.writeJSON(writer, map[string]interface{}{
		"ok":      true,
		"type":    targetType,
		"targets": results,
	}, http.StatusOK)
}

func main() {
	port := 8888
	if envPort := os.Getenv("HUB_PORT"); envPort != "" {
		if parsed, err := strconv.Atoi(envPort); err == nil {
			port = parsed
		}
	}

	hub := NewHubServer(port)
	if err := hub.Start(); err != nil {
		fmt.Printf("Hub server stopped: %v\n", err)
		os.Exit(1)
	}
}
