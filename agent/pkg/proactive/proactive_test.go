package proactive

import (
	"context"
	"testing"
	"time"

	"github.com/nchungdev/home-sentinel/agent/pkg/contracts"
)

type mockMediaProvider struct {
	items []*PlexRecentItem
	err   error
}

func (m *mockMediaProvider) GetRecentlyAdded(
	ctx context.Context,
	withinHours int,
) ([]*PlexRecentItem, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.items, nil
}

type mockEnvProvider struct {
	data *EnvironmentData
	err  error
}

func (m *mockEnvProvider) GetEnvironmentData(ctx context.Context) (*EnvironmentData, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.data, nil
}

func TestGenerateSuggestions_RecentMedia(t *testing.T) {
	media := &mockMediaProvider{
		items: []*PlexRecentItem{
			{
				ID:      "plex-1234",
				Title:   "Dune 2",
				AddedAt: 1729850000000,
			},
		},
	}
	engine := NewSuggestionEngine(media, nil)
	suggestions, err := engine.GenerateSuggestions(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(suggestions) == 0 {
		t.Fatal("expected at least 1 suggestion")
	}

	foundMedia := false
	for _, s := range suggestions {
		if s.Kind == "media_ready" {
			foundMedia = true
			if s.TargetID != "plex-1234" {
				t.Errorf("expected target plex-1234, got %s", s.TargetID)
			}
			if s.Action.ActionType != "plex.open" {
				t.Errorf("expected action plex.open, got %s", s.Action.ActionType)
			}
			if s.Action.RunsOn != "tv_client" {
				t.Errorf("expected runs_on tv_client, got %s", s.Action.RunsOn)
			}
			if s.Evidence["source"] != "radarr_complete" {
				t.Errorf("expected evidence radarr_complete, got %v", s.Evidence["source"])
			}
		}
	}
	if !foundMedia {
		t.Error("expected media_ready suggestion in results")
	}
}

func TestGenerateSuggestions_TimeSlots(t *testing.T) {
	engine := NewSuggestionEngine(nil, nil)

	testCases := []struct {
		name          string
		hour          int
		expectedTitle string
		expectedKind  string
	}{
		{"MorningCafe", 7, "Nhạc Cafe Sáng", "time_context"},
		{"NoonVlog", 12, "Vlog Ẩm Thực", "time_context"},
		{"EveningShow", 20, "Anh Trai Say Hi", "time_context"},
		{"NightLofi", 23, "Nhạc Ngủ Lofi", "time_context"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fakeTime := time.Date(2026, 10, 25, tc.hour, 30, 0, 0, time.Local)
			snapshot := &contracts.ContextSnapshot{
				TimestampEpochMs: fakeTime.UnixMilli(),
			}

			suggestions, err := engine.GenerateSuggestions(context.Background(), snapshot)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(suggestions) == 0 {
				t.Fatal("expected time suggestion")
			}

			found := false
			for _, s := range suggestions {
				if s.Title == tc.expectedTitle && s.Kind == tc.expectedKind {
					found = true
					if s.Action.ActionType != "youtube.launch" {
						t.Errorf("expected action youtube.launch, got %s", s.Action.ActionType)
					}
				}
			}
			if !found {
				t.Errorf("expected suggestion %s not found", tc.expectedTitle)
			}
		})
	}
}

func TestGenerateSuggestions_EnvironmentalAlert(t *testing.T) {
	temp := 31.5
	env := &mockEnvProvider{
		data: &EnvironmentData{
			Temperature: &temp,
		},
	}
	engine := NewSuggestionEngine(nil, env)
	suggestions, err := engine.GenerateSuggestions(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundEnv := false
	for _, s := range suggestions {
		if s.Kind == "environmental" {
			foundEnv = true
			if s.TargetID != "climate.living_room" {
				t.Errorf("expected target climate.living_room, got %s", s.TargetID)
			}
			if s.Action.ActionType != "ha.service_call" {
				t.Errorf("expected action ha.service_call, got %s", s.Action.ActionType)
			}
			if s.Action.RunsOn != "backend" {
				t.Errorf("expected runs_on backend, got %s", s.Action.RunsOn)
			}
		}
	}
	if !foundEnv {
		t.Error("expected environmental alert suggestion")
	}
}

func TestGenerateSuggestions_LimitAndDedupe(t *testing.T) {
	temp := 32.0
	env := &mockEnvProvider{data: &EnvironmentData{Temperature: &temp}}
	media := &mockMediaProvider{
		items: []*PlexRecentItem{
			{ID: "m1", Title: "Phim 1"},
			{ID: "m2", Title: "Phim 2"},
			{ID: "m3", Title: "Phim 3"},
			{ID: "m4", Title: "Phim 4"},
		},
	}
	engine := NewSuggestionEngine(media, env)
	suggestions, err := engine.GenerateSuggestions(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(suggestions) > 3 {
		t.Errorf("expected max 3 suggestions, got %d", len(suggestions))
	}

	seen := make(map[string]bool)
	for _, s := range suggestions {
		if seen[s.DedupeKey] {
			t.Errorf("duplicate suggestion key: %s", s.DedupeKey)
		}
		seen[s.DedupeKey] = true
	}
}

func TestGenerateSuggestions_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	engine := NewSuggestionEngine(nil, nil)
	_, err := engine.GenerateSuggestions(ctx, nil)
	if err == nil {
		t.Fatal("expected error on cancelled context")
	}
}
