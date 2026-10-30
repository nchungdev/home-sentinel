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

type rophimSource struct{ *httpSource }

func (s *rophimSource) Capability(ctx context.Context) contracts.MediaSourceCapability {
	capability := s.Info()
	if !s.Configured() {
		capability.Status = "not_configured"
		return capability
	}
	var response webCatalogResponse
	if err := s.do(ctx, http.MethodGet, "/api/v1/home", nil, &response); err != nil ||
		response.Status != "success" {
		capability.Status = "unreachable"
		return capability
	}
	capability.Detected, capability.Available, capability.Status = true, true, "ready"
	capability.Actions = []string{"open_url"}

	return capability
}

func (s *rophimSource) Search(ctx context.Context, query string) ([]contracts.MediaItem, error) {
	return s.search(ctx, query, resultLimit)
}

func (s *rophimSource) SearchWide(ctx context.Context, query string) ([]contracts.MediaItem, error) {
	return s.search(ctx, query, identitySearchLimit)
}

func (s *rophimSource) search(
	ctx context.Context, query string, limit int,
) ([]contracts.MediaItem, error) {
	var response webCatalogResponse
	path := "/api/v1/search?" + url.Values{"q": {query}}.Encode()
	if err := s.do(ctx, http.MethodGet, path, nil, &response); err != nil {
		return nil, err
	}
	if response.Status != "success" {
		return nil, fmt.Errorf("invalid_source_response")
	}
	items := []contracts.MediaItem{}
	for _, entry := range response.Items {
		if entry.Slug == "" || strings.ContainsAny(entry.Slug, "/?#\\") || entry.Name == "" {
			continue
		}
		target := strings.TrimRight(s.baseURL, "/") + "/phim/" + url.PathEscape(entry.Slug)
		kind := phimAPIContentKind("", entry.Episode)
		summary := []string{entry.OriginalName}
		if entry.Year > 0 {
			summary = append(summary, strconv.Itoa(entry.Year))
		}
		actionLabel := "Chọn tập"
		if kind == "movie" {
			actionLabel = "Xem phim"
		}
		items = append(items, contracts.MediaItem{
			ID: "rophim:" + entry.Slug, Title: entry.Name, Kind: kind, Year: entry.Year,
			URL: target, Summary: strings.Join(summary, " · "), Availability: "external",
			Poster:   phimAPIImage(s.baseURL, entry.Poster, entry.Thumb),
			Original: entry.OriginalName,
			Actions: []contracts.MediaAction{
				{Type: "catalog_detail", Label: actionLabel, SourceID: "rophim",
					ItemID: "rophim:" + entry.Slug, Enabled: true},
				linkAction("Mở trang phim", target),
			},
		})
		if len(items) == limit {
			break
		}
	}

	return items, nil
}
