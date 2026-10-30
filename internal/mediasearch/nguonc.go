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

type nguoncSource struct{ *httpSource }

type nguoncResponse struct {
	Status string `json:"status"`
	Items  []struct {
		Name         string `json:"name"`
		OriginalName string `json:"original_name"`
		Slug         string `json:"slug"`
		Year         int    `json:"year"`
		Episode      string `json:"current_episode"`
		Language     string `json:"language"`
		Poster       string `json:"poster_url"`
		Thumb        string `json:"thumb_url"`
	} `json:"items"`
}

func (s *nguoncSource) Capability(ctx context.Context) contracts.MediaSourceCapability {
	capability := s.Info()
	if !s.Configured() {
		capability.Status = "not_configured"
		return capability
	}
	var response nguoncResponse
	if err := s.do(ctx, http.MethodGet, "/api/films/phim-moi-cap-nhat", nil, &response); err != nil ||
		response.Status != "success" {
		capability.Status = "unreachable"
		return capability
	}
	capability.Detected, capability.Available, capability.Status = true, true, "ready"
	capability.Actions = []string{"open_url"}

	return capability
}

func (s *nguoncSource) Search(ctx context.Context, query string) ([]contracts.MediaItem, error) {
	return s.search(ctx, query, resultLimit)
}

func (s *nguoncSource) SearchWide(ctx context.Context, query string) ([]contracts.MediaItem, error) {
	return s.search(ctx, query, identitySearchLimit)
}

func (s *nguoncSource) search(
	ctx context.Context, query string, limit int,
) ([]contracts.MediaItem, error) {
	var response nguoncResponse
	path := "/api/films/search?" + url.Values{"keyword": {query}}.Encode()
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
		if entry.Episode != "" {
			summary = append(summary, entry.Episode)
		}
		actionLabel := "Chọn tập"
		if kind == "movie" {
			actionLabel = "Xem phim"
		}
		items = append(items, contracts.MediaItem{
			ID: "nguonc:" + entry.Slug, Title: entry.Name, Kind: kind, Year: entry.Year,
			URL: target, Summary: strings.Join(summary, " · "), Availability: "external",
			Poster:   phimAPIImage(s.baseURL, entry.Poster, entry.Thumb),
			Original: entry.OriginalName,
			Actions: []contracts.MediaAction{
				{Type: "catalog_detail", Label: actionLabel, SourceID: "nguonc",
					ItemID: "nguonc:" + entry.Slug, Enabled: true},
				linkAction("Mở trang phim", target),
			},
		})
		if len(items) == limit {
			break
		}
	}

	return items, nil
}
