package mediasearch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPhimAPISearchValidatesAndMapsMetadata(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/api/home" {
			_, _ = w.Write([]byte(`{"status":"success","data":{"items":[]}}`))
			return
		}
		if r.URL.Query().Get("keyword") != "naruto" {
			t.Error("search did not receive normalized title")
		}
		_, _ = w.Write([]byte(`{"status":"success","data":{"items":[
{"name":"Naruto Shippuden","origin_name":"Naruto","slug":"naruto-shippuden",
"year":2007,"type":"series","lang":"Vietsub","episode_current":"Tập 500"},
{"name":"Unsafe slug","slug":"../unexpected"}]}}`))
	}))
	defer provider.Close()
	source := &phimAPISource{&httpSource{id: "phimapi", name: "PhimAPI", kind: "catalog",
		baseURL: provider.URL, client: provider.Client()}}
	service := New(source)
	result := service.SearchSource(context.Background(), "phimapi", "naruto")
	if result.Status != "ok" || len(result.Items) != 1 ||
		result.Items[0].ID != "phimapi:naruto-shippuden" || result.Items[0].Kind != "show" {
		t.Fatalf("invalid PhimAPI mapping: %+v", result)
	}
	if !service.Configured("phimapi") || service.Configured("motchill") {
		t.Fatal("configured source discovery is incorrect")
	}
}

func TestConfiguredSourcesExcludeMissingKeysAndPlaceholders(t *testing.T) {
	service := New(
		&plexSource{httpSource: &httpSource{id: "plex", baseURL: "http://127.0.0.1:32400",
			keyHeader: "X-Plex-Token"}},
		placeholderSource{id: "motchill"},
		&youtubeSource{&httpSource{id: "youtube", baseURL: "https://www.googleapis.com"}},
	)
	for _, source := range []string{"plex", "motchill", "youtube"} {
		if service.Configured(source) {
			t.Fatalf("source without implementation or key was enabled: %s", source)
		}
	}
}
