package mediabridge

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

type PlexResult struct {
	Title     string `json:"title"`
	Year      int    `json:"year"`
	RatingKey string `json:"rating_key"`
	Type      string `json:"type"`
	Summary   string `json:"summary"`
	Thumb     string `json:"thumb"`
	DeepLink  string `json:"deep_link"`
}

type Bridge struct {
	NASHost    string
	HTTPClient *http.Client
}

func NewBridge(nasHost string) *Bridge {
	if nasHost == "" {
		nasHost = os.Getenv("AGENT_NAS_IP")
		if nasHost == "" {
			nasHost = "127.0.0.1"
		}
	}
	return &Bridge{
		NASHost: nasHost,
		HTTPClient: &http.Client{
			Timeout: 4 * time.Second,
		},
	}
}

func CleanQuery(query string) string {
	parts := strings.Fields(query)
	stopwords := map[string]bool{
		"phim": true, "bộ": true, "lẻ": true, "hoạt": true, "hình": true,
		"anime": true, "search": true, "tìm": true, "xem": true, "kiếm": true,
	}
	var kept []string
	for _, p := range parts {
		if !stopwords[strings.ToLower(p)] {
			kept = append(kept, p)
		}
	}
	cleaned := strings.Join(kept, " ")
	reSuffix := regexp.MustCompile(`(?i)\s+(pirood|period|pro|full|vietsub|thuyết\s*minh|tập|season|phần).*$`)
	cleaned = reSuffix.ReplaceAllString(cleaned, "")
	return strings.TrimSpace(cleaned)
}

func (b *Bridge) PlexSearch(query string, token string) ([]PlexResult, error) {
	if token == "" {
		return nil, nil
	}
	cleaned := CleanQuery(query)
	if cleaned == "" {
		cleaned = query
	}

	searchURL := fmt.Sprintf("http://127.0.0.1:32400/hubs/search?query=%s&X-Plex-Token=%s",
		url.QueryEscape(cleaned), url.QueryEscape(token))

	req, err := http.NewRequest(http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := b.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("plex status %d", resp.StatusCode)
	}

	var data struct {
		MediaContainer struct {
			Hub []struct {
				Metadata []struct {
					Type      string `json:"type"`
					Title     string `json:"title"`
					RatingKey string `json:"ratingKey"`
					Year      int    `json:"year"`
					Summary   string `json:"summary"`
					Thumb     string `json:"thumb"`
				} `json:"Metadata"`
			} `json:"Hub"`
		} `json:"MediaContainer"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var results []PlexResult
	for _, hub := range data.MediaContainer.Hub {
		for _, item := range hub.Metadata {
			if item.Type != "movie" && item.Type != "show" {
				continue
			}
			thumbURL := ""
			if item.Thumb != "" {
				thumbURL = fmt.Sprintf("http://%s:32400%s?X-Plex-Token=%s", b.NASHost, item.Thumb, token)
			}
			results = append(results, PlexResult{
				Title:     item.Title,
				Year:      item.Year,
				RatingKey: item.RatingKey,
				Type:      item.Type,
				Summary:   item.Summary,
				Thumb:     thumbURL,
				DeepLink:  fmt.Sprintf("plex://server/play?key=%s", item.RatingKey),
			})
		}
	}
	return results, nil
}
