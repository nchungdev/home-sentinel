package proactive

import (
	"testing"
	"time"
)

func TestMediaEventStoreExpiry(t *testing.T) {
	store := NewMediaEventStore(1 * time.Hour)

	now := time.Now().UnixMilli()
	eventActive := &MediaEvent{
		EventID:     "ev-active",
		EventType:   "Download",
		Title:       "Dune 2",
		TimestampMs: now,
		ExpiresAtMs: now + 3600*1000,
	}
	eventExpired := &MediaEvent{
		EventID:     "ev-old",
		EventType:   "Download",
		Title:       "Matrix",
		TimestampMs: now - 7200*1000,
		ExpiresAtMs: now - 3600*1000,
	}

	store.Add(eventActive)
	store.Add(eventExpired)

	if store.Count() != 2 {
		t.Fatalf("Expected 2 events before purge, got %d", store.Count())
	}

	active := store.GetActive(now)
	if len(active) != 1 {
		t.Fatalf("Expected 1 active event, got %d", len(active))
	}
	if active[0].EventID != "ev-active" {
		t.Errorf("Expected ev-active, got %s", active[0].EventID)
	}
	if store.Count() != 1 {
		t.Errorf("Expected expired event to be deleted, store count %d", store.Count())
	}
}
