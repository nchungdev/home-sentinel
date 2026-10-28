package mediasearch

import (
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/nchungdev/home-sentinel/internal/integrations/mediabridge"
)

func FromEnvironment(bridge *mediabridge.Bridge) *Service {
	if os.Getenv("MEDIA_SEARCH_MODE") == "placeholder" {
		return New(PlaceholderSources()...)
	}

	client := &http.Client{Timeout: searchTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	plexURL := os.Getenv("PLEX_URL")
	if plexURL == "" && bridge != nil {
		plexURL = bridge.PlexURL
	}
	if plexURL == "" {
		plexURL = "http://127.0.0.1:32400"
	}
	makeSource := func(id, name, kind, base, key, header, probe string) *httpSource {
		return &httpSource{id: id, name: name, kind: kind, baseURL: strings.TrimRight(base, "/"),
			publicURL: os.Getenv(strings.ToUpper(id) + "_PUBLIC_URL"), apiKey: key,
			keyHeader: header, probePath: probe, client: client}
	}
	plex := &plexSource{httpSource: makeSource("plex", "Plex", "library", plexURL, mediabridge.ReadPlexToken(),
		"X-Plex-Token", "/library/sections")}
	jellyfin := &jellyfinSource{makeSource("jellyfin", "Jellyfin", "library",
		envDefault("JELLYFIN_URL", "http://127.0.0.1:8096"), os.Getenv("JELLYFIN_API_KEY"),
		"X-Emby-Token", "/System/Info/Public")}
	arr := func(id, name, port string) *arrSource {
		prefix := strings.ToUpper(id)
		profileID, _ := strconv.Atoi(os.Getenv(prefix + "_PROFILE_ID"))
		return &arrSource{httpSource: makeSource(id, name, "catalog",
			envDefault(prefix+"_URL", "http://127.0.0.1:"+port), os.Getenv(prefix+"_API_KEY"),
			"X-Api-Key", "/api/v3/system/status"), profileID: profileID, rootFolder: os.Getenv(prefix + "_ROOT_FOLDER")}
	}
	youtube := &youtubeSource{makeSource("youtube", "YouTube", "video",
		"https://www.googleapis.com/youtube/v3", os.Getenv("YOUTUBE_API_KEY"), "", "")}
	web := &webSource{makeSource("web", "Internet", "web", os.Getenv("SEARXNG_URL"), "", "", "/config")}
	sources := []Source{plex, jellyfin, arr("radarr", "Radarr", "7878"),
		arr("sonarr", "Sonarr", "8989"), youtube, web}
	phimAPI := &phimAPISource{makeSource("phimapi", "PhimAPI", "catalog",
		envDefault("PHIMAPI_URL", "https://phimapi.com"), "", "", "/v1/api/home")}
	sources = append(sources, phimAPI)
	for _, source := range pendingCatalogSources() {
		if source.Info().ID != "phimapi" {
			sources = append(sources, source)
		}
	}
	return New(sources...)
}

func envDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
