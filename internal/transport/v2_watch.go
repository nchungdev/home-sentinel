package transport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/nchungdev/home-sentinel/internal/watching"
)

const maxWatchEventsBody = 64 << 10

type WatchRecorder interface {
	Record(events []watching.Event) error
}

type ContinueLister interface {
	Items(ctx context.Context) []watching.Item
}

type V2WatchEventsHandler struct {
	recorder WatchRecorder
	config   *Config
}

func NewV2WatchEventsHandler(recorder WatchRecorder, cfg *Config) *V2WatchEventsHandler {
	return &V2WatchEventsHandler{recorder: recorder, config: cfg}
}

func (h *V2WatchEventsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	if !VerifyBearerToken(r.Header.Get(HeaderAuthorization), h.config.Token) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var request struct {
		Events []watching.Event `json:"events"`
	}
	body := http.MaxBytesReader(w, r.Body, maxWatchEventsBody)
	if err := json.NewDecoder(body).Decode(&request); err != nil {
		http.Error(w, `{"error":"invalid_json"}`, http.StatusBadRequest)
		return
	}

	err := h.recorder.Record(request.Events)
	if errors.Is(err, watching.ErrNoValidEvents) {
		http.Error(w, `{"error":"no_valid_events"}`, http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, `{"error":"store_failed"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type V2WatchContinueHandler struct {
	lister ContinueLister
	config *Config
}

func NewV2WatchContinueHandler(lister ContinueLister, cfg *Config) *V2WatchContinueHandler {
	return &V2WatchContinueHandler{lister: lister, config: cfg}
}

func (h *V2WatchContinueHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	if !VerifyBearerToken(r.Header.Get(HeaderAuthorization), h.config.Token) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	_, ctx, cancel := ExtractRequestContext(r, h.config.DefaultTimeout)
	defer cancel()

	items := h.lister.Items(ctx)
	if items == nil {
		items = []watching.Item{}
	}
	w.Header().Set(HeaderContentType, "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
}
