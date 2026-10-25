package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func (s *Server) handleClientLogs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.LogsMutex.RLock()
	defer s.LogsMutex.RUnlock()
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":   true,
		"logs": s.ClientLogs,
	})
}

func (s *Server) handleGenerateToken(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		http.Error(w, "error generating token", http.StatusInternalServerError)
		return
	}
	token := hex.EncodeToString(bytes)
	s.Token = token
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":    true,
		"token": token,
	})
}

func (s *Server) handleCameraEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	eventChan := make(chan string, 16)
	s.eventSubMutex.Lock()
	s.eventSubscribers[eventChan] = struct{}{}
	s.eventSubMutex.Unlock()

	defer func() {
		s.eventSubMutex.Lock()
		delete(s.eventSubscribers, eventChan)
		close(eventChan)
		s.eventSubMutex.Unlock()
	}()

	notify := r.Context().Done()
	for {
		select {
		case <-notify:
			return
		case msg, ok := <-eventChan:
			if !ok {
				return
			}
			_, _ = fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		}
	}
}

func (s *Server) handleNotify(w http.ResponseWriter, r *http.Request) {
	var payload map[string]interface{}
	_ = json.NewDecoder(r.Body).Decode(&payload)
	_ = s.BroadcastNotification(payload)

	nowStr := time.Now().Format("15:04:05")
	clientIP := r.RemoteAddr
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		clientIP = strings.Split(fwd, ",")[0]
	}
	summary := "Nhận sự kiện notification"
	terminalLogMsg := ""
	if src, ok := payload["source"].(string); ok && src == "home_assistant" {
		fn, _ := payload["friendly_name"].(string)
		et, _ := payload["event_type"].(string)
		st, _ := payload["state"].(string)
		eid, _ := payload["entity_id"].(string)
		summary = fmt.Sprintf("📡 HA Event [%s]: %s (trạng thái: %s)", et, fn, st)
		if fn == "" {
			fn = "Camera Phòng Khách"
		}
		terminalLogMsg = fmt.Sprintf("[%s] 🚨 CẢNH BÁO: %s - Phát hiện người/chuyển động! (trạng thái: %s)", nowStr, fn, st)

		isPerson := strings.Contains(eid, "person") || et == "person_detected"
		isPetOrMotion := strings.Contains(eid, "motion") || strings.Contains(eid, "pet")

		if isPerson && st == "on" {
			if s.AutomationMgr != nil {
				s.AutomationMgr.RecordPersonPresence()
				go s.AutomationMgr.HandleEveningArrival("phát hiện người")
			}
			go s.handleProactiveSofaPrompt()
		} else if isPetOrMotion && st == "on" {
			terminalLogMsg = fmt.Sprintf("[%s] 🐾 Camera: %s - Bỏ qua chuyển động vật nuôi/môi trường (%s)", nowStr, fn, eid)
		}
	} else if ev, ok := payload["event"].(string); ok {
		summary = fmt.Sprintf("📡 Event: %s", ev)
		terminalLogMsg = fmt.Sprintf("[%s] 🔔 Event: %s", nowStr, ev)
	}

	s.AddClientLog(ClientLogEntry{
		Timestamp: nowStr,
		ClientIP:  clientIP,
		Session:   "system_notify",
		Text:      summary,
		Source:    "ha_webhook",
		Response:  "Đã tiếp nhận và broadcast qua UDP :8790",
	})

	if terminalLogMsg != "" {
		s.broadcastTerminal(terminalLogMsg)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
