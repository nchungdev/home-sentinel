package mediasearch

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/nchungdev/home-sentinel/internal/contracts"
)

type webSource struct{ *httpSource }

func publicHTTPURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.User == nil && parsed.Host != "" &&
		(parsed.Scheme == "http" || parsed.Scheme == "https")
}

func (s *webSource) Search(ctx context.Context, query string) ([]contracts.MediaItem, error) {
	var response struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}
	path := "/search?format=json&q=" + url.QueryEscape(query)
	if err := s.do(ctx, http.MethodGet, path, nil, &response); err != nil {
		return nil, err
	}
	items := []contracts.MediaItem{}
	for _, entry := range response.Results {
		if !publicHTTPURL(entry.URL) {
			continue
		}
		items = append(items, contracts.MediaItem{ID: entry.URL, Title: entry.Title, URL: entry.URL,
			Kind: "web", Summary: entry.Content, Availability: "external", Actions: []contracts.MediaAction{
				linkAction("Mở liên kết", entry.URL),
			}})
		if len(items) == resultLimit {
			break
		}
	}
	return items, nil
}

type youtubeSource struct{ *httpSource }

type youtubeThumbnailVariant struct {
	URL string `json:"url"`
}

func (s *youtubeSource) Capability(ctx context.Context) contracts.MediaSourceCapability {
	capability := s.Info()
	if s.apiKey == "" {
		capability.Status = "not_configured"
		return capability
	}
	var response map[string]any
	path := "/videos?part=id&chart=mostPopular&maxResults=1&key=" + url.QueryEscape(s.apiKey)
	if err := s.do(ctx, http.MethodGet, path, nil, &response); err != nil {
		capability.Status = "unreachable"
		return capability
	}
	if kind, _ := response["kind"].(string); !strings.HasPrefix(kind, "youtube#") {
		capability.Status = "invalid_service"
		return capability
	}
	capability.Detected, capability.Available, capability.Status = true, true, "ready"
	capability.Actions = []string{"play", "open_url"}
	return capability
}

func (s *youtubeSource) Search(ctx context.Context, query string) ([]contracts.MediaItem, error) {
	var response struct {
		Items []struct {
			ID struct {
				VideoID string `json:"videoId"`
			} `json:"id"`
			Snippet struct {
				Title       string                             `json:"title"`
				Description string                             `json:"description"`
				Thumbnails  map[string]youtubeThumbnailVariant `json:"thumbnails"`
			} `json:"snippet"`
		} `json:"items"`
	}
	values := url.Values{"part": {"snippet"}, "type": {"video"}, "maxResults": {"8"},
		"q": {query}, "key": {s.apiKey}}
	if err := s.do(ctx, http.MethodGet, "/search?"+values.Encode(), nil, &response); err != nil {
		return nil, err
	}
	items := []contracts.MediaItem{}
	for _, entry := range response.Items {
		if entry.ID.VideoID == "" {
			continue
		}
		target := "https://www.youtube.com/watch?v=" + url.QueryEscape(entry.ID.VideoID)
		items = append(items, contracts.MediaItem{ID: entry.ID.VideoID, Title: entry.Snippet.Title,
			Summary: entry.Snippet.Description, Kind: "video", URL: target,
			Poster: youtubeThumbnail(entry.Snippet.Thumbnails), Availability: "external",
			Actions: []contracts.MediaAction{playAction(target), linkAction("Mở YouTube", target)}})
	}
	return items, nil
}

func youtubeThumbnail(thumbnails map[string]youtubeThumbnailVariant) string {
	for _, name := range []string{"maxres", "standard", "high", "medium", "default"} {
		if thumbnail, ok := thumbnails[name]; ok && publicHTTPURL(thumbnail.URL) {
			return thumbnail.URL
		}
	}

	return ""
}

func (s *httpSource) Info() contracts.MediaSourceCapability {
	return contracts.MediaSourceCapability{ID: s.id, Name: s.name, Kind: s.kind,
		Status: "checking", Actions: []string{}}
}

func validProbe(sourceID string, info map[string]any) bool {
	switch sourceID {
	case "plex":
		return info["MediaContainer"] != nil
	case "jellyfin":
		return info["Id"] != nil && info["Version"] != nil
	case "radarr", "sonarr":
		name, _ := info["appName"].(string)
		return info["version"] != nil && (name == "" || strings.EqualFold(name, sourceID))
	case "web":
		return info["engines"] != nil
	}
	return false
}

func sourceError(status string) error { return fmt.Errorf("%s", status) }
