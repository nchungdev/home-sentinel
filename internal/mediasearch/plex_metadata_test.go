package mediasearch

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nchungdev/home-sentinel/internal/contracts"
)

func TestPlexSearchMatchesTMDBDespiteDifferentTitle(t *testing.T) {
	for _, hydrate := range []bool{false, true} {
		t.Run(fmt.Sprintf("hydrate=%t", hydrate), func(t *testing.T) {
			metadataRequests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/library/sections":
					fmt.Fprint(w, `{"MediaContainer":{"Directory":[]}}`)
				case "/identity":
					fmt.Fprint(w, `{"MediaContainer":{"machineIdentifier":"server"}}`)
				case "/hubs/search":
					if r.URL.Query().Get("includeGuids") != "1" ||
						r.URL.Query().Get("includeExternalMedia") != "0" {
						t.Error("Plex search must request provider IDs and local media only")
					}
					guids := `,"Guid":[{"id":"tmdb://317442"},{"id":"imdb://tt3717532"}]`
					if hydrate {
						guids = ""
					}
					fmt.Fprintf(w, `{"MediaContainer":{"Hub":[{"Metadata":[`+
						`{"ratingKey":"8253","type":"movie","guid":"plex://movie/naruto",`+
						`"title":"THE LAST -NARUTO THE MOVIE-"%s}`+
						`]}]}}`, guids)
				case "/library/metadata/8253":
					metadataRequests++
					fmt.Fprint(w, `{"MediaContainer":{"Metadata":[{"ratingKey":"8253",`+
						`"type":"movie","guid":"plex://movie/naruto",`+
						`"Guid":[{"id":"tmdb://317442"}]}]}}`)
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			source := &plexSource{httpSource: &httpSource{
				id: "plex", baseURL: server.URL, probePath: "/library/sections", client: server.Client(),
			}}
			results := New(source).SearchMatching(context.Background(),
				"Naruto Shippuden Movie 7: Trận Chiến Cuối Cùng", "movie", 2014, nil,
				map[string]string{"tmdb": "317442", "imdb": "tt3717532"})
			var result contracts.MediaSourceResults
			for _, candidate := range results {
				if candidate.SourceID == "plex" {
					result = candidate
				}
			}
			if result.Status != "ok" || len(result.Items) != 1 ||
				result.Items[0].ID != "8253" || result.Items[0].MatchMethod != "external_id" {
				t.Fatalf("TMDB identity not resolved: %+v", result)
			}
			expectedRequests := 0
			if hydrate {
				expectedRequests = 1
			}
			if metadataRequests != expectedRequests {
				t.Fatalf("metadata requests: got %d, want %d", metadataRequests, expectedRequests)
			}
		})
	}
}
