package mediasearch

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestPhimAPILiveSearch(t *testing.T) {
	if os.Getenv("RUN_LIVE_CATALOG_TESTS") != "1" {
		t.Skip("explicit live catalog verification only")
	}
	source := &phimAPISource{&httpSource{id: "phimapi", name: "PhimAPI", kind: "catalog",
		baseURL: "https://phimapi.com", client: &http.Client{Timeout: 5 * time.Second}}}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	result := New(source).SearchSource(ctx, "phimapi", "naruto")
	if result.Status != "ok" || len(result.Items) == 0 {
		t.Fatalf("live source did not return usable results: %s / %s",
			result.Status, result.ErrorCode)
	}
	t.Logf("PhimAPI live: %d results; first title: %s", len(result.Items), result.Items[0].Title)
}
