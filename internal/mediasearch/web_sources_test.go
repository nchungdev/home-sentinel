package mediasearch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestYouTubeSearchIncludesBestAvailableThumbnail(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/search" || request.URL.Query().Get("q") != "running man" {
			t.Errorf("unexpected YouTube request: %s", request.URL.String())
		}
		_, _ = writer.Write([]byte(`{"items":[
{"id":{"videoId":"video-1"},"snippet":{"title":"Episode 1","description":"Description",
"thumbnails":{"medium":{"url":"https://i.ytimg.com/vi/video-1/mqdefault.jpg"},
"high":{"url":"https://i.ytimg.com/vi/video-1/hqdefault.jpg"}}}},
{"id":{"videoId":"video-2"},"snippet":{"title":"Episode 2",
"thumbnails":{"default":{"url":"https://i.ytimg.com/vi/video-2/default.jpg"},
"maxres":{"url":"javascript:alert(1)"}}}}]}`))
	}))
	defer provider.Close()

	source := &youtubeSource{&httpSource{id: "youtube", name: "YouTube", kind: "video",
		baseURL: provider.URL, apiKey: "test-key", client: provider.Client()}}
	items, err := source.Search(context.Background(), "running man")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].Poster != "https://i.ytimg.com/vi/video-1/hqdefault.jpg" ||
		items[1].Poster != "https://i.ytimg.com/vi/video-2/default.jpg" {
		t.Fatalf("YouTube thumbnails were not mapped safely: %+v", items)
	}
}

func TestYouTubeCapabilityRequiresAPIKey(t *testing.T) {
	source := &youtubeSource{&httpSource{id: "youtube", name: "YouTube", kind: "video",
		baseURL: "https://www.googleapis.com/youtube/v3"}}

	capability := source.Capability(context.Background())
	if capability.Status != "not_configured" || capability.Available {
		t.Fatalf("unexpected capability without API key: %+v", capability)
	}
}
