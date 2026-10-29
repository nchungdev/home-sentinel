package skills

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nchungdev/home-sentinel/internal/contracts"
	"github.com/nchungdev/home-sentinel/internal/requestflow"
	websearch "github.com/nchungdev/home-sentinel/internal/search/web"
)

type fakeMedia struct {
	sources []string
}

func (s *fakeMedia) SearchSource(
	ctx context.Context, source, query string,
) contracts.MediaSourceResults {
	s.sources = append(s.sources, source)
	return contracts.MediaSourceResults{SourceID: source, Status: "ok",
		Items: []contracts.MediaItem{{ID: "1", Title: query}}}
}

func TestMediaSkillQueriesOnlySelectedSource(t *testing.T) {
	service := &fakeMedia{}
	result := (MediaSearch{Service: service}).Execute(context.Background(), requestflow.Task{
		Source: "plex", Query: "Interstellar",
	})
	if len(service.sources) != 1 || service.sources[0] != "plex" || len(result.Items) != 1 {
		t.Fatalf("media skill fanned out outside its task: %+v, %+v", service.sources, result)
	}
}

func TestOfficialDocumentationFiltersUntrustedHosts(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"results":[
{"title":"Official","url":"https://learn.microsoft.com/windows"},
{"title":"Spoofed","url":"https://learn.microsoft.com.evil.example/windows"}]}`))
	}))
	defer provider.Close()
	skill := DocumentationSearch{Searcher: websearch.New(provider.URL, nil)}
	result := skill.Execute(context.Background(), requestflow.Task{Query: "sửa Windows"})
	if len(result.Items) != 1 || result.Items[0].Title != "Official" {
		t.Fatalf("official source label included a spoofed host: %+v", result)
	}
}

func TestMovieMetadataIncludesDirectorAndDoesNotLaunchPlayback(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-only-token" {
			t.Error("missing provider auth header")
		}
		if r.URL.Path == "/search/movie" {
			if r.URL.Query().Get("query") != "Interstellar" {
				t.Error("metadata did not receive normalized title")
			}
			_, _ = w.Write([]byte(`{"results":[{"id":157336,"media_type":"movie",
"title":"Interstellar","release_date":"2014-11-05","overview":"Synopsis",
"poster_path":"/gEU2QniE6E77NI6lCU6MxlNBvIx.jpg"}]}`))
			return
		}
		if r.URL.Path == "/search/tv" {
			_, _ = w.Write([]byte(`{"results":[]}`))
			return
		}
		_, _ = w.Write([]byte(`{"overview":"Synopsis","credits":{"crew":[
{"name":"Christopher Nolan","job":"Director"}]}}`))
	}))
	defer provider.Close()
	skill := MovieMetadata{
		Token: "test-only-token", BaseURL: provider.URL, Client: provider.Client(),
	}
	result := skill.Execute(context.Background(), requestflow.Task{
		Query: "Interstellar", Strategy: "tmdb_search",
	})
	if result.Status != "ok" || len(result.Items) != 1 ||
		result.Items[0].Content != "Synopsis" || result.Items[0].Year != 2014 ||
		result.Items[0].Kind != "movie" ||
		result.Items[0].Poster == "" || result.Items[0].ExternalIDs["tmdb"] != "157336" ||
		len(result.Items[0].Actions) != 1 || result.Items[0].Actions[0].Type != "media_detail" {
		t.Fatalf("incorrect metadata behavior: %+v", result)
	}
}

func TestMovieMetadataSearchStripsSeasonQualifierBeforeTMDBLookup(t *testing.T) {
	queries := []string{}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		queries = append(queries, query)
		if query != "naruto" {
			t.Errorf("TMDB search did not remove the season qualifier: %q", query)
		}
		if r.URL.Path == "/search/tv" {
			_, _ = w.Write([]byte(`{"results":[{"id":46260,
"name":"Naruto","first_air_date":"2002-10-03"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer provider.Close()

	result := (MovieMetadata{
		APIKey: "test-key", BaseURL: provider.URL, Client: provider.Client(),
	}).Execute(context.Background(), requestflow.Task{
		Query: "naruto phần 1", Strategy: "tmdb_search",
	})
	if result.Status != "ok" || len(result.Items) != 1 ||
		result.Items[0].Title != "Naruto" || len(queries) != 2 ||
		queries[0] != "naruto" || queries[1] != "naruto" {
		t.Fatalf("season-qualified query did not find the TMDB series: %+v, %v",
			result, queries)
	}
}

func TestTMDBSearchRetriesOriginalQueryAfterTitleSearchMiss(t *testing.T) {
	queries := []string{}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		if r.URL.Path == "/search/tv" {
			queries = append(queries, query)
			if query == "Naruto" {
				_, _ = w.Write([]byte(`{"results":[]}`))
				return
			}
			if query != "Naruto phần 1" {
				t.Errorf("unexpected TMDB fallback query: %q", query)
			}
			_, _ = w.Write([]byte(`{"results":[{"id":46260,
"name":"Naruto","first_air_date":"2002-10-03"}]}`))
			return
		}
		if query == "Naruto" {
			_, _ = w.Write([]byte(`{"results":[]}`))
			return
		}
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer provider.Close()

	result := (MovieMetadata{
		APIKey: "test-key", BaseURL: provider.URL, Client: provider.Client(),
	}).Execute(context.Background(), requestflow.Task{
		Query: "Naruto phần 1", Strategy: "tmdb_search",
	})
	if result.Status != "ok" || len(result.Items) != 1 ||
		len(queries) != 2 || queries[0] != "Naruto" || queries[1] != "Naruto phần 1" {
		t.Fatalf("original query fallback was not attempted: %+v, %v", result, queries)
	}
}

func TestMovieMetadataDetailReturnsStableProviderIDs(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tv/1399" ||
			r.URL.Query().Get("append_to_response") != "external_ids,alternative_titles" {
			t.Errorf("unexpected TMDB detail request: %s", r.URL.String())
		}
		_, _ = w.Write([]byte(`{"id":1399,"name":"Game of Thrones","first_air_date":"2011-04-17",
"original_name":"Game of Thrones",
"overview":"Synopsis","poster_path":"/poster.jpg",
"external_ids":{"imdb_id":"tt0944947","tvdb_id":121361},
"alternative_titles":{"results":[
{"title":"A Guerra dos Tronos","iso_3166_1":"BR"},
{"title":"Игра престолов","iso_3166_1":"US"},
{"title":"Trò chơi vương quyền","iso_3166_1":"VN"}]}}`))
	}))
	defer provider.Close()
	skill := MovieMetadata{Token: "test-key", BaseURL: provider.URL, Client: provider.Client()}

	detail, code := skill.Detail(context.Background(), "tv", 1399)
	if code != "" || detail.Kind != "show" || detail.Title != "Game of Thrones" ||
		detail.ExternalIDs["tmdb"] != "1399" || detail.ExternalIDs["imdb"] != "tt0944947" ||
		detail.ExternalIDs["tvdb"] != "121361" || len(detail.Aliases) != 4 ||
		detail.Aliases[1] != "Trò chơi vương quyền" ||
		detail.Aliases[2] != "Игра престолов" ||
		detail.Aliases[3] != "A Guerra dos Tronos" {
		t.Fatalf("TMDB detail returned incomplete matching identity: %+v, %s", detail, code)
	}
}

func TestMovieMetadataDetailRequiresConfiguredToken(t *testing.T) {
	detail, code := (MovieMetadata{}).Detail(context.Background(), "movie", 1)
	if code != "not_configured" || detail.ID != "" {
		t.Fatalf("missing token did not fail clearly: %+v, %q", detail, code)
	}
}

func TestMovieMetadataUsesTMDBAPIKeyQueryAuthentication(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("api_key") != "test-api-key" ||
			r.Header.Get("Authorization") != "" {
			t.Errorf("TMDB API key was not sent with query authentication")
		}
		_, _ = w.Write([]byte(`{"id":550,"title":"Fight Club",
"release_date":"1999-10-15"}`))
	}))
	defer provider.Close()

	detail, code := (MovieMetadata{
		APIKey: "test-api-key", BaseURL: provider.URL, Client: provider.Client(),
	}).Detail(context.Background(), "movie", 550)
	if code != "" || detail.Title != "Fight Club" {
		t.Fatalf("TMDB API key auth failed: %+v, %q", detail, code)
	}
}
