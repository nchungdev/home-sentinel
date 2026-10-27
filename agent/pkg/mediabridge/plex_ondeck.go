package mediabridge

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type OnDeckEntry struct {
	RatingKey    string `json:"ratingKey"`
	Type         string `json:"type"`
	Title        string `json:"title"`
	ShowTitle    string `json:"grandparentTitle"`
	Season       int    `json:"parentIndex"`
	Episode      int    `json:"index"`
	Year         int    `json:"year"`
	ViewOffset   int64  `json:"viewOffset"`
	Duration     int64  `json:"duration"`
	Thumb        string `json:"thumb"`
	ShowThumb    string `json:"grandparentThumb"`
	LastViewedAt int64  `json:"lastViewedAt"`
}

func (b *Bridge) PlexOnDeck(ctx context.Context, token string) ([]OnDeckEntry, error) {
	if token == "" {
		token = ReadPlexToken()
	}
	if token == "" {
		return nil, nil
	}

	deckURL := b.plexBase() + "/library/onDeck?X-Plex-Token=" + url.QueryEscape(token)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, deckURL, nil)
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
			Metadata []OnDeckEntry `json:"Metadata"`
		} `json:"MediaContainer"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	return data.MediaContainer.Metadata, nil
}

func (b *Bridge) PlexImageURL(path string, token string) string {
	if path == "" {
		return ""
	}
	if token == "" {
		token = ReadPlexToken()
	}
	return fmt.Sprintf("http://%s:32400%s?X-Plex-Token=%s", b.NASHost, path, url.QueryEscape(token))
}

func (b *Bridge) plexBase() string {
	if b.PlexURL != "" {
		return b.PlexURL
	}
	return "http://127.0.0.1:32400"
}
