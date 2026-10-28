package watching

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	maxContinueItems = 8
	keepFor          = 14 * 24 * time.Hour
	minProgress      = 0.03
	maxProgress      = 0.95
	youtubePackage   = "com.google.android.youtube.tv"
	plexPackage      = "com.plexapp.android"
)

var youtubeVideoID = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

type OnDeckSource interface {
	OnDeck(ctx context.Context) ([]Item, error)
}

type Continue struct {
	history *History
	onDeck  OnDeckSource
	now     func() time.Time
}

func NewContinue(history *History, onDeck OnDeckSource) *Continue {
	return &Continue{history: history, onDeck: onDeck, now: time.Now}
}

func (c *Continue) Items(ctx context.Context) []Item {
	deck := c.deckItems(ctx)
	items := append([]Item{}, deck...)
	for _, event := range c.history.Since(c.now().Add(-keepFor)) {
		if !unfinished(event) || coveredByDeck(event, deck) {
			continue
		}
		items = append(items, itemFromEvent(event))
	}

	sort.SliceStable(items, func(i, j int) bool { return items[i].UpdatedAt > items[j].UpdatedAt })
	if len(items) > maxContinueItems {
		items = items[:maxContinueItems]
	}
	return items
}

func (c *Continue) deckItems(ctx context.Context) []Item {
	if c.onDeck == nil {
		return nil
	}
	deck, err := c.onDeck.OnDeck(ctx)
	if err != nil {
		return nil
	}
	return deck
}

func unfinished(event Event) bool {
	if event.DurationMs <= 0 {
		return true
	}
	watched := progressOf(event)
	return watched >= minProgress && watched <= maxProgress
}

func progressOf(event Event) float64 {
	if event.DurationMs <= 0 {
		return 0
	}
	watched := float64(event.PositionMs) / float64(event.DurationMs)
	if watched > 1 {
		return 1
	}
	return watched
}

func coveredByDeck(event Event, deck []Item) bool {
	if event.PackageName != plexPackage {
		return false
	}
	for _, item := range deck {
		if strings.EqualFold(item.Title, event.Title) ||
			strings.Contains(strings.ToLower(item.Subtitle), strings.ToLower(event.Title)) {
			return true
		}
	}
	return false
}

func itemFromEvent(event Event) Item {
	return Item{
		ID:          "tv:" + event.key(),
		Title:       event.Title,
		Subtitle:    event.Subtitle,
		AppLabel:    event.AppLabel,
		PackageName: event.PackageName,
		Progress:    progressOf(event),
		PosterURL:   event.ArtworkURL,
		Action:      actionFor(event),
		UpdatedAt:   event.At,
	}
}

func actionFor(event Event) Action {
	if event.PackageName == youtubePackage && youtubeVideoID.MatchString(event.MediaID) {
		return Action{
			ActionType: "youtube.launch",
			Parameters: map[string]any{"video_id": event.MediaID},
		}
	}
	return Action{
		ActionType: "app.open",
		Parameters: map[string]any{"package_name": event.PackageName},
	}
}
