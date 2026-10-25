package proactive

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nchungdev/home-sentinel/agent/pkg/contracts"
)

type PlexRecentItem struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	AddedAt  int64  `json:"added_at"`
	ThumbURL string `json:"thumb_url,omitempty"`
	DeepLink string `json:"deep_link,omitempty"`
}

type EnvironmentData struct {
	Temperature  *float64 `json:"temperature,omitempty"`
	LivingRoomAC string   `json:"living_room_ac,omitempty"`
	FrontDoorOpen bool    `json:"front_door_open"`
}

type RecentMediaProvider interface {
	GetRecentlyAdded(ctx context.Context, withinHours int) ([]*PlexRecentItem, error)
}

type EnvironmentProvider interface {
	GetEnvironmentData(ctx context.Context) (*EnvironmentData, error)
}

type SuggestionEngine struct {
	mediaProvider RecentMediaProvider
	envProvider   EnvironmentProvider
}

func NewSuggestionEngine(media RecentMediaProvider, env EnvironmentProvider) *SuggestionEngine {
	return &SuggestionEngine{
		mediaProvider: media,
		envProvider:   env,
	}
}

func (e *SuggestionEngine) GenerateSuggestions(
	ctx context.Context,
	snapshot *contracts.ContextSnapshot,
) ([]*contracts.Suggestion, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var nowEpochMs int64
	if snapshot != nil && snapshot.TimestampEpochMs > 0 {
		nowEpochMs = snapshot.TimestampEpochMs
	} else {
		nowEpochMs = time.Now().UnixMilli()
	}

	localTime := time.UnixMilli(nowEpochMs)
	currentHour := localTime.Hour()

	var candidates []*contracts.Suggestion

	if e.mediaProvider != nil {
		recentItems, err := e.mediaProvider.GetRecentlyAdded(ctx, 12)
		if err == nil {
			for _, item := range recentItems {
				if item == nil || strings.TrimSpace(item.ID) == "" {
					continue
				}
				deepLink := item.DeepLink
				if deepLink == "" {
					deepLink = fmt.Sprintf("plex://server/%s", item.ID)
				}
				suggestionID := fmt.Sprintf("sug-media-%s", item.ID)
				candidates = append(candidates, &contracts.Suggestion{
					ID:       suggestionID,
					Kind:     "media_ready",
					Title:    fmt.Sprintf("Đã có %s", item.Title),
					Message:  fmt.Sprintf("Phim %s vừa tải xong trên NAS, xem ngay không?", item.Title),
					TargetID: item.ID,
					Action: contracts.ActionStep{
						StepID:       fmt.Sprintf("step-%s", suggestionID),
						ActionType:   "plex.open",
						TargetID:     item.ID,
						Parameters:   map[string]any{"deep_link": deepLink},
						RunsOn:       "tv_client",
						TimeoutMs:    5000,
						IsIdempotent: true,
					},
					Evidence: map[string]any{
						"source":   "radarr_complete",
						"added_at": item.AddedAt,
					},
					GeneratedAt: nowEpochMs,
					ExpiresAt:   nowEpochMs + int64(6*time.Hour/time.Millisecond),
					DedupeKey:   fmt.Sprintf("recent_plex_%s", item.ID),
				})
			}
		}
	}

	if e.envProvider != nil {
		envData, err := e.envProvider.GetEnvironmentData(ctx)
		if err == nil && envData != nil {
			if envData.Temperature != nil && *envData.Temperature >= 30.0 {
				tempValue := *envData.Temperature
				candidates = append(candidates, &contracts.Suggestion{
					ID:       "sug-env-high-temp",
					Kind:     "environmental",
					Title:    "Phòng khách hơi nóng",
					Message:  fmt.Sprintf("Nhiệt độ phòng khách là %.1f°C, bạn có muốn bật điều hòa 25°C không?", tempValue),
					TargetID: "climate.living_room",
					Action: contracts.ActionStep{
						StepID:       "step-env-climate-on",
						ActionType:   "ha.service_call",
						TargetID:     "climate.living_room",
						Parameters:   map[string]any{"service": "set_temperature", "temperature": 25.0},
						RunsOn:       "backend",
						TimeoutMs:    5000,
						IsIdempotent: true,
					},
					Evidence: map[string]any{
						"temperature": tempValue,
						"source":      "ha_sensor",
					},
					GeneratedAt: nowEpochMs,
					ExpiresAt:   nowEpochMs + int64(1*time.Hour/time.Millisecond),
					DedupeKey:   "env_high_temp",
				})
			}
		}
	}

	timeSuggestion := timeContextSuggestion(currentHour, nowEpochMs)
	if timeSuggestion != nil {
		candidates = append(candidates, timeSuggestion)
	}

	var results []*contracts.Suggestion
	seenKeys := make(map[string]bool)

	for _, candidate := range candidates {
		if candidate == nil {
			continue
		}
		if candidate.DedupeKey != "" && seenKeys[candidate.DedupeKey] {
			continue
		}
		if candidate.DedupeKey != "" {
			seenKeys[candidate.DedupeKey] = true
		}
		results = append(results, candidate)
		if len(results) >= 3 {
			break
		}
	}

	return results, nil
}

