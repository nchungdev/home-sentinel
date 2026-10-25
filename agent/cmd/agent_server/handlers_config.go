package main

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
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
