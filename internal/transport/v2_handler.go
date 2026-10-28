package transport

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/nchungdev/home-sentinel/internal/contracts"
	"github.com/nchungdev/home-sentinel/internal/orchestrator"
)

type V2EventType string

const (
	V2EventAccepted     V2EventType = "accepted"
	V2EventProgress     V2EventType = "progress"
	V2EventPresentation V2EventType = "presentation"
	V2EventClientAction V2EventType = "client_action"
	V2EventCompleted    V2EventType = "completed"
	V2EventError        V2EventType = "error"
)

type V2StreamEvent struct {
	Type      V2EventType `json:"type"`
	Timestamp int64       `json:"timestamp"`
	Payload   any         `json:"payload"`
}

type V2Handler struct {
	orchestrator *orchestrator.Orchestrator
	config       *Config
}

func NewV2Handler(orch *orchestrator.Orchestrator, cfg *Config) *V2Handler {
	return &V2Handler{
		orchestrator: orch,
		config:       cfg,
	}
}

func (h *V2Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	if !IsIPAllowed(r.RemoteAddr, h.config.AllowedIPs) {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	authHeader := r.Header.Get(HeaderAuthorization)
	if !VerifyBearerToken(authHeader, h.config.Token) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, `{"error":"streaming_unsupported"}`, http.StatusInternalServerError)
		return
	}

	maxBytes := h.config.MaxBodyBytes
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBodyBytes
	}

	input, err := ParseInputEvent(r, maxBytes)
	if err != nil {
		http.Error(w, `{"error":"invalid_request_body"}`, http.StatusBadRequest)
		return
	}

	reqCtx, ctx, cancel := ExtractRequestContext(r, h.config.DefaultTimeout)
	defer cancel()

	w.Header().Set(HeaderContentType, "application/x-ndjson; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	sendEvent := func(eventType V2EventType, payload any) error {
		event := V2StreamEvent{
			Type:      eventType,
			Timestamp: time.Now().UnixMilli(),
			Payload:   payload,
		}
		data, marshalErr := json.Marshal(event)
		if marshalErr != nil {
			return marshalErr
		}
		if _, writeErr := fmt.Fprintf(w, "%s\n", data); writeErr != nil {
			return writeErr
		}
		flusher.Flush()
		return nil
	}

	_ = sendEvent(V2EventAccepted, map[string]any{
		"request_id": input.RequestID,
		"session_id": input.SessionID,
	})

	_ = sendEvent(V2EventProgress, map[string]any{
		"stage": "processing",
	})

	ctx = contracts.WithMediaSearchEmitter(ctx, func(event contracts.MediaSearchEvent) {
		_ = sendEvent(V2EventType("media_search"), event)
		if event.Type == "source_results" && event.Result.Searched {
			_ = sendEvent(V2EventPresentation, contracts.MediaPresentation(event.Snapshot, false))
		}
	})
	resp, err := h.orchestrator.Process(ctx, input, reqCtx)
	if err != nil {
		_ = sendEvent(V2EventError, map[string]any{
			"code":    "pipeline_error",
			"message": err.Error(),
		})
		return
	}

	_ = sendEvent(V2EventPresentation, resp)

	for _, action := range resp.ClientActions {
		_ = sendEvent(V2EventClientAction, action)
	}

	_ = sendEvent(V2EventCompleted, map[string]any{
		"request_id": input.RequestID,
		"status":     "success",
	})
}
