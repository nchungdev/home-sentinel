package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/nchungdev/home-sentinel/internal/skills"
)

func (s *Server) handleTMDBMediaDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		writeJSON(w, map[string]string{"error": "invalid_media_id"})
		return
	}
	if s.MediaSearch == nil {
		writeJSON(w, map[string]string{"error": "media_sources_unavailable"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	title, code := newMetadataSkill().Detail(ctx, r.PathValue("kind"), id)
	if code != "" {
		writeTMDBDetailError(w, code)
		return
	}
	sources := s.MediaSearch.SearchMatching(
		ctx, title.Title, title.Kind, title.Year, title.Aliases, title.ExternalIDs,
	)
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, map[string]any{"title": title, "sources": sources})
}

func (s *Server) handleTMDBMediaDiscovery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "invalid_media_id"})
		return
	}
	if s.MediaSearch == nil || s.LLMManager == nil ||
		s.LLMManager.Settings().SelectedEngine == "rule_fallback" {
		writeJSONStatus(w, http.StatusServiceUnavailable,
			map[string]string{"error": "ai_search_unavailable"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 65*time.Second)
	defer cancel()
	detail, code := newMetadataSkill().Detail(ctx, r.PathValue("kind"), id)
	if code != "" {
		writeTMDBDetailError(w, code)
		return
	}
	source, code := (skills.MovieSearchDiscovery{
		Assistant: s.LLMManager, Searcher: s.MediaSearch,
	}).Search(ctx, detail)
	if code != "" {
		writeTMDBDiscoveryError(w, code)
		return
	}
	source.DiscoveryMethod = "ai_query_expansion"
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, map[string]any{"source": source, "method": "ai_query_expansion"})
}

func writeTMDBDiscoveryError(w http.ResponseWriter, code string) {
	status := http.StatusBadGateway
	if code == "ai_search_unavailable" {
		status = http.StatusServiceUnavailable
	} else if code == "ai_search_timeout" {
		status = http.StatusGatewayTimeout
	}
	writeJSONStatus(w, status, map[string]string{"error": code})
}

func writeTMDBDetailError(w http.ResponseWriter, code string) {
	status := http.StatusBadGateway
	switch code {
	case "invalid_media_id":
		status = http.StatusBadRequest
	case "not_configured":
		status = http.StatusServiceUnavailable
	case "auth_required":
		status = http.StatusBadGateway
	case "provider_unavailable":
		status = http.StatusBadGateway
	case "invalid_source_response":
		status = http.StatusBadGateway
	}
	writeJSONStatus(w, status, map[string]string{"error": code})
}

func writeJSONStatus(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