func timeContextSuggestion(hour int, nowEpochMs int64) *contracts.Suggestion {
	switch {
	case hour >= 6 && hour <= 8:
		return &contracts.Suggestion{
			ID:       "sug-time-morning-cafe",
			Kind:     "time_context",
			Title:    "Nhạc Cafe Sáng",
			Message:  "Chào buổi sáng! Mở playlist nhạc cafe nhẹ nhàng nhé?",
			TargetID: "yt_morning_cafe",
			Action: contracts.ActionStep{
				StepID:       "step-morning-cafe",
				ActionType:   "youtube.launch",
				TargetID:     "yt_morning_cafe",
				Parameters:   map[string]any{"query": "nhạc cafe sáng acoustic"},
				RunsOn:       "tv_client",
				TimeoutMs:    5000,
				IsIdempotent: true,
			},
			Evidence:    map[string]any{"source": "time_of_day", "hour": hour},
			GeneratedAt: nowEpochMs,
			ExpiresAt:   nowEpochMs + int64(2*time.Hour/time.Millisecond),
			DedupeKey:   "time_morning_cafe",
		}
	case hour >= 11 && hour <= 13:
		return &contracts.Suggestion{
			ID:       "sug-time-noon-vlog",
			Kind:     "time_context",
			Title:    "Vlog Ẩm Thực",
			Message:  "Bữa trưa ngon miệng! Xem vlog ẩm thực giải trí nhé?",
			TargetID: "yt_noon_vlog",
			Action: contracts.ActionStep{
				StepID:       "step-noon-vlog",
				ActionType:   "youtube.launch",
				TargetID:     "yt_noon_vlog",
				Parameters:   map[string]any{"query": "vlog ẩm thực việt nam"},
				RunsOn:       "tv_client",
				TimeoutMs:    5000,
				IsIdempotent: true,
			},
			Evidence:    map[string]any{"source": "time_of_day", "hour": hour},
			GeneratedAt: nowEpochMs,
			ExpiresAt:   nowEpochMs + int64(2*time.Hour/time.Millisecond),
			DedupeKey:   "time_noon_vlog",
		}
	case hour >= 19 && hour <= 21:
		return &contracts.Suggestion{
			ID:       "sug-time-evening-show",
			Kind:     "time_context",
			Title:    "Anh Trai Say Hi",
			Message:  "Tối nay cùng xem tập mới nhất của Anh Trai Say Hi nhé?",
			TargetID: "yt_say_hi",
			Action: contracts.ActionStep{
				StepID:       "step-evening-show",
				ActionType:   "youtube.launch",
				TargetID:     "yt_say_hi",
				Parameters:   map[string]any{"query": "Anh Trai Say Hi tập mới nhất"},
				RunsOn:       "tv_client",
				TimeoutMs:    5000,
				IsIdempotent: true,
			},
			Evidence:    map[string]any{"source": "time_of_day", "hour": hour},
			GeneratedAt: nowEpochMs,
			ExpiresAt:   nowEpochMs + int64(2*time.Hour/time.Millisecond),
			DedupeKey:   "time_evening_show",
		}
	case hour >= 22 || hour <= 4:
		return &contracts.Suggestion{
			ID:       "sug-time-night-lofi",
			Kind:     "time_context",
			Title:    "Nhạc Ngủ Lofi",
			Message:  "Đã khuya rồi, mở nhạc lofi êm dịu và nghỉ ngơi sớm nhé?",
			TargetID: "yt_night_lofi",
			Action: contracts.ActionStep{
				StepID:       "step-night-lofi",
				ActionType:   "youtube.launch",
				TargetID:     "yt_night_lofi",
				Parameters:   map[string]any{"query": "nhạc lofi thư giãn dễ ngủ"},
				RunsOn:       "tv_client",
				TimeoutMs:    5000,
				IsIdempotent: true,
			},
			Evidence:    map[string]any{"source": "time_of_day", "hour": hour},
			GeneratedAt: nowEpochMs,
			ExpiresAt:   nowEpochMs + int64(2*time.Hour/time.Millisecond),
			DedupeKey:   "time_night_lofi",
		}
	default:
		return nil
	}
}
