package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/nchungdev/home-sentinel/internal/contracts"
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
	w.Header().Set("Cache-Control", "no-store")

	isStream := r.URL.Query().Get("stream") == "1" ||
		strings.Contains(r.Header.Get("Accept"), "application/x-ndjson")

	if isStream {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
		w.Header().Set("X-Accel-Buffering", "no")
		encoder := json.NewEncoder(w)

		_ = encoder.Encode(map[string]any{"type": "media", "media": title})
		flusher.Flush()

		s.MediaSearch.SearchMatchingStream(
			ctx, title.Title, title.Kind, title.Year, title.Aliases, title.ExternalIDs,
			func(source contracts.MediaSourceResults) {
				if ctx.Err() == nil {
					s.populateSourceCatalogData(ctx, &source)
					_ = encoder.Encode(map[string]any{"type": "source", "source": source})
					flusher.Flush()
				}
			},
		)

		_ = encoder.Encode(map[string]any{"type": "completed"})
		flusher.Flush()
		return
	}

	sources := s.MediaSearch.SearchMatching(
		ctx, title.Title, title.Kind, title.Year, title.Aliases, title.ExternalIDs,
	)
	for i := range sources {
		s.populateSourceCatalogData(ctx, &sources[i])
	}
	writeJSON(w, tmdbDetailResponse{Media: title, Sources: sources})
}

type tmdbDetailResponse struct {
	Media   any                            `json:"media"`
	Sources []contracts.MediaSourceResults `json:"sources"`
}

func (s *Server) populateSourceCatalogData(ctx context.Context, source *contracts.MediaSourceResults) {
	if source == nil || len(source.Items) == 0 {
		return
	}
	for index := range source.Items {
		item := &source.Items[index]
		if !strings.HasPrefix(item.ID, "phimapi:") &&
			!strings.HasPrefix(item.ID, "ophim:") &&
			!strings.HasPrefix(item.ID, "nguonc:") &&
			!strings.HasPrefix(item.ID, "motchill:") &&
			!strings.HasPrefix(item.ID, "rophim:") &&
			!strings.HasPrefix(item.ID, "phimtuoitho:") {
			continue
		}

		catTitle, err := s.catalog().GetTitle(ctx, item.ID)
		if err != nil || len(catTitle.Episodes) == 0 {
			continue
		}

		if item.Kind == "show" || len(catTitle.Episodes) > 1 {
			episodes := make([]contracts.CatalogEpisode, 0, len(catTitle.Episodes))
			for _, ep := range catTitle.Episodes {
				episodes = append(episodes, contracts.CatalogEpisode{
					ID:       ep.ID,
					TitleID:  ep.TitleID,
					Label:    ep.Label,
					Group:    ep.Group,
					Order:    ep.Order,
					SeasonID: ep.SeasonID,
					Playable: ep.Playable,
				})
			}
			item.Episodes = episodes
			item.Actions = nil
		} else if len(catTitle.Episodes) == 1 && s.Playback != nil {
			ep := catTitle.Episodes[0]
			stream, err := s.catalog().ResolveEpisode(ctx, ep.ID)
			if err == nil && stream.URL != "" {
				endpoint, headers, err := s.Playback.Resolver.Resolve(ctx, stream.URL)
				if err == nil {
					descriptor, err := s.Playback.Create(endpoint, stream.Title+" · "+ep.Label, "", headers, false)
					if err == nil {
						item.Playback = descriptor
						item.Actions = nil
					}
				}
			}
		}
	}
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
