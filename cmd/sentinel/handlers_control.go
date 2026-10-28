package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/nchungdev/home-sentinel/internal/integrations/camstream"
)

func (s *Server) handleCameraPTZ(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	var cfg CameraConfigData
	if data, err := os.ReadFile("sentinel_camera_config.json"); err == nil {
		_ = json.Unmarshal(data, &cfg)
	}

	effectivePass := cfg.CloudPassword
	if effectivePass == "" {
		effectivePass = cfg.Password
	}

	if cfg.IP == "" || effectivePass == "" {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": "Chưa cấu hình Camera IP hoặc Mật khẩu"})
		return
	}

	session := camstream.GetTapoSession(cfg.IP, cfg.Username, effectivePass)

	if r.Method == http.MethodGet {
		presets, err := session.GetPresets()
		if err != nil {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "presets": presets})
		return
	}

	if r.Method == http.MethodPost {
		var req struct {
			Direction string `json:"direction"`
			Step      int    `json:"step"`
			Preset    string `json:"preset"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		var err error
		if req.Preset != "" {
			err = session.GotoPreset(req.Preset)
		} else if req.Direction != "" {
			err = session.MoveDirection(req.Direction, req.Step)
		} else {
			err = fmt.Errorf("thiếu hướng xoay hoặc preset")
		}

		if err != nil {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "message": "Thao tác PTZ thành công"})
		return
	}

	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
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
	_ = json.NewDecoder(r.Body).Decode(&req)
	text := req.Text
	if text == "" {
		text = "Xin chào, tôi là trợ lý ảo Duinch"
	}

	if err := s.speakOnCamera(text, req.Voice); err != nil {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":    false,
			"error": fmt.Sprintf("Không thể phát loa camera: %v", err),
		})
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"message": fmt.Sprintf("Đã phát giọng nói lên loa Camera thành công: \"%s\"", text),
	})
}
