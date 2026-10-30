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

type phimAPISource struct{ *httpSource }

type phimAPIResponse struct {
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
			AlternativeNames []string `json:"alternative_names"`
		} `json:"items"`
		ImageDomain string `json:"APP_DOMAIN_CDN_IMAGE"`
	} `json:"data"`
}

func (s *phimAPISource) Capability(ctx context.Context) contracts.MediaSourceCapability {
	capability := s.Info()
	if !s.Configured() {
		capability.Status = "not_configured"
		return capability
	}
	var response phimAPIResponse
	if err := s.do(ctx, http.MethodGet, "/v1/api/home", nil, &response); err != nil ||
		response.Status != "success" || response.Data.Items == nil {
		capability.Status = "unreachable"
		return capability
	}
	capability.Detected, capability.Available, capability.Status = true, true, "ready"
	capability.Actions = []string{"open_url"}

	return capability
}

// identitySearchLimit widens the PhimAPI candidate pool for identity matching, where the
// provider's relevance order often pushes the canonical entry past the display limit.
const identitySearchLimit = 24

func (s *phimAPISource) Search(ctx context.Context, query string) ([]contracts.MediaItem, error) {
	return s.search(ctx, query, resultLimit)
}

func (s *phimAPISource) SearchWide(ctx context.Context, query string) ([]contracts.MediaItem, error) {
	return s.search(ctx, query, identitySearchLimit)
}

func (s *phimAPISource) search(
	ctx context.Context, query string, limit int,
) ([]contracts.MediaItem, error) {
	var response phimAPIResponse
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
		target := "https://www.kkphim2.com/phim/" + url.PathEscape(entry.Slug)
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
		items = append(items, contracts.MediaItem{
			ID: "phimapi:" + entry.Slug, Title: entry.Name, Kind: kind, Year: entry.Year,
			Season: season,
			URL: target, Summary: strings.Join(summary, " · "), Availability: "external",
			Poster:   phimAPIImage(response.Data.ImageDomain, entry.Poster, entry.Thumb),
			Original: entry.OriginName, AlternativeTitles: entry.AlternativeNames,
			ExternalIDs: externalIDs,
			Actions: []contracts.MediaAction{
				{Type: "catalog_detail", Label: actionLabel, SourceID: "phimapi",
					ItemID: "phimapi:" + entry.Slug, Enabled: true},
				linkAction("Mở trang phim", target),
			},
		})
		if len(items) == limit {
			break
		}
	}

	return items, nil
}

func providerID(value json.RawMessage) string {
	var text string
	if json.Unmarshal(value, &text) == nil {
		return text
	}
	var number int64
	if json.Unmarshal(value, &number) == nil && number > 0 {
		return strconv.FormatInt(number, 10)
	}

	return ""
}

func phimAPIContentKind(providerType, episode string) string {
	if strings.EqualFold(providerType, "series") {
		return "show"
	}
	if strings.EqualFold(strings.TrimSpace(episode), "full") {
		return "movie"
	}
	if strings.TrimSpace(episode) != "" {
		return "show"
	}
	return "movie"
}
