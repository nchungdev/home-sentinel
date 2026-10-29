package skills

import (
	"testing"

	"github.com/nchungdev/home-sentinel/internal/contracts"
)

func titles(items []contracts.MediaItem) []string {
	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.Title)
	}
	return names
}

func TestRelevantFirstDropsUnrelatedWhenSomethingMatches(t *testing.T) {
	items := []contracts.MediaItem{
		{Title: "One Piece Film: Red"}, {Title: "Naruto Shippuden"},
		{Title: "Kindaichi Case Files"},
	}
	got := titles(relevantFirst("sonarr", "nauto", items))
	if len(got) != 1 || got[0] != "Naruto Shippuden" {
		t.Fatalf("want only Naruto for a one-letter typo, got %v", got)
	}
}

func TestRelevantFirstKeepsAllWhenNothingMatches(t *testing.T) {
	items := []contracts.MediaItem{{Title: "Đảo Hải Tặc"}, {Title: "Hoa Anh Đào"}}
	if got := relevantFirst("sonarr", "one piece", items); len(got) != 2 {
		t.Fatalf("translated titles must be kept, got %v", titles(got))
	}
}

func TestStrictSourcesDropUnrelatedLibraryHits(t *testing.T) {
	items := []contracts.MediaItem{
		{Title: "Đảo Hải Tặc", Original: "One Piece"}, {Title: "Madö King Granzört"},
	}
	if got := relevantFirst("plex", "nauto", items); len(got) != 0 {
		t.Fatalf("plex hits unrelated to the query must go, got %v", titles(got))
	}
	if got := titles(relevantFirst("plex", "one piece", items)); len(got) != 1 {
		t.Fatalf("original title should match, got %v", got)
	}
}
