package watching

import (
	"encoding/json"
	"errors"
	"os"
	"sort"
	"sync"
	"time"
)

const maxHistoryEntries = 100

var ErrNoValidEvents = errors.New("no valid watch events")

type History struct {
	path    string
	mu      sync.Mutex
	entries []Event
}

func NewHistory(path string) *History {
	history := &History{path: path}
	history.load()
	return history
}

func (h *History) Record(events []Event) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	accepted := 0
	for _, event := range events {
		if !event.valid() {
			continue
		}
		h.replace(event)
		accepted++
	}
	if accepted == 0 {
		return ErrNoValidEvents
	}

	sort.SliceStable(h.entries, func(i, j int) bool { return h.entries[i].At > h.entries[j].At })
	if len(h.entries) > maxHistoryEntries {
		h.entries = h.entries[:maxHistoryEntries]
	}
	return h.save()
}

func (h *History) Since(cutoff time.Time) []Event {
	h.mu.Lock()
	defer h.mu.Unlock()

	limit := cutoff.UnixMilli()
	var recent []Event
	for _, event := range h.entries {
		if event.At >= limit {
			recent = append(recent, event)
		}
	}
	return recent
}

func (h *History) replace(event Event) {
	for index, existing := range h.entries {
		if existing.key() != event.key() {
			continue
		}
		if event.At >= existing.At {
			h.entries[index] = keepPosition(event, existing)
		}
		return
	}
	h.entries = append(h.entries, event)
}

func keepPosition(event, earlier Event) Event {
	if event.PositionMs > 0 || earlier.PositionMs <= 0 {
		return event
	}
	event.PositionMs = earlier.PositionMs
	event.DurationMs = max(event.DurationMs, earlier.DurationMs)
	return event
}

func (h *History) load() {
	if h.path == "" {
		return
	}
	data, err := os.ReadFile(h.path)
	if err != nil {
		return
	}
	_ = json.Unmarshal(data, &h.entries)
}

func (h *History) save() error {
	if h.path == "" {
		return nil
	}
	data, err := json.Marshal(h.entries)
	if err != nil {
		return err
	}
	temporary := h.path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return err
	}
	return os.Rename(temporary, h.path)
}
