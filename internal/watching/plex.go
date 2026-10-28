package watching

import (
	"context"
	"fmt"
	"strings"

	"github.com/nchungdev/home-sentinel/internal/integrations/mediabridge"
)

type PlexOnDeck struct {
	bridge *mediabridge.Bridge
}

func NewPlexOnDeck(bridge *mediabridge.Bridge) *PlexOnDeck {
	return &PlexOnDeck{bridge: bridge}
}

func (p *PlexOnDeck) OnDeck(ctx context.Context) ([]Item, error) {
	token := mediabridge.ReadPlexToken()
	entries, err := p.bridge.PlexOnDeck(ctx, token)
	if err != nil {
		return nil, err
	}

	items := make([]Item, 0, len(entries))
	for _, entry := range entries {
		items = append(items, p.item(entry, token))
	}
	return items, nil
}

func (p *PlexOnDeck) item(entry mediabridge.OnDeckEntry, token string) Item {
	title, subtitle, poster := entry.Title, movieSubtitle(entry), entry.Thumb
	if entry.Type == "episode" && entry.ShowTitle != "" {
		title, subtitle = entry.ShowTitle, episodeSubtitle(entry)
		if entry.ShowThumb != "" {
			poster = entry.ShowThumb
		}
	}

	return Item{
		ID:          "plex:" + entry.RatingKey,
		Title:       title,
		Subtitle:    subtitle,
		AppLabel:    "Plex",
		PackageName: plexPackage,
		Progress:    deckProgress(entry),
		PosterURL:   p.bridge.PlexImageURL(poster, token),
		RemainingMs: remaining(entry.ViewOffset, entry.Duration),
		Action: Action{
			ActionType: "plex.open",
			Parameters: map[string]any{"uri": "plex://server/play?key=" + entry.RatingKey},
		},
		UpdatedAt: entry.LastViewedAt * 1000,
	}
}

func episodeSubtitle(entry mediabridge.OnDeckEntry) string {
	parts := []string{fmt.Sprintf("S%d · E%d", entry.Season, entry.Episode)}
	if entry.Title != "" {
		parts = append(parts, entry.Title)
	}
	return strings.Join(parts, " · ")
}

func movieSubtitle(entry mediabridge.OnDeckEntry) string {
	if entry.Year == 0 {
		return ""
	}
	return fmt.Sprint(entry.Year)
}

func deckProgress(entry mediabridge.OnDeckEntry) float64 {
	if entry.Duration <= 0 {
		return 0
	}
	return float64(entry.ViewOffset) / float64(entry.Duration)
}
