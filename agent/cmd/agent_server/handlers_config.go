package main

import (
	"encoding/json"
	"net/http"
	"os"
)

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
			cfg.URL = "http://127.0.0.1:8123"
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

func (s *Server) handleGroqConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	configPath := "sentinel_groq_config.json"

	if r.Method == http.MethodGet {
		var cfg struct {
			APIKey string `json:"api_key"`
		}
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
		var cfg struct {
			APIKey string `json:"api_key"`
		}
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if s.Transcriber != nil {
			s.Transcriber.UpdateGroqKey(cfg.APIKey)
		}
		data, _ := json.MarshalIndent(cfg, "", "  ")
		_ = os.WriteFile(configPath, data, 0644)

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":      true,
			"message": "Đã lưu cấu hình Groq Whisper API thành công",
		})
		return
	}

	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handleAutomationConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	if r.Method == http.MethodGet {
		if s.AutomationMgr != nil {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":     true,
				"config": s.AutomationMgr.GetConfig(),
			})
			return
		}
		http.Error(w, "automation manager not ready", http.StatusServiceUnavailable)
		return
	}

	if r.Method == http.MethodPost {
		var cfg struct {
			Config json.RawMessage `json:"config"`
		}
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if s.AutomationMgr != nil {
			cur := s.AutomationMgr.GetConfig()
			if err := json.Unmarshal(cfg.Config, &cur); err == nil {
				s.AutomationMgr.UpdateConfig(cur)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"ok":      true,
					"message": "Đã lưu kịch bản tự động hóa thành công",
					"config":  cur,
				})
				return
			}
		}
		http.Error(w, "cannot update config", http.StatusBadRequest)
		return
	}

	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
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
