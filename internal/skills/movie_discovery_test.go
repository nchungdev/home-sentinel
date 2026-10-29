package skills

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/nchungdev/home-sentinel/internal/contracts"
)

type movieDiscoveryAssistant struct {
	response string
	err      error
}

func (a movieDiscoveryAssistant) StreamChatContext(
	_ context.Context,
	_ string,
	onChunk func(string),
) error {
	if a.err != nil {
		return a.err
	}
	onChunk(a.response)

	return nil
}

type movieDiscoverySearcher struct {
	title       string
	kind        string
	year        int
	aliases     []string
	queries     []string
	externalIDs map[string]string
}

func (s *movieDiscoverySearcher) SearchPhimAPIDiscovery(
	_ context.Context,
	title, kind string,
	year int,
	aliases, queries []string,
	externalIDs map[string]string,
) contracts.MediaSourceResults {
	s.title, s.kind, s.year = title, kind, year
	s.aliases, s.queries, s.externalIDs = aliases, queries, externalIDs

	return contracts.MediaSourceResults{SourceID: "phimapi", Status: "empty"}
}

func TestMovieSearchDiscoveryUsesValidatedAIQueries(t *testing.T) {
	searcher := &movieDiscoverySearcher{}
	detail := TMDBDetail{
		Title: "Naruto", Kind: "show", Year: 2002,
		Aliases:     []string{"ナルト", "Naruto"},
		ExternalIDs: map[string]string{"tmdb": "46260"},
	}
	response, err := json.Marshal(map[string][]string{"queries": {
		"Naruto", "ナルト", "NARUTO Shippuden", "Alias 2", "Alias 3", "Alias 4",
		"Alias 5", "Alias 6",
	}})
	if err != nil {
		t.Fatal(err)
	}
	result, code := (MovieSearchDiscovery{
		Assistant: movieDiscoveryAssistant{response: string(response)},
		Searcher:  searcher,
	}).Search(context.Background(), detail)

	if code != "" || result.SourceID != "phimapi" {
		t.Fatalf("AI query discovery failed: %+v, %q", result, code)
	}
	if searcher.title != detail.Title || searcher.kind != detail.Kind ||
		searcher.year != detail.Year || searcher.externalIDs["tmdb"] != "46260" {
		t.Fatalf("TMDB identity was not passed to the matcher: %+v", searcher)
	}
	if len(searcher.aliases) != len(detail.Aliases) ||
		searcher.aliases[0] != detail.Aliases[0] {
		t.Fatalf("AI suggestions were incorrectly promoted to trusted aliases: %v",
			searcher.aliases)
	}
	if len(searcher.queries) != movieDiscoveryQueryLimit ||
		searcher.queries[0] != "NARUTO Shippuden" ||
		searcher.queries[4] != "Alias 5" {
		t.Fatalf("AI suggestions were not deduplicated and bounded: %v", searcher.queries)
	}
}

func TestMovieSearchDiscoveryRejectsInvalidAIOutput(t *testing.T) {
	searcher := &movieDiscoverySearcher{}
	result, code := (MovieSearchDiscovery{
		Assistant: movieDiscoveryAssistant{response: "not json"},
		Searcher:  searcher,
	}).Search(context.Background(), TMDBDetail{Title: "Naruto", Kind: "show"})

	if code != "ai_invalid_response" || result.Status != "error" {
		t.Fatalf("invalid AI response was not reported: %+v, %q", result, code)
	}
	if len(searcher.queries) != 0 {
		t.Fatalf("invalid AI response reached PhimAPI search: %v", searcher.queries)
	}
}

func TestMovieSearchDiscoveryReportsAssistantFailure(t *testing.T) {
	searcher := &movieDiscoverySearcher{}
	result, code := (MovieSearchDiscovery{
		Assistant: movieDiscoveryAssistant{err: errors.New("unavailable")},
		Searcher:  searcher,
	}).Search(context.Background(), TMDBDetail{Title: "Naruto", Kind: "show"})

	if code != "ai_search_failed" || result.Status != "error" {
		t.Fatalf("assistant failure was not surfaced: %+v, %q", result, code)
	}
}

func TestMovieSearchDiscoverySkipsDuplicateSuggestions(t *testing.T) {
	searcher := &movieDiscoverySearcher{}
	result, code := (MovieSearchDiscovery{
		Assistant: movieDiscoveryAssistant{response: `{"queries":["Naruto","ナルト"]}`},
		Searcher:  searcher,
	}).Search(context.Background(), TMDBDetail{
		Title: "Naruto", Kind: "show", Aliases: []string{"ナルト"},
	})

	if code != "" || result.ErrorCode != "ai_no_new_queries" || result.Status != "empty" {
		t.Fatalf("duplicate-only suggestions were not reported: %+v, %q", result, code)
	}
	if len(searcher.queries) != 0 {
		t.Fatalf("duplicate suggestions triggered another search: %v", searcher.queries)
	}
}
