package mediasearch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/nchungdev/home-sentinel/internal/contracts"
)

type ophimSource struct{ *httpSource }

type ophimResponse struct {
	Status string `json:"status"`
	Data   struct {
		Items *[]struct {
			Name       string `json:"name"`
			OriginName string `json:"origin_name"`
			Slug       string `json:"slug"`
			Year       int    `json:"year"`
			Type       string `json:"type"`
			Episode    string `json:"episode_current"`
			Language   string `json:"lang"`
			Poster     string `json:"poster_url"`
			Thumb      string `json:"thumb_url"`
			TMDB       struct {
				ID     json.RawMessage `json:"id"`
				Type   string          `json:"type"`
				Season json.RawMessage `json:"season"`
			} `json:"tmdb"`
			IMDb struct {
				ID json.RawMessage `json:"id"`
			} `json:"imdb"`
		} `json:"items"`
		ImageDomain string `json:"APP_DOMAIN_CDN_IMAGE"`
	} `json:"data"`
}

func (s *ophimSource) Capability(ctx context.Context) contracts.MediaSourceCapability {
	capability := s.Info()
	if !s.Configured() {
		capability.Status = "not_configured"
		return capability
	}
	var response ophimResponse
	if err := s.do(ctx, http.MethodGet, "/v1/api/home", nil, &response); err != nil ||
		response.Status != "success" {
		capability.Status = "unreachable"
		return capability
	}
	capability.Detected, capability.Available, capability.Status = true, true, "ready"
	capability.Actions = []string{"open_url"}

	return capability
}

func (s *ophimSource) Search(ctx context.Context, query string) ([]contracts.MediaItem, error) {
	return s.search(ctx, query, resultLimit)
}

func (s *ophimSource) SearchWide(ctx context.Context, query string) ([]contracts.MediaItem, error) {
	return s.search(ctx, query, identitySearchLimit)
}

func (s *ophimSource) search(
	ctx context.Context, query string, limit int,
) ([]contracts.MediaItem, error) {
	var response ophimResponse
	path := "/v1/api/tim-kiem?" + url.Values{
		"keyword": {query}, "limit": {strconv.Itoa(limit)},
	}.Encode()
	if err := s.do(ctx, http.MethodGet, path, nil, &response); err != nil {
		return nil, err
	}
	if response.Status != "success" || response.Data.Items == nil {
		return nil, fmt.Errorf("invalid_source_response")
	}
	items := []contracts.MediaItem{}
	for _, entry := range *response.Data.Items {
		if entry.Slug == "" || strings.ContainsAny(entry.Slug, "/?#\\") || entry.Name == "" {
			continue
		}
		target := strings.TrimRight(s.baseURL, "/") + "/phim/" + url.PathEscape(entry.Slug)
		kind := phimAPIContentKind(entry.Type, entry.Episode)
		if entry.TMDB.Type == "movie" {
			kind = "movie"
		} else if entry.TMDB.Type == "tv" {
			kind = "show"
		}
		externalIDs := make(map[string]string)
		if id := providerID(entry.TMDB.ID); id != "" {
			externalIDs["tmdb"] = id
		}
		if id := providerID(entry.IMDb.ID); id != "" {
			externalIDs["imdb"] = id
		}
		summary := []string{entry.OriginName}
		if entry.Year > 0 {
			summary = append(summary, strconv.Itoa(entry.Year))
		}
		for _, label := range []string{entry.Episode, entry.Language} {
			if label != "" {
				summary = append(summary, label)
			}
		}
		var season *int
		if value, err := strconv.Atoi(providerID(entry.TMDB.Season)); err == nil && value > 0 {
			season = &value
		}
		actionLabel := "Chọn tập"
		if kind == "movie" {
			actionLabel = "Xem phim"
		}
		imgDomain := response.Data.ImageDomain
		if imgDomain == "" {
			imgDomain = "https://img.ophim.live/uploads/movies"
		}
		items = append(items, contracts.MediaItem{
			ID: "ophim:" + entry.Slug, Title: entry.Name, Kind: kind, Year: entry.Year,
			Season: season,
			URL: target, Summary: strings.Join(summary, " · "), Availability: "external",
			Poster:   phimAPIImage(imgDomain, entry.Poster, entry.Thumb),
			Original: entry.OriginName,
			ExternalIDs: externalIDs,
			Actions: []contracts.MediaAction{
				{Type: "catalog_detail", Label: actionLabel, SourceID: "ophim",
					ItemID: "ophim:" + entry.Slug, Enabled: true},
				linkAction("Mở trang phim", target),
			},
		})
		if len(items) == limit {
			break
		}
	}

	return items, nil
}
