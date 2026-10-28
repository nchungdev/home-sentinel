package skills

import (
	"context"
	"errors"

	"github.com/nchungdev/home-sentinel/internal/contracts"
	"github.com/nchungdev/home-sentinel/internal/requestflow"
	websearch "github.com/nchungdev/home-sentinel/internal/search/web"
)

type MediaSearcher interface {
	SearchSource(context.Context, string, string) contracts.MediaSourceResults
}

type MediaSearch struct {
	Service MediaSearcher
}

func (s MediaSearch) Execute(ctx context.Context, task requestflow.Task) requestflow.Result {
	result := requestflow.Result{Cache: "bypassed", Items: []requestflow.Item{}, Status: "skipped"}
	if s.Service == nil {
		result.ErrorCode = "not_configured"
		return result
	}
	media := s.Service.SearchSource(ctx, task.Source, task.Query)
	result.Status, result.ErrorCode = media.Status, media.ErrorCode
	for _, entry := range media.Items {
		item := requestflow.Item{ID: entry.ID, Kind: "media_title", Title: entry.Title,
			URL: entry.URL, Content: entry.Summary, Poster: entry.Poster, Year: entry.Year,
			Season: valueOf(entry.Season), Episode: valueOf(entry.Episode)}
		for _, action := range entry.Actions {
			if action.Type == "open_url" || action.Type == "catalog_detail" ||
				action.Type == "open_app" {
				item.Actions = append(item.Actions, requestflow.ItemAction{
					Type: action.Type, Label: action.Label, URL: action.URL,
					SourceID: action.SourceID, ItemID: action.ItemID,
				})
			}
		}
		result.Items = append(result.Items, item)
	}

	return result
}

type WebSearch struct {
	Searcher *websearch.Searcher
}

func (s WebSearch) Execute(ctx context.Context, task requestflow.Task) requestflow.Result {
	result := requestflow.Result{Cache: "bypassed", Items: []requestflow.Item{}, Status: "error"}
	links, cacheStatus, err := s.Searcher.Search(ctx, task.Query)
	result.Cache = cacheStatus
	if err != nil {
		result.ErrorCode = "search_failed"
		switch {
		case errors.Is(err, websearch.ErrNotConfigured):
			result.Status, result.ErrorCode = "skipped", "not_configured"
		case errors.Is(err, websearch.ErrAuthRequired):
			result.ErrorCode = "auth_required"
		case errors.Is(err, context.DeadlineExceeded):
			result.ErrorCode = "timeout"
		}
		return result
	}
	result.Status = "empty"
	if len(links) > 0 {
		result.Status = "ok"
	}
	for _, link := range links {
		result.Items = append(result.Items, requestflow.Item{
			ID: link.URL, Kind: "web_link", Title: link.Title, URL: link.URL, Content: link.Content,
			Poster: youtubeThumbnail(link.URL),
		})
	}

	return result
}
