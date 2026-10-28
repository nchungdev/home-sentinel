package proactive

import (
	"sync"
	"time"
)

type MediaEvent struct {
	EventID        string `json:"event_id"`
	EventType      string `json:"event_type"`
	Title          string `json:"title"`
	Year           int    `json:"year,omitempty"`
	TmdbID         int    `json:"tmdb_id,omitempty"`
	PlexRatingKey  string `json:"plex_rating_key,omitempty"`
	DeepLink       string `json:"deep_link,omitempty"`
	TimestampMs    int64  `json:"timestamp_ms"`
	ExpiresAtMs    int64  `json:"expires_at_ms"`
}

type MediaEventStore struct {
	mu     sync.RWMutex
	events map[string]*MediaEvent
	ttl    time.Duration
}

func NewMediaEventStore(ttl time.Duration) *MediaEventStore {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &MediaEventStore{
		events: make(map[string]*MediaEvent),
		ttl:    ttl,
	}
}

func (s *MediaEventStore) Add(event *MediaEvent) {
	if event == nil || event.EventID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixMilli()
	if event.TimestampMs <= 0 {
		event.TimestampMs = now
	}
	if event.ExpiresAtMs <= 0 {
		event.ExpiresAtMs = event.TimestampMs + s.ttl.Milliseconds()
	}
	s.events[event.EventID] = event
}

func (s *MediaEventStore) GetActive(nowMs int64) []*MediaEvent {
	s.mu.Lock()
	defer s.mu.Unlock()

	if nowMs <= 0 {
		nowMs = time.Now().UnixMilli()
	}

	var active []*MediaEvent
	for id, event := range s.events {
		if event.ExpiresAtMs <= nowMs {
			delete(s.events, id)
			continue
		}
		active = append(active, event)
	}
	return active
}

func (s *MediaEventStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.events)
}
