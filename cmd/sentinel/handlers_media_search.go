package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/nchungdev/home-sentinel/internal/contracts"
	"github.com/nchungdev/home-sentinel/internal/mediasearch"
)

func (s *Server) handleMediaSources(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "sources": s.MediaSearch.Discover(r.Context(), r.URL.Query().Get("refresh") == "1")})
}

func (s *Server) handleMediaSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(query) > 300 {
		http.Error(w, "query too long", 400)
		return
	}
	if r.URL.Query().Get("format") == "json" {
		writeJSON(w, map[string]any{"ok": true, "result": s.MediaSearch.Search(r.Context(), query)})
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", 500)
		return
	}
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	encoder := json.NewEncoder(w)
	ctx = contracts.WithMediaSearchEmitter(ctx, func(event contracts.MediaSearchEvent) {
		if err := encoder.Encode(event); err != nil {
			cancel()
			return
		}
		flusher.Flush()
	})
	s.MediaSearch.Search(ctx, query)
}

func (s *Server) handleMediaBrowse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	items, err := s.MediaSearch.Browse(r.Context(), r.URL.Query().Get("source_id"), r.URL.Query().Get("item_id"))
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true, "items": items})
}

type mediaRequest struct {
	SourceID string `json:"source_id"`
	ItemID   string `json:"item_id"`
	Seasons  []int  `json:"seasons"`
}

func (s *Server) handleMediaRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var request mediaRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&request); err != nil {
		http.Error(w, "bad json", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	item, err := s.MediaSearch.Request(ctx, request.SourceID, request.ItemID,
		mediasearch.RequestOptions{Seasons: request.Seasons})
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true, "item": item})
}
