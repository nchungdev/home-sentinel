package main

import (
	"encoding/json"
	"errors"
	"net/http"
)

func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed")
		return
	}

	var payload struct {
		Token string `json:"token"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_request_body")
		return
	}
	if !s.Auth.Matches(payload.Token) {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if s.Auth.Required() {
		setSessionCookie(w, payload.Token)
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	clearSessionCookie(w)
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{
		"required":   s.Auth.Required(),
		"authorized": s.Auth.Authorized(r),
	})
}

func (s *Server) handleGenerateToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed")
		return
	}

	token, err := s.Auth.Rotate()
	if errors.Is(err, errTokenFromEnv) {
		writeJSONError(w, http.StatusConflict, "token_managed_by_env")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "token_generation_failed")
		return
	}

	setSessionCookie(w, token)
	writeJSON(w, map[string]any{"ok": true, "token": token})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "" && !s.Auth.Authorized(r) {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	writeJSON(w, map[string]any{
		"service": "sentinel-server-go",
		"status":  "ok",
		"token":   s.Auth.Required(),
	})
}

func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(payload)
}
