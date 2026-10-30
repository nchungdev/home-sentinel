package mediasearch

import (
	"context"
	"strings"
	"sync"

	"github.com/nchungdev/home-sentinel/internal/contracts"
)

const phimAPIQueryConcurrency = 3

func (s *Service) SearchPhimAPI(
	ctx context.Context,
	title, kind string,
	year int,
	aliases []string,
	externalIDs map[string]string,
) contracts.MediaSourceResults {
	return s.SearchPhimAPIQueries(ctx, title, kind, year, aliases,
		phimAPIQueries(title, aliases), externalIDs)
}

func (s *Service) SearchPhimAPIQueries(
	ctx context.Context,
	title, kind string,
	year int,
	aliases, queries []string,
	externalIDs map[string]string,
) contracts.MediaSourceResults {
	result := s.searchPhimAPIQueries(ctx, title, aliases, queries)
	result.Items = matchPhimAPIItems(
		result.Items, title, aliases, year, kind, externalIDs,
	)
	if result.Status == "ok" && len(result.Items) == 0 {
		result.Status = "empty"
		if result.ErrorCode == "" {
			result.ErrorCode = "identity_not_verified"
		}
	}

	return result
}

func (s *Service) SearchPhimAPIDiscovery(
	ctx context.Context,
	title, kind string,
	year int,
	tmdbAliases, aiQueries []string,
	externalIDs map[string]string,
) contracts.MediaSourceResults {
	result := s.searchPhimAPIQueries(ctx, title, tmdbAliases, aiQueries)
	result.Items = matchPhimAPIItemsWithDiscovery(
		result.Items, title, tmdbAliases, aiQueries, year, kind, externalIDs,
	)
	if result.Status == "ok" && len(result.Items) == 0 {
		result.Status = "empty"
		if result.ErrorCode == "" {
			result.ErrorCode = "identity_not_verified"
		}
	}

	return result
}

func (s *Service) searchPhimAPIQueries(
	ctx context.Context,
	title string,
	aliases, queryCandidates []string,
) contracts.MediaSourceResults {
	queries := phimAPIQueries("", queryCandidates)
	combined := contracts.MediaSourceResults{
		SourceID: "phimapi", Status: "empty", Cache: "hit",
		PlaybackMode: 2,
		Items:        []contracts.MediaItem{},
	}
	seen := make(map[string]struct{})
	for start := 0; start < len(queries); start += phimAPIQueryConcurrency {
		end := min(start+phimAPIQueryConcurrency, len(queries))
		batch := searchPhimAPIBatch(ctx, s, queries[start:end])
		mergePhimAPIResults(&combined, seen, batch)
	}
	if ctx.Err() != nil && combined.ErrorCode == "" {
		combined.ErrorCode = "partial_search_failed"
	}

	return combined
}

func searchPhimAPIBatch(
	ctx context.Context,
	service *Service,
	queries []string,
) []contracts.MediaSourceResults {
	results := make([]contracts.MediaSourceResults, len(queries))
	var workers sync.WaitGroup
	for index, query := range queries {
		workers.Add(1)
		go func() {
			defer workers.Done()
			results[index] = service.SearchSourceWide(ctx, "phimapi", query)
		}()
	}
	workers.Wait()

	return results
}

func mergePhimAPIResults(
	combined *contracts.MediaSourceResults,
	seen map[string]struct{},
	results []contracts.MediaSourceResults,
) {
	anyError := false
	anySuccessfulSearch := false
	for _, result := range results {
		if !combined.Searched {
			combined.Status, combined.ErrorCode = result.Status, result.ErrorCode
		}
		combined.Searched = combined.Searched || result.Searched
		anyError = anyError || result.Status == "error"
		anySuccessfulSearch = anySuccessfulSearch ||
			(result.Searched && result.Status != "error")
		if result.Cache == "miss" {
			combined.Cache = "miss"
		}
		for _, item := range result.Items {
			if _, found := seen[item.ID]; found {
				continue
			}
			seen[item.ID] = struct{}{}
			combined.Items = append(combined.Items, item)
		}
		if combined.ErrorCode == "" && result.ErrorCode != "" {
			combined.ErrorCode = result.ErrorCode
		}
	}
	if anyError {
		combined.ErrorCode = "partial_search_failed"
	}
	if len(combined.Items) > 0 {
		combined.Status = "ok"
	} else if anySuccessfulSearch {
		combined.Status = "empty"
	} else if anyError {
		combined.Status = "error"
	}
}

func phimAPIQueries(title string, aliases []string) []string {
	queries := make([]string, 0, len(aliases)+1)
	seen := make(map[string]struct{}, len(aliases)+1)
	for _, candidate := range append([]string{title}, aliases...) {
		candidate = strings.TrimSpace(candidate)
		normalized := normalizeIdentityTitle(candidate)
		if normalized == "" {
			continue
		}
		if _, found := seen[normalized]; found {
			continue
		}
		seen[normalized] = struct{}{}
		queries = append(queries, candidate)
	}

	return queries
}
