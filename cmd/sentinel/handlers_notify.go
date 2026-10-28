package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func (s *Server) handleApiNotify(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Source     string         `json:"source"`
		Title      string         `json:"title"`
		Message    string         `json:"message"`
		Severity   string         `json:"severity"`
		ActionType string         `json:"action_type"`
		Parameters map[string]any `json:"parameters"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Message == "" {
		req.Message = req.Title
	}
	if req.Title == "" {
		req.Title = "Thông báo"
	}
	if req.Source == "" {
		req.Source = "nas"
	}
	var action *DuinchActionPayload
	if req.ActionType != "" {
		action = &DuinchActionPayload{
			ActionType: req.ActionType,
			Parameters: req.Parameters,
		}
	}
	notif := &DuinchNotificationPayload{
		ID:        fmt.Sprintf("notif-%d", time.Now().UnixNano()),
		Source:    req.Source,
		Title:     req.Title,
		Message:   req.Message,
		Severity:  req.Severity,
		Action:    action,
		Timestamp: time.Now().UnixMilli(),
	}
	sent := s.BroadcastDuinchNotification(notif)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":       "ok",
		"id":           notif.ID,
		"clients_sent": sent,
	})
}

func (s *Server) handleRadarrWebhook(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var payload struct {
		EventType string `json:"eventType"`
		Movie     struct {
			Title string `json:"title"`
			Year  int    `json:"year"`
		} `json:"movie"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	msg := fmt.Sprintf("Phim %s (%d) đã tải xong và sẵn sàng xem!",
		payload.Movie.Title, payload.Movie.Year)
	if payload.EventType != "" && payload.EventType != "Download" {
		msg = fmt.Sprintf("Radarr [%s]: %s", payload.EventType, payload.Movie.Title)
	}
	notif := &DuinchNotificationPayload{
		ID:        fmt.Sprintf("radarr-%d", time.Now().UnixNano()),
		Source:    "radarr",
		Title:     "Radarr - Tải phim xong",
		Message:   msg,
		Severity:  "success",
		Timestamp: time.Now().UnixMilli(),
	}
	sent := s.BroadcastDuinchNotification(notif)
	_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "clients_sent": sent})
}

func (s *Server) handleSonarrWebhook(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var payload struct {
		EventType string `json:"eventType"`
		Series    struct {
			Title string `json:"title"`
		} `json:"series"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	msg := fmt.Sprintf("Phim bộ %s đã có tập mới sẵn sàng trên NAS!", payload.Series.Title)
	notif := &DuinchNotificationPayload{
		ID:        fmt.Sprintf("sonarr-%d", time.Now().UnixNano()),
		Source:    "sonarr",
		Title:     "Sonarr - Tập mới",
		Message:   msg,
		Severity:  "success",
		Timestamp: time.Now().UnixMilli(),
	}
	sent := s.BroadcastDuinchNotification(notif)
	_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "clients_sent": sent})
}
