package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/nchungdev/home-sentinel/internal/integrations/haclient"
	"github.com/nchungdev/home-sentinel/internal/tvmodes"
)

const (
	tvModesFile     = "sentinel_tv_modes.json"
	tvModeEventType = "tv_mode"
	tvModeBodyLimit = 64 << 10
)

var (
	tvModesOnce   sync.Once
	tvModesEngine *tvmodes.Engine
	tvModesStore  *tvmodes.Store
)

type haServices struct{ client *haclient.Client }

func (h haServices) CallService(domain, service string, data map[string]any) error {
	if h.client == nil || h.client.BaseURL == "" {
		return errors.New("home assistant is not configured")
	}
	return h.client.RequestJSON(fmt.Sprintf("/api/services/%s/%s", domain, service), data, nil)
}

func (s *Server) tvModes() (*tvmodes.Engine, *tvmodes.Store) {
	tvModesOnce.Do(func() {
		tvModesStore = tvmodes.NewStore(tvModesFile)
		tvModesEngine = tvmodes.NewEngine(tvModesStore, haServices{s.HAClient}, s.broadcastTVMode)
	})
	return tvModesEngine, tvModesStore
}

func (s *Server) broadcastTVMode(state tvmodes.State) {
	s.sendDuinchPayload(&DuinchWsEvent{
		Type:       tvModeEventType,
		TargetID:   state.Active,
		Parameters: map[string]any{"active": state.Active},
		Timestamp:  time.Now().UnixMilli(),
	}, false)
}

func (s *Server) handleTVModes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed")
		return
	}
	engine, _ := s.tvModes()
	writeJSON(w, engine.State())
}

func (s *Server) handleTVModeActivate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed")
		return
	}
	engine, _ := s.tvModes()
	state, err := engine.Activate(r.PathValue("id"))
	if errors.Is(err, tvmodes.ErrUnknownMode) {
		writeJSONError(w, http.StatusNotFound, "unknown_mode")
		return
	}
	if err != nil && state.Modes == nil {
		writeJSONError(w, http.StatusBadGateway, "home_assistant_failed")
		return
	}
	response := map[string]any{"active": state.Active, "modes": state.Modes}
	if err != nil {
		response["warning"] = err.Error()
	}
	writeJSON(w, response)
}

func (s *Server) handleTVModesConfig(w http.ResponseWriter, r *http.Request) {
	_, store := s.tvModes()
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, store.Get())
	case http.MethodPut:
		var next tvmodes.Config
		r.Body = http.MaxBytesReader(w, r.Body, tvModeBodyLimit)
		if err := json.NewDecoder(r.Body).Decode(&next); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_request_body")
			return
		}
		if err := store.Replace(next); err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		engine, _ := s.tvModes()
		s.broadcastTVMode(engine.State())
		writeJSON(w, map[string]any{"ok": true, "config": store.Get()})
	default:
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed")
	}
}
