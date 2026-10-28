package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type DuinchActionPayload struct {
	StepID       string         `json:"step_id,omitempty"`
	ActionType   string         `json:"action_type"`
	TargetID     string         `json:"target_id,omitempty"`
	Parameters   map[string]any `json:"parameters,omitempty"`
	FeedbackText string         `json:"feedback_text,omitempty"`
}

type DuinchNotificationPayload struct {
	ID        string               `json:"id"`
	Source    string               `json:"source"`
	Title     string               `json:"title"`
	Message   string               `json:"message"`
	Severity  string               `json:"severity,omitempty"`
	Action    *DuinchActionPayload `json:"action,omitempty"`
	Timestamp int64                `json:"timestamp"`
}

type DuinchWsEvent struct {
	Type         string                     `json:"type"`
	Text         string                     `json:"text,omitempty"`
	Mood         string                     `json:"mood,omitempty"`
	StepID       string                     `json:"step_id,omitempty"`
	ActionType   string                     `json:"action_type,omitempty"`
	TargetID     string                     `json:"target_id,omitempty"`
	Parameters   map[string]any             `json:"parameters,omitempty"`
	FeedbackText string                     `json:"feedback_text,omitempty"`
	Notification *DuinchNotificationPayload `json:"notification,omitempty"`
	Timestamp    int64                      `json:"timestamp"`
}

var (
	duinchWsUpgrader = websocket.Upgrader{
		ReadBufferSize:  2048,
		WriteBufferSize: 2048,
		CheckOrigin:     func(r *http.Request) bool { return true },
	}
	duinchClientsMu sync.Mutex
	duinchClients   = make(map[*websocket.Conn]struct{})
	lastDuinchEvent *DuinchWsEvent
	duinchPingOnce  sync.Once
)

func startDuinchPingLoop() {
	go func() {
		ticker := time.NewTicker(35 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			duinchClientsMu.Lock()
			for conn := range duinchClients {
				_ = conn.SetWriteDeadline(time.Now().Add(4 * time.Second))
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					_ = conn.Close()
					delete(duinchClients, conn)
				}
			}
			duinchClientsMu.Unlock()
		}
	}()
}

func (s *Server) handleDuinchWS(w http.ResponseWriter, r *http.Request) {
	duinchPingOnce.Do(startDuinchPingLoop)

	conn, err := duinchWsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	duinchClientsMu.Lock()
	duinchClients[conn] = struct{}{}
	current := lastDuinchEvent
	clientCount := len(duinchClients)
	if current != nil {
		_ = conn.SetWriteDeadline(time.Now().Add(4 * time.Second))
		_ = conn.WriteJSON(current)
	}
	duinchClientsMu.Unlock()

	s.broadcastTerminal(fmt.Sprintf(
		"[%s] 🤖 Duinch Bot TV đã kết nối WebSocket (%s, tổng: %d thiết bị)",
		time.Now().Format("15:04:05"), r.RemoteAddr, clientCount,
	))

	defer func() {
		duinchClientsMu.Lock()
		delete(duinchClients, conn)
		duinchClientsMu.Unlock()
		_ = conn.Close()
	}()

	conn.SetReadLimit(4096)
	_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		return nil
	})

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
		_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	}
}

func (s *Server) broadcastDuinchPayload(event *DuinchWsEvent) int {
	return s.sendDuinchPayload(event, true)
}

func (s *Server) sendDuinchPayload(event *DuinchWsEvent, replayLater bool) int {
	duinchClientsMu.Lock()
	if replayLater {
		lastDuinchEvent = event
	}
	sent := 0
	for conn := range duinchClients {
		_ = conn.SetWriteDeadline(time.Now().Add(4 * time.Second))
		if err := conn.WriteJSON(event); err != nil {
			_ = conn.Close()
			delete(duinchClients, conn)
		} else {
			sent++
		}
	}
	duinchClientsMu.Unlock()
	return sent
}

func (s *Server) BroadcastDuinchGreeting(text, mood string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	if mood == "" {
		mood = "happy"
	}
	payload := &DuinchWsEvent{
		Type:      "greeting",
		Text:      text,
		Mood:      mood,
		Timestamp: time.Now().UnixMilli(),
	}
	sent := s.broadcastDuinchPayload(payload)
	s.broadcastTerminal(fmt.Sprintf(
		"[%s] 💬 Push Duinch Greeting (%d TV): %q (mood=%s)",
		time.Now().Format("15:04:05"), sent, text, mood,
	))
	return sent
}

