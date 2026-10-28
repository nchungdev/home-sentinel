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
		if r.URL.Path == "/search/multi" {
			if r.URL.Query().Get("query") != "Interstellar" {
				t.Error("metadata did not receive normalized title")
			}
			_, _ = w.Write([]byte(`{"results":[{"id":157336,"media_type":"movie",
"title":"Interstellar","release_date":"2014-11-05","overview":"Synopsis"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"overview":"Synopsis","credits":{"crew":[
{"name":"Christopher Nolan","job":"Director"}]}}`))
	}))
	defer provider.Close()
	skill := MovieMetadata{
		Token: "test-only-token", BaseURL: provider.URL, Client: provider.Client(),
	}
	result := skill.Execute(context.Background(), requestflow.Task{Query: "Interstellar"})
	if result.Status != "ok" || len(result.Items) != 1 ||
		result.Items[0].Content != "Đạo diễn: Christopher Nolan\nSynopsis" ||
		len(result.Items[0].Actions) != 0 {
		t.Fatalf("incorrect metadata behavior: %+v", result)
	}
}
