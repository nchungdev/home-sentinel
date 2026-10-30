package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
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
	s.searchRunner()
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

type mediaConfig struct {
	TMDBApiKey     string `json:"tmdb_api_key"`
	YouTubeApiKey  string `json:"youtube_api_key"`
	PlexURL        string `json:"plex_url"`
	PlexToken      string `json:"plex_token"`
	JellyfinURL    string `json:"jellyfin_url"`
	JellyfinApiKey string `json:"jellyfin_api_key"`
	RadarrURL      string `json:"radarr_url"`
	RadarrApiKey   string `json:"radarr_api_key"`
	SonarrURL      string `json:"sonarr_url"`
	SonarrApiKey   string `json:"sonarr_api_key"`
	SearXNGURL     string `json:"searxng_url"`
}

func (s *Server) handleMediaConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, map[string]any{
			"ok": true,
			"config": mediaConfig{
				TMDBApiKey:     envDefault("TMDB_API_KEY", ""),
				YouTubeApiKey:  envDefault("YOUTUBE_API_KEY", ""),
				PlexURL:        envDefault("PLEX_URL", "http://127.0.0.1:32400"),
				PlexToken:      envDefault("PLEX_TOKEN", ""),
				JellyfinURL:    envDefault("JELLYFIN_URL", "http://127.0.0.1:8096"),
				JellyfinApiKey: envDefault("JELLYFIN_API_KEY", ""),
				RadarrURL:      envDefault("RADARR_URL", "http://127.0.0.1:7878"),
				RadarrApiKey:   envDefault("RADARR_API_KEY", ""),
				SonarrURL:      envDefault("SONARR_URL", "http://127.0.0.1:8989"),
				SonarrApiKey:   envDefault("SONARR_API_KEY", ""),
				SearXNGURL:     envDefault("SEARXNG_URL", ""),
			},
		})
	case http.MethodPost:
		var cfg mediaConfig
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&cfg); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}

		setIfPresent := func(key, val string) {
			if strings.TrimSpace(val) != "" {
				_ = os.Setenv(key, strings.TrimSpace(val))
			}
		}

		setIfPresent("TMDB_API_KEY", cfg.TMDBApiKey)
		setIfPresent("YOUTUBE_API_KEY", cfg.YouTubeApiKey)
		setIfPresent("PLEX_URL", cfg.PlexURL)
		setIfPresent("PLEX_TOKEN", cfg.PlexToken)
		setIfPresent("JELLYFIN_URL", cfg.JellyfinURL)
		setIfPresent("JELLYFIN_API_KEY", cfg.JellyfinApiKey)
		setIfPresent("RADARR_URL", cfg.RadarrURL)
		setIfPresent("RADARR_API_KEY", cfg.RadarrApiKey)
		setIfPresent("SONARR_URL", cfg.SonarrURL)
		setIfPresent("SONARR_API_KEY", cfg.SonarrApiKey)
		setIfPresent("SEARXNG_URL", cfg.SearXNGURL)

		saveMediaConfigEnv(cfg)

		s.MediaSearch = mediasearch.FromEnvironment(nil)

		writeJSON(w, map[string]any{
			"ok":      true,
			"sources": s.MediaSearch.Discover(r.Context(), true),
		})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func saveMediaConfigEnv(cfg mediaConfig) {
	envPath := "env.local"
	if _, err := os.Stat("/home/chungnh/.config/home-sentinel.env"); err == nil {
		envPath = "/home/chungnh/.config/home-sentinel.env"
	} else if _, err := os.Stat("/home/chungnh/.config/tv-agent.env"); err == nil {
		envPath = "/home/chungnh/.config/tv-agent.env"
	}

	content, _ := os.ReadFile(envPath)
	lines := strings.Split(string(content), "\n")
	envMap := make(map[string]string)
	order := []string{}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, found := strings.Cut(line, "=")
		if found {
			k = strings.TrimSpace(k)
			envMap[k] = strings.TrimSpace(v)
			order = append(order, k)
		}
	}

	updateKey := func(k, v string) {
		if strings.TrimSpace(v) == "" {
			return
		}
		if _, exists := envMap[k]; !exists {
			order = append(order, k)
		}
		envMap[k] = strings.TrimSpace(v)
	}

	updateKey("TMDB_API_KEY", cfg.TMDBApiKey)
	updateKey("YOUTUBE_API_KEY", cfg.YouTubeApiKey)
	updateKey("PLEX_URL", cfg.PlexURL)
	updateKey("PLEX_TOKEN", cfg.PlexToken)
	updateKey("JELLYFIN_URL", cfg.JellyfinURL)
	updateKey("JELLYFIN_API_KEY", cfg.JellyfinApiKey)
	updateKey("RADARR_URL", cfg.RadarrURL)
	updateKey("RADARR_API_KEY", cfg.RadarrApiKey)
	updateKey("SONARR_URL", cfg.SonarrURL)
	updateKey("SONARR_API_KEY", cfg.SonarrApiKey)
	updateKey("SEARXNG_URL", cfg.SearXNGURL)

	var newLines []string
	for _, k := range order {
		if v, ok := envMap[k]; ok {
			newLines = append(newLines, k+"="+v)
		}
	}
	_ = os.WriteFile(envPath, []byte(strings.Join(newLines, "\n")+"\n"), 0600)
}

func envDefault(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