func (s *Server) BroadcastDuinchAction(
	stepID, actionType, targetID, feedbackText string,
	parameters map[string]any,
) int {
	payload := &DuinchWsEvent{
		Type:         "action",
		StepID:       stepID,
		ActionType:   actionType,
		TargetID:     targetID,
		Parameters:   parameters,
		FeedbackText: feedbackText,
		Timestamp:    time.Now().UnixMilli(),
	}
	sent := s.broadcastDuinchPayload(payload)
	s.broadcastTerminal(fmt.Sprintf(
		"[%s] ⚡ Push Duinch Action (%d TV): %s (%s)",
		time.Now().Format("15:04:05"), sent, actionType, feedbackText,
	))
	return sent
}

func (s *Server) BroadcastDuinchNotification(notif *DuinchNotificationPayload) int {
	if notif.ID == "" {
		notif.ID = fmt.Sprintf("notif-%d", time.Now().UnixNano())
	}
	if notif.Timestamp <= 0 {
		notif.Timestamp = time.Now().UnixMilli()
	}
	payload := &DuinchWsEvent{
		Type:         "notification",
		Text:         notif.Message,
		Notification: notif,
		Timestamp:    notif.Timestamp,
	}
	sent := s.broadcastDuinchPayload(payload)
	s.broadcastTerminal(fmt.Sprintf(
		"[%s] 🔔 Push Duinch Notification (%d TV): [%s] %s",
		time.Now().Format("15:04:05"), sent, notif.Source, notif.Title,
	))
	return sent
}

func (s *Server) handleDuinchGreeting(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if r.Method == http.MethodGet {
		duinchClientsMu.Lock()
		count := len(duinchClients)
		current := lastDuinchEvent
		duinchClientsMu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"connected_clients": count,
			"last_event":        current,
		})
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Greeting string `json:"greeting"`
		Text     string `json:"text"`
		Mood     string `json:"mood"`
		Generate bool   `json:"generate"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	msg := strings.TrimSpace(req.Greeting)
	if msg == "" {
		msg = strings.TrimSpace(req.Text)
	}
	if msg == "" && req.Generate && s.LLMManager != nil {
		hour := time.Now().Hour()
		period := "đêm khuya"
		topic := "khuyên nghỉ ngơi sớm giữ sức khỏe và chúc một giấc ngủ thật ngon"
		switch {
		case hour >= 5 && hour <= 10:
			period = "buổi sáng"
			topic = "chúc ngày mới tràn đầy năng lượng và khởi đầu thuận lợi"
		case hour >= 11 && hour <= 13:
			period = "buổi trưa"
			topic = "chúc bữa trưa ngon miệng và có thời gian nghỉ ngơi thư thái"
		case hour >= 14 && hour <= 17:
			period = "buổi chiều"
			topic = "gợi ý nghe nhạc nhẹ thư giãn cho buổi chiều dễ chịu"
		case hour >= 18 && hour <= 21:
			period = "buổi tối"
			topic = "chúc buổi tối ấm cúng bên gia đình và thưởng thức một bộ phim hay"
		}
		prompt := fmt.Sprintf(
			"Bạn là Duinch - trợ lý robot thông minh trên màn hình TV phòng khách. "+
				"Hãy viết 1 hoặc 2 câu chào ngắn gọn (12 đến 22 từ), tự nhiên, ấm áp cho %s, "+
				"trọng tâm duy nhất: %s. "+
				"Tuyệt đối không dùng từ 'hoặc' để ghép nhiều ý rời rạc, không dùng ngoặc kép.",
			period, topic,
		)
		var sb strings.Builder
		err := s.LLMManager.StreamChatContext(r.Context(), prompt, func(chunk string) {
			sb.WriteString(chunk)
		})
		if err != nil {
			http.Error(w, "greeting generation unavailable", http.StatusServiceUnavailable)
			return
		}
		msg = strings.Trim(strings.TrimSpace(sb.String()), "\"'“”")
	}
	if msg == "" {
		http.Error(w, "greeting text is required", http.StatusBadRequest)
		return
	}

	sent := s.BroadcastDuinchGreeting(msg, req.Mood)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":       "ok",
		"greeting":     msg,
		"mood":         req.Mood,
		"clients_sent": sent,
	})
}
