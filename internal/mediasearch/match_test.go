package mediasearch

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/nchungdev/home-sentinel/internal/contracts"
)

type queryAwareSource struct {
	mu      sync.Mutex
	queries []string
	search  func(string) ([]contracts.MediaItem, error)
}

func (s *queryAwareSource) Info() contracts.MediaSourceCapability {
	return contracts.MediaSourceCapability{ID: "phimapi", Status: "ready"}
}

func (s *queryAwareSource) Capability(context.Context) contracts.MediaSourceCapability {
	return contracts.MediaSourceCapability{
		ID: "phimapi", Available: true, Detected: true, Status: "ready",
	}
}

func (s *queryAwareSource) Search(_ context.Context, query string) ([]contracts.MediaItem, error) {
	s.mu.Lock()
	s.queries = append(s.queries, query)
	s.mu.Unlock()

	return s.search(query)
}

func TestSearchMatchingRequiresExactExternalIDsAndMediaKind(t *testing.T) {
	expected := map[string]string{
		"tmdb": "123", "imdb": "tt456", "tvdb": "789",
	}
	service := New(
		controlledSource{id: "plex", search: func(context.Context) ([]contracts.MediaItem, error) {
			return []contracts.MediaItem{
				{ID: "right", Kind: "movie", Title: "Different localized title",
					ExternalIDs: map[string]string{"tmdb": "123"}},
				{ID: "wrong-id", Kind: "movie", Title: "Exact same title",
					ExternalIDs: map[string]string{"tmdb": "999"}},
				{ID: "wrong-kind", Kind: "show", ExternalIDs: map[string]string{"tmdb": "123"}},
				{ID: "conflicting", Kind: "movie",
					ExternalIDs: map[string]string{"tmdb": "999", "imdb": "tt456"}},
			}, nil
		}},
		controlledSource{id: "jellyfin", search: func(context.Context) ([]contracts.MediaItem, error) {
			return []contracts.MediaItem{{ID: "jellyfin-right", Kind: "movie",
				ExternalIDs: map[string]string{"imdb": "tt456"}}}, nil
		}},
		controlledSource{id: "radarr", search: func(context.Context) ([]contracts.MediaItem, error) {
			return []contracts.MediaItem{{ID: "radarr-right", Kind: "movie",
				ExternalIDs: map[string]string{"tmdb": "123"}}}, nil
		}},
		controlledSource{id: "sonarr", search: func(context.Context) ([]contracts.MediaItem, error) {
			return []contracts.MediaItem{{ID: "sonarr-right", Kind: "movie",
				ExternalIDs: map[string]string{"tvdb": "789"}}}, nil
		}},
		controlledSource{id: "phimapi", search: func(context.Context) ([]contracts.MediaItem, error) {
			return []contracts.MediaItem{{ID: "phimapi:right", Kind: "movie",
				ExternalIDs: map[string]string{"tmdb": "123"}}}, nil
		}},
	)

	results := service.SearchMatching(context.Background(), "Localized title", "movie", 2024,
		[]string{"Localized title"}, expected)
	for _, result := range results {
		if result.Status != "ok" || len(result.Items) != 1 {
			t.Fatalf("source %q returned unverified matches: %+v", result.SourceID, result)
		}
	}
	if results[0].Items[0].ID != "right" {
		t.Fatalf("title similarity overrode external identity: %+v", results[0].Items)
	}
}

func TestSearchMatchingQueriesPhimAPIUsingTMDBAliases(t *testing.T) {
	matchedItem := contracts.MediaItem{
		ID: "phimapi:westward-wukong", Title: "Tây Hành Kỷ: Đại Viên Hồn",
		Kind: "show", Year: 2025, ExternalIDs: map[string]string{"tmdb": "262330"},
	}
	source := &queryAwareSource{search: func(query string) ([]contracts.MediaItem, error) {
		switch query {
		case "Tây Hành Kỷ: Đại Viên Hồn":
			return []contracts.MediaItem{{ID: "phimapi:unrelated", Kind: "show"}}, nil
		case "大猿魂":
			return []contracts.MediaItem{matchedItem}, nil
		case "The Westward: Wukong":
			return []contracts.MediaItem{matchedItem}, nil
		default:
			return nil, errors.New("unexpected query")
		}
	}}
	service := New(source)

	results := service.SearchMatching(context.Background(), "Tây Hành Kỷ: Đại Viên Hồn",
		"show", 2025, []string{"Tây Hành Kỷ: Đại Viên Hồn", "大猿魂",
			"The Westward: Wukong", "Alt"}, map[string]string{"tmdb": "262330"})
	var phimAPIResult contracts.MediaSourceResults
	for _, result := range results {
		if result.SourceID == "phimapi" {
			phimAPIResult = result
		}
	}
	if len(source.queries) != len(phimAPIQueries("Tây Hành Kỷ: Đại Viên Hồn",
		[]string{"Tây Hành Kỷ: Đại Viên Hồn", "大猿魂", "The Westward: Wukong", "Alt"})) {
		t.Fatalf("not all distinct TMDB aliases were searched, got %v", source.queries)
	}
	if phimAPIResult.Status != "ok" || len(phimAPIResult.Items) != 1 ||
		phimAPIResult.Items[0].ID != matchedItem.ID ||
		phimAPIResult.Items[0].MatchMethod != "external_id" {
		t.Fatalf("alias searches did not locate and deduplicate the exact media: %+v",
			phimAPIResult)
	}
}

