package mediasearch

import (
	"strings"
)

type externalIDEntry struct {
	ID string `json:"id"`
}

func plexExternalIDs(guids []externalIDEntry) map[string]string {
	externalIDs := make(map[string]string)
	for _, guid := range guids {
		scheme, value, found := strings.Cut(guid.ID, "://")
		if !found || value == "" {
			continue
		}
		switch strings.ToLower(scheme) {
		case "tmdb", "imdb", "tvdb":
			externalIDs[strings.ToLower(scheme)] = value
		}
	}

	return externalIDs
}

func jellyfinExternalIDs(providerIDs map[string]string) map[string]string {
	externalIDs := make(map[string]string)
	for key, value := range providerIDs {
		if value == "" {
			continue
		}
		switch strings.ToLower(key) {
		case "tmdb":
			externalIDs["tmdb"] = value
		case "imdb":
			externalIDs["imdb"] = value
		case "tvdb":
			externalIDs["tvdb"] = value
		}
	}

	return externalIDs
}
