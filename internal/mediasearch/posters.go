package mediasearch

import (
	"net/url"
	"strings"
)

const (
	posterWidth      = "240"
	posterHeight     = "360"
	phimAPIImageHost = "https://phimimg.com"
)

func (s *plexSource) poster(thumb string) string {
	base := strings.TrimRight(s.publicURL, "/")
	if thumb == "" || base == "" || s.apiKey == "" {
		return ""
	}
	query := url.Values{
		"width": {posterWidth}, "height": {posterHeight}, "minSize": {"1"},
		"url": {thumb}, "X-Plex-Token": {s.apiKey},
	}
	return base + "/photo/:/transcode?" + query.Encode()
}

func phimAPIImage(domain string, candidates ...string) string {
	for _, path := range candidates {
		if path == "" {
			continue
		}
		if strings.HasPrefix(path, "https://") || strings.HasPrefix(path, "http://") {
			return path
		}
		host := strings.TrimRight(domain, "/")
		if host == "" {
			host = phimAPIImageHost
		}
		return host + "/" + strings.TrimLeft(path, "/")
	}
	return ""
}

func arrPoster(entry map[string]any) string {
	if remote, _ := entry["remotePoster"].(string); remote != "" {
		return remote
	}
	images, _ := entry["images"].([]any)
	for _, raw := range images {
		image, _ := raw.(map[string]any)
		if kind, _ := image["coverType"].(string); kind != "poster" {
			continue
		}
		if remote, _ := image["remoteUrl"].(string); remote != "" {
			return remote
		}
	}
	return ""
}
