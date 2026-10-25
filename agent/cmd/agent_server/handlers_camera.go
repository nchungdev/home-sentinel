package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/nchungdev/home-sentinel/agent/pkg/camstream"
	"github.com/nchungdev/home-sentinel/agent/pkg/onvifscan"
)

type CameraConfigData struct {
	IP            string `json:"ip"`
	Port          int    `json:"port"`
	Username      string `json:"username"`
	Password      string `json:"password"`
	CloudPassword string `json:"cloud_password,omitempty"`
	StreamPath    string `json:"stream_path"`
}

func (s *Server) handleScanCameras(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	devices, err := onvifscan.ScanDevices(3 * time.Second)
	if err != nil {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":    false,
			"error": err.Error(),
		})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"devices": devices,
	})
}

func (s *Server) handleCameraConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	configPath := "sentinel_camera_config.json"

	if r.Method == http.MethodGet {
		var cfg CameraConfigData
		if data, err := os.ReadFile(configPath); err == nil {
			_ = json.Unmarshal(data, &cfg)
		}
		if cfg.Port == 0 {
			cfg.Port = 2020
		}
		if cfg.StreamPath == "" {
			cfg.StreamPath = "/stream1"
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
		if cfg.Port == 0 {
			cfg.Port = 2020
		}
		if cfg.StreamPath == "" {
			cfg.StreamPath = "/stream1"
		}
		data, _ := json.MarshalIndent(cfg, "", "  ")
		_ = os.WriteFile(configPath, data, 0644)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":      true,
			"message": "Cấu hình camera đã được lưu",
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
		Username string `json:"username"`
		Password string `json:"password"`
		Path     string `json:"path"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Port == 0 {
		req.Port = 554
	}
	if req.Path == "" {
		req.Path = "/stream1"
	}

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", req.IP, req.Port), 3*time.Second)
	if err != nil {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":    false,
			"error": fmt.Sprintf("Không thể kết nối cổng %d: %v", req.Port, err),
		})
		return
	}
	defer conn.Close()

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"message": "Kết nối thành công tới RTSP port của Camera!",
	})
}

func (s *Server) handleStreamMjpeg(w http.ResponseWriter, r *http.Request) {
	var cfg CameraConfigData
	if data, err := os.ReadFile("sentinel_camera_config.json"); err == nil {
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
