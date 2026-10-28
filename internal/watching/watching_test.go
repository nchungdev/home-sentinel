package watching

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

type fakeDeck struct {
	items []Item
	err   error
}

func (f fakeDeck) OnDeck(context.Context) ([]Item, error) {
	return f.items, f.err
}

func event(pkg, title string, position, duration, at int64) Event {
	return Event{
		PackageName: pkg, AppLabel: "App", Title: title,
		PositionMs: position, DurationMs: duration, State: StatePaused, At: at,
	}
}

func TestHistoryKeepsNewestEventPerTitle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	history := NewHistory(path)
	now := time.Now().UnixMilli()

	if err := history.Record([]Event{event("a.pkg", "Show", 100, 1000, now-10)}); err != nil {
		t.Fatal(err)
	}
	if err := history.Record([]Event{event("a.pkg", "Show", 500, 1000, now)}); err != nil {
		t.Fatal(err)
	}

	reloaded := NewHistory(path).Since(time.Now().Add(-time.Hour))
	if len(reloaded) != 1 || reloaded[0].PositionMs != 500 {
		t.Fatalf("want one entry at 500, got %+v", reloaded)
	}
}

func TestHistoryKeepsPositionWhenSessionResetsToZero(t *testing.T) {
	history := NewHistory(filepath.Join(t.TempDir(), "history.json"))
	now := time.Now().UnixMilli()

	if err := history.Record([]Event{event("a.pkg", "Show", 450, 1000, now-10)}); err != nil {
		t.Fatal(err)
	}
	if err := history.Record([]Event{event("a.pkg", "Show", 0, 0, now)}); err != nil {
		t.Fatal(err)
	}

	recent := history.Since(time.Now().Add(-time.Hour))
	if len(recent) != 1 || recent[0].PositionMs != 450 || recent[0].DurationMs != 1000 {
		t.Fatalf("want position 450 of 1000 kept, got %+v", recent)
	}
}

func TestHistoryRejectsEventsWithoutTitle(t *testing.T) {
	history := NewHistory("")
	err := history.Record([]Event{{PackageName: "a.pkg"}})
	if !errors.Is(err, ErrNoValidEvents) {
		t.Fatalf("want ErrNoValidEvents, got %v", err)
	}
}

func TestContinueDropsFinishedAndStartedOnly(t *testing.T) {
	history := NewHistory("")
	now := time.Now().UnixMilli()
	_ = history.Record([]Event{
		event("a.pkg", "Half", 500, 1000, now),
		event("a.pkg", "Done", 990, 1000, now),
		event("a.pkg", "Barely", 10, 1000, now),
		event("a.pkg", "Live", 0, 0, now),
	})

	items := NewContinue(history, nil).Items(context.Background())
	titles := map[string]bool{}
	for _, item := range items {
		titles[item.Title] = true
	}
	if !titles["Half"] || !titles["Live"] || titles["Done"] || titles["Barely"] {
		t.Fatalf("unexpected titles %v", titles)
	}
}

func TestContinueMergesDeckAndSortsNewestFirst(t *testing.T) {
	history := NewHistory("")
	now := time.Now().UnixMilli()
	_ = history.Record([]Event{
		event(youtubePackage, "Clip", 300, 1000, now-1000),
		event(plexPackage, "The Expanse", 400, 1000, now),
	})
	deck := fakeDeck{items: []Item{{ID: "plex:1", Title: "The Expanse", UpdatedAt: now - 500}}}

	items := NewContinue(history, deck).Items(context.Background())
	if len(items) != 2 || items[0].ID != "plex:1" || items[1].Title != "Clip" {
		t.Fatalf("want deck item then YouTube clip, got %+v", items)
	}
}

func TestContinueIgnoresDeckErrors(t *testing.T) {
	history := NewHistory("")
	_ = history.Record([]Event{event("a.pkg", "Half", 500, 1000, time.Now().UnixMilli())})

	items := NewContinue(history, fakeDeck{err: errors.New("down")}).Items(context.Background())
	if len(items) != 1 {
		t.Fatalf("want local item despite deck error, got %+v", items)
	}
}

func TestYoutubeVideoGetsLaunchAction(t *testing.T) {
	clip := event(youtubePackage, "Clip", 300, 1000, 1)
	clip.MediaID = "aqz-KE-bpKQ"
	if action := actionFor(clip); action.ActionType != "youtube.launch" {
		t.Fatalf("want youtube.launch, got %+v", action)
	}

	other := event("com.netflix.ninja", "Film", 300, 1000, 1)
	if action := actionFor(other); action.ActionType != "app.open" {
		t.Fatalf("want app.open, got %+v", action)
	}
}
