package mediasearch

import (
	"context"
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

func (s *phimAPISource) Search(ctx context.Context, query string) ([]contracts.MediaItem, error) {
	var response phimAPIResponse
	path := "/v1/api/tim-kiem?" + url.Values{
		"keyword": {query}, "limit": {strconv.Itoa(resultLimit)},
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
		kind := "movie"
		if entry.Type == "series" {
			kind = "show"
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
		items = append(items, contracts.MediaItem{
			ID: "phimapi:" + entry.Slug, Title: entry.Name, Kind: kind, Year: entry.Year,
			URL: target, Summary: strings.Join(summary, " · "), Availability: "external",
			Poster: phimAPIImage(response.Data.ImageDomain, entry.Poster, entry.Thumb),
			Actions: []contracts.MediaAction{
				{Type: "catalog_detail", Label: "Chọn tập", SourceID: "phimapi",
					ItemID: "phimapi:" + entry.Slug, Enabled: true},
				linkAction("Mở trang phim", target),
			},
		})
		if len(items) == resultLimit {
			break
		}
	}

	return items, nil
}
