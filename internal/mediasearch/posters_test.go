package mediasearch

import (
	"strings"
	"testing"
)

func TestPhimAPIImageJoinsRelativePaths(t *testing.T) {
	got := phimAPIImage("https://img.example/", "/upload/a.jpg")
	if got != "https://img.example/upload/a.jpg" {
		t.Fatalf("relative poster: %q", got)
	}
	if got := phimAPIImage("", "", "https://cdn/x.jpg"); got != "https://cdn/x.jpg" {
		t.Fatalf("absolute thumb fallback: %q", got)
	}
}

func TestArrPosterPrefersRemotePoster(t *testing.T) {
	entry := map[string]any{"images": []any{
		map[string]any{"coverType": "fanart", "remoteUrl": "https://f"},
		map[string]any{"coverType": "poster", "remoteUrl": "https://p"},
	}}
	if got := arrPoster(entry); got != "https://p" {
		t.Fatalf("poster from images: %q", got)
	}
	entry["remotePoster"] = "https://r"
	if got := arrPoster(entry); got != "https://r" {
		t.Fatalf("remotePoster: %q", got)
	}
}

func TestPlexPosterUsesPublicURLAndTranscode(t *testing.T) {
	source := &plexSource{httpSource: &httpSource{publicURL: "http://nas:32400", apiKey: "k"}}
	got := source.poster("/library/metadata/1/thumb/2")
	if !strings.HasPrefix(got, "http://nas:32400/photo/:/transcode?") || !strings.Contains(got, "url=") {
		t.Fatalf("plex poster: %q", got)
	}
}
