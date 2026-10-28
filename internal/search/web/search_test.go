package websearch

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nchungdev/home-sentinel/internal/cache"
)

func TestSearchParsesLinksAndCachesSuccessfulResponse(t *testing.T) {
	var calls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/search" || r.URL.Query().Get("q") != "cách sửa Windows" ||
			r.URL.Query().Get("format") != "json" {
			t.Errorf("invalid provider query: %s", r.URL)
		}
		_, _ = w.Write([]byte(`{"results":[
{"title":"Docs","url":"https://learn.microsoft.com/windows","content":"Guide"},
{"title":"Duplicate","url":"https://learn.microsoft.com/windows"},
{"title":"Bad","url":"javascript:alert(1)"},
{"title":"Credential URL","url":"https://user:secret@example.com/"}]}`))
	}))
	defer provider.Close()
	search := New(provider.URL, cache.NewMemory(10))
	for index, expectedCache := range []string{"miss", "hit"} {
		links, cached, err := search.Search(context.Background(), "cách sửa Windows")
		if err != nil || cached != expectedCache || len(links) != 1 || links[0].Title != "Docs" {
			t.Fatalf("request %d: %+v, %s, %v", index, links, cached, err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("cache hit still called provider: %d", calls.Load())
	}
}

func TestErrorsAreNotCachedAsEmpty(t *testing.T) {
	var calls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer provider.Close()
	search := New(provider.URL, cache.NewMemory(10))
	if _, _, err := search.Search(context.Background(), "movie"); !errors.Is(err, ErrAuthRequired) {
		t.Fatalf("expected auth error: %v", err)
	}
	links, _, err := search.Search(context.Background(), "movie")
	if err != nil || len(links) != 0 || calls.Load() != 2 {
		t.Fatal("provider error became cached empty results")
	}
}

type brokenCache struct{}

func (brokenCache) Get(context.Context, string) ([]byte, error) {
	return nil, errors.New("redis unavailable")
}
func (brokenCache) Set(context.Context, string, []byte, time.Duration) error {
	return errors.New("redis unavailable")
}

func TestUnavailableCacheDoesNotBlockSource(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"results":[{"title":"Page","url":"https://example.com"}]}`))
	}))
	defer provider.Close()
	search := New(provider.URL, brokenCache{})
	links, _, err := search.Search(context.Background(), "query")
	if err != nil || len(links) != 1 {
		t.Fatalf("cache failure blocked search: %v", err)
	}
}

func TestInvalidResponseAndMissingConfiguration(t *testing.T) {
	if _, _, err := New("", nil).Search(context.Background(), "query"); err != ErrNotConfigured {
		t.Fatalf("missing config error: %v", err)
	}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"error":"not a search result"}`))
	}))
	defer provider.Close()
	if _, _, err := New(provider.URL, nil).Search(context.Background(), "q"); err != ErrInvalid {
		t.Fatalf("invalid response became empty results: %v", err)
	}
}
