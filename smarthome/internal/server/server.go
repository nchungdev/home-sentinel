package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"smarthome/internal/gateway"
	"smarthome/internal/model"
)

type Server struct {
	gatewayClient *gateway.Client
	listenAddress string
	httpServer    *http.Server
}

type healthResponse struct {
	Status        string `json:"status"`
	GatewayOnline bool   `json:"gateway_online"`
	DeviceCount   int    `json:"device_count"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type successResponse struct {
	Success bool         `json:"success"`
	Device  model.Device `json:"device"`
}

func NewServer(listenAddress string, gatewayClient *gateway.Client) *Server {
	serverInstance := &Server{
		gatewayClient: gatewayClient,
		listenAddress: listenAddress,
	}

	routerMux := http.NewServeMux()
	routerMux.HandleFunc("/health", serverInstance.handleHealth)
	routerMux.HandleFunc("/api/sync", serverInstance.handleSync)
	routerMux.HandleFunc("/api/devices", serverInstance.handleDevices)
	routerMux.HandleFunc("/api/devices/", serverInstance.handleDeviceDetail)

	serverInstance.httpServer = &http.Server{
		Addr:    listenAddress,
		Handler: enableCORS(routerMux),
	}

	return serverInstance
}

func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Close() error {
	return s.httpServer.Close()
}

func (s *Server) handleHealth(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeJSON(writer, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}

	devicesList := s.gatewayClient.Devices()
	writeJSON(writer, http.StatusOK, healthResponse{
		Status:        "ok",
		GatewayOnline: true,
		DeviceCount:   len(devicesList),
	})
}

func (s *Server) handleSync(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writeJSON(writer, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}

	if errorSync := s.gatewayClient.Sync(request.Context()); errorSync != nil {
		writeJSON(writer, http.StatusBadGateway, errorResponse{Error: errorSync.Error()})
		return
	}

	writeJSON(writer, http.StatusOK, map[string]string{"status": "synchronized"})
}

func (s *Server) handleDevices(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeJSON(writer, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}

	writeJSON(writer, http.StatusOK, s.gatewayClient.Devices())
}

func (s *Server) handleDeviceDetail(writer http.ResponseWriter, request *http.Request) {
	pathRemainder := strings.TrimPrefix(request.URL.Path, "/api/devices/")
	pathTokens := strings.Split(strings.Trim(pathRemainder, "/"), "/")

	if len(pathTokens) == 0 || pathTokens[0] == "" {
		writeJSON(writer, http.StatusBadRequest, errorResponse{Error: "missing device id"})
		return
	}

	deviceID := pathTokens[0]

	if len(pathTokens) == 1 {
		if request.Method != http.MethodGet {
			writeJSON(writer, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
			return
		}

		targetDevice, exists := s.gatewayClient.Device(deviceID)
		if !exists {
			writeJSON(writer, http.StatusNotFound, errorResponse{Error: "device not found"})
			return
		}

		writeJSON(writer, http.StatusOK, targetDevice)
		return
	}

	subAction := pathTokens[1]
	if request.Method != http.MethodPost {
		writeJSON(writer, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}

	switch subAction {
	case "toggle":
		if errorToggle := s.gatewayClient.Toggle(request.Context(), deviceID); errorToggle != nil {
			statusResponse := http.StatusInternalServerError
			if errors.Is(errorToggle, gateway.ErrCriticalDevice) {
				statusResponse = http.StatusForbidden
			} else if errors.Is(errorToggle, gateway.ErrDeviceNotFound) {
				statusResponse = http.StatusNotFound
			}
			writeJSON(writer, statusResponse, errorResponse{Error: errorToggle.Error()})
			return
		}

		targetDevice, _ := s.gatewayClient.Device(deviceID)
		writeJSON(writer, http.StatusOK, successResponse{Success: true, Device: targetDevice})

	case "state":
		var commandPayload model.DeviceCommand
		if errorDecode := json.NewDecoder(request.Body).Decode(&commandPayload); errorDecode != nil {
			writeJSON(writer, http.StatusBadRequest, errorResponse{Error: "invalid json body"})
			return
		}

		if errorSet := s.gatewayClient.SetState(
			request.Context(),
			deviceID,
			commandPayload.State,
		); errorSet != nil {
			statusResponse := http.StatusInternalServerError
			if errors.Is(errorSet, gateway.ErrCriticalDevice) {
				statusResponse = http.StatusForbidden
			} else if errors.Is(errorSet, gateway.ErrDeviceNotFound) {
				statusResponse = http.StatusNotFound
			} else if errors.Is(errorSet, gateway.ErrInvalidState) {
				statusResponse = http.StatusBadRequest
			}
			writeJSON(writer, statusResponse, errorResponse{Error: errorSet.Error()})
			return
		}

		targetDevice, _ := s.gatewayClient.Device(deviceID)
		writeJSON(writer, http.StatusOK, successResponse{Success: true, Device: targetDevice})

	default:
		writeJSON(writer, http.StatusNotFound, errorResponse{Error: "unknown sub-action"})
	}
}

func enableCORS(nextHandler http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Access-Control-Allow-Origin", "*")
		writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if request.Method == http.MethodOptions {
			writer.WriteHeader(http.StatusNoContent)
			return
		}

		nextHandler.ServeHTTP(writer, request)
	})
}

func writeJSON(writer http.ResponseWriter, statusCode int, payload any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(statusCode)
	_ = json.NewEncoder(writer).Encode(payload)
}