func TestSearchMatchingReportsPartialAliasQueryFailure(t *testing.T) {
	source := &queryAwareSource{search: func(query string) ([]contracts.MediaItem, error) {
		if query == "Alias" {
			return nil, errors.New("provider unavailable")
		}

		return []contracts.MediaItem{{ID: "phimapi:verified", Kind: "movie",
			ExternalIDs: map[string]string{"tmdb": "550"}}}, nil
	}}
	results := New(source).SearchMatching(context.Background(), "Fight Club", "movie", 1999,
		[]string{"Alias"}, map[string]string{"tmdb": "550"})
	for _, result := range results {
		if result.SourceID == "phimapi" {
			if result.Status != "ok" || result.ErrorCode != "partial_search_failed" ||
				len(result.Items) != 1 {
				t.Fatalf("partial alias failure was not reported with exact results: %+v", result)
			}
			return
		}
	}

	t.Fatal("PhimAPI result section was not returned")
}

func TestPhimAPIMatcherUsesExactIDsOrUniqueTitleYearKind(t *testing.T) {
	expectedIDs := map[string]string{"tmdb": "123", "imdb": "tt456"}
	matching := contracts.MediaItem{
		ID: "phimapi:match", Title: "Thử Thách Thần Tượng", Original: "Running Man",
		AlternativeTitles: []string{"런닝맨"}, Kind: "show", Year: 2010,
	}
	tests := []struct {
		name        string
		items       []contracts.MediaItem
		expected    int
		matchMethod string
	}{
		{
			name: "exact shared TMDB ID",
			items: []contracts.MediaItem{{ID: "phimapi:exact", Kind: "show",
				ExternalIDs: map[string]string{"tmdb": "123"}}},
			expected: 1, matchMethod: "external_id",
		},
		{
			name:     "normalized localized alias",
			items:    []contracts.MediaItem{matching},
			expected: 1, matchMethod: "title_year",
		},
		{
			name: "case and punctuation normalization",
			items: []contracts.MediaItem{{ID: "phimapi:normalized", Title: "running-man!",
				Kind: "show", Year: 2010}},
			expected: 1, matchMethod: "title_year",
		},
		{
			name: "conflicting external ID blocks title fallback",
			items: []contracts.MediaItem{{ID: "phimapi:conflict", Title: "Running Man",
				Kind: "show", Year: 2010, ExternalIDs: map[string]string{"tmdb": "999"}}},
		},
		{
			name: "different year is not a match",
			items: []contracts.MediaItem{{ID: "phimapi:remake", Title: "Running Man",
				Kind: "show", Year: 2026}},
		},
		{
			name: "missing year is not a match",
			items: []contracts.MediaItem{{ID: "phimapi:no-year", Title: "Running Man",
				Kind: "show"}},
		},
		{
			name: "different media kind is not a match",
			items: []contracts.MediaItem{{ID: "phimapi:movie", Title: "Running Man",
				Kind: "movie", Year: 2010}},
		},
		{
			name: "similar title is not a match",
			items: []contracts.MediaItem{{ID: "phimapi:similar", Title: "Running Man Thailand",
				Kind: "show", Year: 2010}},
		},
		{
			name: "ambiguous fallback candidates are not matched",
			items: []contracts.MediaItem{
				{ID: "phimapi:first", Title: "Running Man", Kind: "show", Year: 2010},
				{ID: "phimapi:second", Original: "Running Man", Kind: "show", Year: 2010},
			},
		},
		{
			name: "same IDs with conflicting second provider ID are rejected",
			items: []contracts.MediaItem{{ID: "phimapi:partial-conflict", Kind: "show",
				ExternalIDs: map[string]string{"tmdb": "123", "imdb": "tt999"}}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			matches := matchPhimAPIItems(test.items, "Thử Thách Thần Tượng",
				[]string{"Running Man", "런닝맨"}, 2010, "show", expectedIDs)
			if len(matches) != test.expected {
				t.Fatalf("unexpected match count: got %d, want %d: %+v",
					len(matches), test.expected, matches)
			}
			if test.matchMethod != "" && matches[0].MatchMethod != test.matchMethod {
				t.Fatalf("unexpected match method: got %q, want %q",
					matches[0].MatchMethod, test.matchMethod)
			}
		})
	}
}

func TestIdentityTitleNormalizationIgnoresVietnameseDiacritics(t *testing.T) {
	if normalizeIdentityTitle("Tây Hành Kỷ") != normalizeIdentityTitle("Tay Hanh Ky") {
		t.Fatal("Vietnamese diacritics prevented exact normalized title matching")
	}
}

func TestPhimAPIQueriesUseCanonicalAndDistinctTMDBAliases(t *testing.T) {
	queries := phimAPIQueries("Tây Hành Kỷ", []string{
		"Tay Hanh Ky", "西行纪", "The Westward", "English title",
	})
	if len(queries) != 4 || queries[0] != "Tây Hành Kỷ" ||
		queries[1] != "西行纪" || queries[2] != "The Westward" ||
		queries[3] != "English title" {
		t.Fatalf("PhimAPI aliases were not normalized and deduplicated: %v", queries)
	}
}
