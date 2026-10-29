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
"year":2007,"type":"series","lang":"Vietsub","episode_current":"Tập 500",
"tmdb":{"id":"123","type":"tv"},"imdb":{"id":"tt456"},
"alternative_names":["ナルト 疾風伝"]},
{"name":"Naruto Movie","slug":"naruto-movie","year":2004,"type":"hoathinh",
"episode_current":"Full","tmdb":{"id":456,"type":"movie"}},
{"name":"Naruto Anime","slug":"naruto-anime","year":2002,"type":"hoathinh",
"episode_current":"Hoàn Tất (220/220)"},
{"name":"Unsafe slug","slug":"../unexpected"}]}}`))
	}))
	defer provider.Close()
	source := &phimAPISource{&httpSource{id: "phimapi", name: "PhimAPI", kind: "catalog",
		baseURL: provider.URL, client: provider.Client()}}
	service := New(source)
	result := service.SearchSource(context.Background(), "phimapi", "naruto")
	if result.Status != "ok" || len(result.Items) != 3 ||
		result.Items[0].ID != "phimapi:naruto-shippuden" || result.Items[0].Kind != "show" {
		t.Fatalf("invalid PhimAPI mapping: %+v", result)
	}
	if result.Items[1].Kind != "movie" || result.Items[2].Kind != "show" ||
		result.Items[1].ExternalIDs["tmdb"] != "456" {
		t.Fatalf("invalid PhimAPI movie/series classification: %+v", result.Items)
	}
	if result.Items[0].ExternalIDs["tmdb"] != "123" ||
		result.Items[0].ExternalIDs["imdb"] != "tt456" ||
		result.Items[0].Original != "Naruto" ||
		len(result.Items[0].AlternativeTitles) != 1 {
		t.Fatalf("PhimAPI identity metadata was not preserved: %+v", result.Items[0])
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
