package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/nchungdev/home-sentinel/internal/cache"
)

func catalogFixture(t *testing.T) (*Service, *atomic.Int32, *httptest.Server) {
	t.Helper()
	calls := &atomic.Int32{}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/phim/series" {
			t.Errorf("unexpected catalog path: %s", r.URL.Path)
		}
		generation := calls.Add(1)
		_, _ = fmt.Fprintf(w, `{"status":true,"movie":{"name":"Series","slug":"series",
"type":"series","year":2024},"episodes":[{"server_name":"Vietsub","server_data":[
{"name":"Tập đặc biệt Tết","slug":"tet","link_m3u8":"https://cdn.example/tet.m3u8?v=%d"},
{"name":"Tập 002","slug":"tap-002","link_m3u8":"https://cdn.example/2.m3u8"}]},
{"server_name":"Lồng tiếng","server_data":[
{"name":"Tập đặc biệt Tết","slug":"tet","link_m3u8":"https://cdn.example/dubbed.m3u8"}]}]}`,
			generation)
	}))
	service := &Service{Sources: map[string]Source{"phimapi": NewPhimAPI(provider.URL)},
		Cache: cache.NewMemory(10)}

	return service, calls, provider
}

func TestTitlePreservesLabelsAndDoesNotLeakStreamURLs(t *testing.T) {
	service, calls, provider := catalogFixture(t)
	defer provider.Close()
	title, err := service.GetTitle(context.Background(), "phimapi:series")
	if err != nil || len(title.Episodes) != 3 || len(title.Seasons) != 0 || title.Kind != "series" {
		t.Fatalf("invalid catalog title: %+v, %v", title, err)
	}
	if title.Episodes[0].Label != "Tập đặc biệt Tết" || title.Episodes[1].Order != 2 ||
		title.Episodes[0].ID == title.Episodes[2].ID {
		t.Fatal("lost special label, source order or server identity")
	}
	serialized, _ := json.Marshal(title)
	if strings.Contains(string(serialized), "cdn.example") {
		t.Fatal("title metadata contains upstream playback URL")
	}
	_, _ = service.GetTitle(context.Background(), "phimapi:series")
	if calls.Load() != 1 {
		t.Fatal("normalized title metadata was not cached")
	}
}

func TestResolveRefreshesURLAndKeepsEpisodeIdentity(t *testing.T) {
	service, calls, provider := catalogFixture(t)
	defer provider.Close()
	title, _ := service.GetTitle(context.Background(), "phimapi:series")
	identifier := title.Episodes[0].ID
	first, err := service.ResolveEpisode(context.Background(), identifier)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.ResolveEpisode(context.Background(), identifier)
	if err != nil || first.URL == second.URL || first.Episode.ID != second.Episode.ID ||
		calls.Load() != 3 {
		t.Fatalf("stream URLs were reused as episode identity: %+v, %+v, %v", first, second, err)
	}
}

func TestForgedEpisodeAndUnsafeTitleIDsAreRejected(t *testing.T) {
	service, calls, provider := catalogFixture(t)
	defer provider.Close()
	for _, id := range []string{"phimapi:../private", "phimapi:/admin", "phimapi:bad?query"} {
		if _, err := service.GetTitle(context.Background(), id); !errors.Is(err, ErrInvalidID) {
			t.Fatalf("unsafe title ID accepted: %s, %v", id, err)
		}
	}
	if calls.Load() != 0 {
		t.Fatal("invalid IDs reached provider")
	}
	identifier := episodeID("phimapi", "series", "x", "tet")
	_, err := service.ResolveEpisode(context.Background(), identifier)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("forged server identity accepted: %v", err)
	}
}

func TestEmbeddedPagesAreNotAdvertisedAsDirectPlayback(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":true,"movie":{"name":"Movie","slug":"movie"},
"episodes":[{"server_name":"Embed","server_data":[{"name":"Full","slug":"full",
"link_embed":"https://embed.example/page","link_m3u8":""}]}]}`))
	}))
	defer provider.Close()
	source := NewPhimAPI(provider.URL)
	title, err := source.GetTitle(context.Background(), "phimapi:movie")
	if err != nil || len(title.Episodes) != 1 || title.Episodes[0].Playable {
		t.Fatalf("embed-only source marked playable: %+v, %v", title, err)
	}
	_, err = source.ResolveEpisode(context.Background(), title.Episodes[0].ID)
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("embed link accepted as stream: %v", err)
	}
}
