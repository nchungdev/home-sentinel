package main

import (
	"encoding/json"
	"net/http"

	"github.com/nchungdev/home-sentinel/internal/llm"
)

func (s *Server) handleLLMAuth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	report, err := s.LLMManager.AuthReport(
		r.Context(), r.URL.Query().Get("engine_id"), r.URL.Query().Get("method"),
	)
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true, "auth": report})
}

func (s *Server) handleLLMLogin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodGet {
		writeJSON(w, map[string]any{
			"ok": true, "login": s.LLMManager.LoginStatus(r.URL.Query().Get("engine_id")),
		})
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var request llm.LoginRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&request); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if request.Action == "cancel" {
		s.LLMManager.CancelLogin(request.EngineID)
		writeJSON(w, map[string]any{"ok": true, "login": s.LLMManager.LoginStatus(request.EngineID)})
		return
	}
	if request.Key != "" {
		err := s.LLMManager.NavigateLogin(request.EngineID, request.Key)
		if err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true})
		return
	}
	if request.Code != "" {
		err := s.LLMManager.SubmitLoginCode(request.EngineID, request.Code)
		if err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true, "login": s.LLMManager.LoginStatus(request.EngineID)})
		return
	}
	state, err := s.LLMManager.BeginLogin(r.Context(), request)
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true, "login": state})
}
