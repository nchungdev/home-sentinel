package main

import (
	"encoding/json"
	"github.com/nchungdev/home-sentinel/internal/llm"
	"net/http"
	"os"
	"strings"
)

type haConfigFile struct {
	URL   string `json:"url"`
	Token string `json:"token"`
}

func loadHAConfig(path string) haConfigFile {
	var cfg haConfigFile
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &cfg)
	}
	return cfg
}

func (s *Server) handleHAConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	configPath := "sentinel_ha_config.json"

	if r.Method == http.MethodGet {
		cfg := loadHAConfig(configPath)
		if cfg.URL == "" {
			cfg.URL = "http://127.0.0.1:8123"
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ok": true,
			"config": map[string]interface{}{
				"url":       cfg.URL,
				"token":     "",
				"token_set": cfg.Token != "",
			},
		})
		return
	}

	if r.Method == http.MethodPost {
		var cfg haConfigFile
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if cfg.Token == "" {
			cfg.Token = loadHAConfig(configPath).Token
		}

		s.HAClient.UpdateConfig(cfg.URL, cfg.Token)
		data, _ := json.MarshalIndent(cfg, "", "  ")
		_ = os.WriteFile(configPath, data, 0600)

		ok, msg := s.HAClient.TestConnection()
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":      ok,
			"message": msg,
		})
		return
	}

	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handleHAEntities(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if s.HAClient == nil {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "entities": []interface{}{}})
		return
	}

	states, err := s.HAClient.GetStates()
	if err != nil {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":       false,
			"error":    err.Error(),
			"entities": []interface{}{},
		})
		return
	}

	type DeviceItem struct {
		EntityID string `json:"entity_id"`
		Name     string `json:"name"`
		Domain   string `json:"domain"`
		State    string `json:"state"`
	}

	allowed := map[string]bool{
		"light": true, "fan": true, "script": true, "switch": true, "climate": true,
	}

	var list []DeviceItem
	for _, st := range states {
		if st.State == "unavailable" || strings.Contains(st.EntityID, "_2_2") {
			continue
		}
		parts := strings.SplitN(st.EntityID, ".", 2)
		if len(parts) != 2 || !allowed[parts[0]] {
			continue
		}
		name := st.EntityID
		if fn, ok := st.Attributes["friendly_name"].(string); ok && strings.TrimSpace(fn) != "" {
			name = strings.TrimSpace(fn)
		}
		list = append(list, DeviceItem{
			EntityID: st.EntityID,
			Name:     name,
			Domain:   parts[0],
			State:    st.State,
		})
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":       true,
		"entities": list,
	})
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
			Config   json.RawMessage `json:"config"`
			TestRule string          `json:"test_rule"`
		}
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if s.AutomationMgr != nil && cfg.TestRule != "" {
			affected := s.AutomationMgr.RunRuleNow(cfg.TestRule)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":       true,
				"affected": affected,
			})
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
		response := map[string]interface{}{"ok": true, "engines": engines,
			"config": s.LLMManager.Settings()}
		if usage, used := s.LLMManager.LastUsage(); used {
			response["last_used"] = usage
		}
		_ = json.NewEncoder(w).Encode(response)
		return
	}

	if r.Method == http.MethodPost {
		var payload llm.Settings
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}

		err := s.LLMManager.SaveSettings(payload)
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

	var settings llm.Settings
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	ok, desc := s.LLMManager.TestSettings(r.Context(), settings)

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      ok,
		"message": desc,
	})
}
