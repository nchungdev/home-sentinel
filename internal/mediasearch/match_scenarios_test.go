package mediasearch

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/nchungdev/home-sentinel/internal/contracts"
)

type widePhimAPISource struct {
	mu           sync.Mutex
	wideCalls    int
	regularCalls int
	search       func(string, bool) ([]contracts.MediaItem, error)
}

func (s *widePhimAPISource) Info() contracts.MediaSourceCapability {
	return contracts.MediaSourceCapability{ID: "phimapi", Status: "ready"}
}

func (s *widePhimAPISource) Capability(context.Context) contracts.MediaSourceCapability {
	return contracts.MediaSourceCapability{
		ID: "phimapi", Available: true, Detected: true, Status: "ready",
	}
}

func (s *widePhimAPISource) Search(_ context.Context, query string) ([]contracts.MediaItem, error) {
	s.mu.Lock()
	s.regularCalls++
	s.mu.Unlock()

	return s.search(query, false)
}

func (s *widePhimAPISource) SearchWide(
	_ context.Context, query string,
) ([]contracts.MediaItem, error) {
	s.mu.Lock()
	s.wideCalls++
	s.mu.Unlock()

	return s.search(query, true)
}

func TestSearchMatchingUsesWidePhimAPIPool(t *testing.T) {
	target := contracts.MediaItem{
		ID: "phimapi:westward-main", Kind: "show", ExternalIDs: map[string]string{"tmdb": "262330"},
	}
	source := &widePhimAPISource{search: func(_ string, wide bool) ([]contracts.MediaItem, error) {
		if !wide {
			return []contracts.MediaItem{{ID: "phimapi:unrelated", Kind: "show"}}, nil
		}

		return []contracts.MediaItem{target}, nil
	}}

	results := New(source).SearchMatching(context.Background(), "Tây Hành Kỷ",
		"show", 2025, nil, map[string]string{"tmdb": "262330"})
	for _, result := range results {
		if result.SourceID != "phimapi" {
			continue
		}
		if result.Status != "ok" || len(result.Items) != 1 ||
			result.Items[0].ID != target.ID || result.Items[0].MatchMethod != "external_id" {
			t.Fatalf("wide identity search missed the exact PhimAPI item: %+v", result)
		}
		if source.wideCalls != 1 || source.regularCalls != 0 {
			t.Fatalf("expected one wide query and no regular queries, got wide=%d regular=%d",
				source.wideCalls, source.regularCalls)
		}
		return
	}

	t.Fatal("PhimAPI result section was not returned")
}

func TestSearchMatchingContinuesThroughAllTMDBAliases(t *testing.T) {
	target := contracts.MediaItem{
		ID: "phimapi:naruto-main", Kind: "show", ExternalIDs: map[string]string{"tmdb": "46260"},
	}
	aliases := []string{"日本語別名", "Alias two", "Alias three", "Alias four"}
	source := &queryAwareSource{search: func(query string) ([]contracts.MediaItem, error) {
		if query == "Alias four" {
			return []contracts.MediaItem{target}, nil
		}
		return nil, nil
	}}

	results := New(source).SearchMatching(context.Background(), "Naruto", "show", 2002,
		aliases, map[string]string{"tmdb": "46260"})
	for _, result := range results {
		if result.SourceID == "phimapi" {
			if result.Status != "ok" || len(result.Items) != 1 ||
				result.Items[0].ID != target.ID || result.Items[0].MatchMethod != "external_id" {
				t.Fatalf("later TMDB alias did not find the exact PhimAPI item: %+v", result)
			}
			if len(source.queries) != len(phimAPIQueries("Naruto", aliases)) {
				t.Fatalf("not all unique aliases were queried: %v", source.queries)
			}
			return
		}
	}

	t.Fatal("PhimAPI result section was not returned")
}

func TestPhimAPIMatcherPreservesDistinctItemsWithSharedTMDBID(t *testing.T) {
	items := []contracts.MediaItem{
		{ID: "phimapi:loki-season-1", Title: "Loki mùa 1", Kind: "show", Year: 2021,
			ExternalIDs: map[string]string{"tmdb": "84958"}},
		{ID: "phimapi:loki-season-2", Title: "Loki mùa 2", Kind: "show", Year: 2023,
			ExternalIDs: map[string]string{"tmdb": "84958"}},
	}

	matches := matchPhimAPIItems(items, "Loki", nil, 2021, "show",
		map[string]string{"tmdb": "84958"})
	if len(matches) != 2 {
		t.Fatalf("expected both distinct PhimAPI entries for Loki, got %+v", matches)
	}
	for _, match := range matches {
		if match.MatchMethod != "external_id" {
			t.Fatalf("shared TMDB identity was not labeled as an ID match: %+v", match)
		}
	}
}

func TestAIDiscoveryReturnsUnverifiedCandidatesSeparately(t *testing.T) {
	source := &widePhimAPISource{search: func(query string, _ bool) ([]contracts.MediaItem, error) {
		if query != "ワンピース" {
			return nil, nil
		}
		return []contracts.MediaItem{
			{ID: "phimapi:one-piece", Title: "ワンピース", Kind: "show", Year: 1999},
			{ID: "phimapi:conflict", Title: "ワンピース", Kind: "show", Year: 1999,
				ExternalIDs: map[string]string{"tmdb": "999"}},
		}, nil
	}}

	result := New(source).SearchPhimAPIDiscovery(context.Background(), "One Piece",
		"show", 1999, []string{"One Piece"}, []string{"ワンピース"},
		map[string]string{"tmdb": "123"})
	if result.Status != "ok" || len(result.Items) != 1 ||
		result.Items[0].ID != "phimapi:one-piece" ||
		result.Items[0].MatchMethod != "ai_candidate" {
		t.Fatalf("AI-discovered candidate was not kept separate from verified matches: %+v", result)
	}
}

func TestPhimAPIQueriesSelectDistinctAnimeAliases(t *testing.T) {
	tests := []struct {
		name    string
		title   string
		aliases []string
		want    []string
	}{
		{
			name:    "Naruto",
			title:   "Naruto",
			aliases: []string{"ナルト", "Naruto", "Naruto Shippuden"},
			want:    []string{"Naruto", "ナルト", "Naruto Shippuden"},
		},
		{
			name:    "One Piece",
			title:   "One Piece",
			aliases: []string{"ワンピース", "One-Piece", "Đảo Hải Tặc"},
			want:    []string{"One Piece", "ワンピース", "Đảo Hải Tặc"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			queries := phimAPIQueries(test.title, test.aliases)
			if len(queries) != len(test.want) {
				t.Fatalf("unexpected query count: got %v, want %v", queries, test.want)
			}
			for index := range test.want {
				if queries[index] != test.want[index] {
					t.Fatalf("unexpected queries: got %v, want %v", queries, test.want)
				}
			}
		})
	}
}

func TestWidePhimAPISearchPropagatesAliasQueryError(t *testing.T) {
	source := &widePhimAPISource{search: func(query string, _ bool) ([]contracts.MediaItem, error) {
		if query == "Alias" {
			return nil, errors.New("provider unavailable")
		}
		return nil, nil
	}}
	results := New(source).SearchMatching(context.Background(), "Title", "show", 2025,
		[]string{"Alias"}, map[string]string{"tmdb": "123"})
	for _, result := range results {
		if result.SourceID == "phimapi" {
			if result.Status != "empty" || result.ErrorCode != "partial_search_failed" {
				t.Fatalf("wide alias query error was not surfaced: %+v", result)
			}
			return
		}
	}

	t.Fatal("PhimAPI result section was not returned")
}
