package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/nchungdev/home-sentinel/internal/catalog"
)

func (s *Server) handleCatalogTitle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	title, err := s.catalog().GetTitle(ctx, r.PathValue("id"))
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, map[string]any{"title": title})
}

func (s *Server) handleCatalogPlayback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.ContentLength != 0 {
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
		decoder.DisallowUnknownFields()
		if decoder.Decode(new(struct{})) != nil || decoder.Decode(new(any)) != io.EOF {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
	}
	if s.Playback == nil {
		writeCatalogError(w, catalog.ErrUnavailable)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	stream, err := s.catalog().ResolveEpisode(ctx, r.PathValue("id"))
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	endpoint, headers, err := s.Playback.Resolver.Resolve(ctx, stream.URL)
	if err != nil {
		writeCatalogError(w, catalog.ErrUnsupported)
		return
	}
	descriptor, err := s.Playback.Create(endpoint,
		stream.Title+" · "+stream.Episode.Label, "", headers, false)
	if err != nil {
		writeCatalogError(w, catalog.ErrUnavailable)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, map[string]any{"episode": stream.Episode, "playback": descriptor})
}

func writeCatalogError(w http.ResponseWriter, err error) {
	status, code := http.StatusBadGateway, "catalog_source_failed"
	switch {
	case errors.Is(err, catalog.ErrInvalidID):
		status, code = http.StatusBadRequest, "invalid_catalog_id"
	case errors.Is(err, catalog.ErrNotFound):
		status, code = http.StatusNotFound, "catalog_item_not_found"
	case errors.Is(err, catalog.ErrUnsupported):
		status, code = http.StatusUnprocessableEntity, "episode_playback_unsupported"
	case errors.Is(err, context.DeadlineExceeded):
		status, code = http.StatusGatewayTimeout, "catalog_timeout"
	case errors.Is(err, catalog.ErrUnavailable):
		status, code = http.StatusServiceUnavailable, "catalog_source_unavailable"
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
}
