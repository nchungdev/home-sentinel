package mediasearch

import (
	"regexp"
	"strings"
)

var tmdbFilePattern = regexp.MustCompile(`(?i)\{tmdb-(\d+)\}`)
var imdbFilePattern = regexp.MustCompile(`(?i)\{imdb-(tt\d+)\}`)
var tvdbFilePattern = regexp.MustCompile(`(?i)\{tvdb-(\d+)\}`)

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

func parseFileExternalIDs(filePath string) map[string]string {
	ids := make(map[string]string)
	if match := tmdbFilePattern.FindStringSubmatch(filePath); len(match) > 1 {
		ids["tmdb"] = match[1]
	}
	if match := imdbFilePattern.FindStringSubmatch(filePath); len(match) > 1 {
		ids["imdb"] = match[1]
	}
	if match := tvdbFilePattern.FindStringSubmatch(filePath); len(match) > 1 {
		ids["tvdb"] = match[1]
	}
	return ids
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
