package catalog

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestPhimAPILiveSeries(t *testing.T) {
	if os.Getenv("RUN_LIVE_CATALOG_TESTS") != "1" {
		t.Skip("explicit public metadata verification only")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	source := NewPhimAPI("https://phimapi.com")
	title, err := source.GetTitle(ctx, "phimapi:naruto-shippuden")
	if err != nil || len(title.Episodes) < 100 {
		t.Fatalf("live series metadata failed: episodes=%d, error=%v", len(title.Episodes), err)
	}
	stream, err := source.ResolveEpisode(ctx, title.Episodes[0].ID)
	if err != nil || stream.Episode.ID != title.Episodes[0].ID || !supportedStream(stream.URL) {
		t.Fatalf("live episode resolution failed: %v", err)
	}
	t.Logf("%s: %d episodes, first label %s, stable ID retained after resolution",
		title.Name, len(title.Episodes), stream.Episode.Label)
}
