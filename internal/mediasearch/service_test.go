package mediasearch

import (
	"context"
	"errors"
	"testing"

	"github.com/nchungdev/home-sentinel/internal/contracts"
)

type controlledSource struct {
	id       string
	probe    func(context.Context)
	search   func(context.Context) ([]contracts.MediaItem, error)
	disabled bool
}

func (s controlledSource) Info() contracts.MediaSourceCapability {
	return contracts.MediaSourceCapability{ID: s.id, Status: "checking"}
}

func (s controlledSource) Capability(ctx context.Context) contracts.MediaSourceCapability {
	if s.probe != nil {
		s.probe(ctx)
	}
	return contracts.MediaSourceCapability{ID: s.id, Available: !s.disabled, Status: "ready"}
}

func (s controlledSource) Search(ctx context.Context, _ string) ([]contracts.MediaItem, error) {
	return s.search(ctx)
}

func TestStreamsFastSourceBeforeSlowDiscovery(t *testing.T) {
	release := make(chan struct{})
	fast := controlledSource{id: "fast", search: func(context.Context) ([]contracts.MediaItem, error) {
		return []contracts.MediaItem{{ID: "movie", Title: "Movie"}}, nil
	}}
	slow := controlledSource{id: "slow", probe: func(ctx context.Context) {
		select {
		case <-release:
		case <-ctx.Done():
		}
	}, search: func(context.Context) ([]contracts.MediaItem, error) { return nil, nil }}
	var events []contracts.MediaSearchEvent
	ctx := contracts.WithMediaSearchEmitter(context.Background(), func(event contracts.MediaSearchEvent) {
		events = append(events, event)
		if event.Type == "source_results" && event.SourceID == "fast" {
			close(release)
		}
	})
	report := New(fast, slow).Search(ctx, "movie")
	if report.Status != "completed" || len(report.Sources[0].Items) != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if !report.Sources[1].Searched {
		t.Fatal("first hit prevented another source search")
	}
	fastIndex, slowIndex := -1, -1
	for index, event := range events {
		if event.RequestID != report.RequestID || event.Sequence != index+1 || event.Version != 1 {
			t.Fatalf("invalid event envelope: %+v", event)
		}
		if event.Type == "source_results" && event.SourceID == "fast" {
			fastIndex = index
		}
		if event.Type == "source_capability" && event.SourceID == "slow" {
			slowIndex = index
		}
	}
	if fastIndex < 0 || slowIndex <= fastIndex {
		t.Fatal("fast result waited for slow discovery")
	}
	if events[0].Snapshot.Status != "running" || len(events[0].Sources) != 2 {
		t.Fatal("initial source manifest or immutable snapshot missing")
	}
}

func TestPlaceholderRegistryDoesNotPretendToHaveSearched(t *testing.T) {
	t.Setenv("MEDIA_SEARCH_MODE", "placeholder")
	service := FromEnvironment(nil)
	report := service.Search(context.Background(), "movie")
	if len(report.Sources) != 12 {
		t.Fatalf("missing source placeholders: %d", len(report.Sources))
	}
	seen := map[string]bool{}
	for index, result := range report.Sources {
		if seen[result.SourceID] {
			t.Fatalf("duplicate source: %s", result.SourceID)
		}
		seen[result.SourceID] = true
		if result.Searched || result.Status != "skipped" || result.ErrorCode != "not_implemented" {
			t.Fatalf("placeholder reported live results: %+v", result)
		}
		if report.Capabilities[index].Available || report.Capabilities[index].Detected {
			t.Fatal("placeholder claimed source detection")
		}
		_, err := service.Browse(context.Background(), result.SourceID, "title")
		if !errors.Is(err, ErrNotImplemented) {
			t.Fatalf("unexpected browse error: %v", err)
		}
	}
}

func TestUnavailableAndFailedSourcesDoNotEraseResults(t *testing.T) {
	service := New(
		controlledSource{id: "offline", disabled: true},
		controlledSource{id: "broken", search: func(context.Context) ([]contracts.MediaItem, error) {
			return nil, context.DeadlineExceeded
		}},
		controlledSource{id: "ready", search: func(context.Context) ([]contracts.MediaItem, error) {
			return []contracts.MediaItem{{ID: "found"}}, nil
		}},
	)
	report := service.Search(context.Background(), "movie")
	if report.Sources[0].Searched || report.Sources[1].ErrorCode != "timeout" {
		t.Fatalf("unexpected source state: %+v", report.Sources)
	}
	if len(report.Sources[2].Items) != 1 {
		t.Fatal("failed source erased another source's results")
	}
}

func TestCancellationTerminatesStream(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	service := New(controlledSource{id: "slow",
		search: func(ctx context.Context) ([]contracts.MediaItem, error) {
			cancel()
			<-ctx.Done()
			return nil, ctx.Err()
		}})
	var terminal contracts.MediaSearchEvent
	ctx = contracts.WithMediaSearchEmitter(ctx, func(event contracts.MediaSearchEvent) {
		terminal = event
	})
	report := service.Search(ctx, "movie")
	if report.Status != "cancelled" || terminal.Type != "search_completed" {
		t.Fatalf("missing cancellation: %+v, %+v", report, terminal)
	}
}
