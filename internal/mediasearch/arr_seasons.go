package mediasearch

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/nchungdev/home-sentinel/internal/contracts"
)

type RequestOptions struct {
	Seasons []int `json:"seasons,omitempty"`
}

func (s *arrSource) Browse(ctx context.Context, itemID string) ([]contracts.MediaItem, error) {
	if s.id != "sonarr" {
		return nil, fmt.Errorf("nguồn không có mùa/tập")
	}
	number, err := strconv.Atoi(itemID)
	if err != nil || number <= 0 {
		return nil, fmt.Errorf("invalid_item_id")
	}
	entries, err := s.lookup(ctx, "tvdb:"+itemID)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		item, valid := s.item(entry)
		if !valid || item.ID != itemID {
			continue
		}
		seasons, _ := entry["seasons"].([]any)
		items := []contracts.MediaItem{}
		for _, raw := range seasons {
			season, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			value, ok := season["seasonNumber"].(float64)
			if !ok {
				continue
			}
			seasonNumber := int(value)
			seasonLabel := fmt.Sprintf("Mùa %d", seasonNumber)
			actionLabel := fmt.Sprintf("➕ Yêu cầu Mùa %d", seasonNumber)
			if seasonNumber == 0 {
				seasonLabel = "Tập đặc biệt (Specials)"
				actionLabel = "➕ Yêu cầu Tập đặc biệt"
			}
			items = append(items, contracts.MediaItem{
				ID: itemID + ":" + strconv.Itoa(seasonNumber), ParentID: itemID, Kind: "season",
				Title: fmt.Sprintf("%s · %s", item.Title, seasonLabel), Season: &seasonNumber,
				Availability: "requestable", Actions: []contracts.MediaAction{{Type: "request_library",
					Label: actionLabel, SourceID: s.id, ItemID: itemID, Season: &seasonNumber, Enabled: true}},
			})
		}
		return items, nil
	}
	return nil, fmt.Errorf("không tìm thấy phim bộ")
}

func selectSeasons(entry map[string]any, options RequestOptions) error {
	if len(options.Seasons) == 0 {
		return fmt.Errorf("hãy chọn mùa trước khi yêu cầu phim bộ")
	}
	selected := map[int]bool{}
	for _, season := range options.Seasons {
		if season < 0 {
			return fmt.Errorf("invalid_season")
		}
		selected[season] = true
	}
	seasons, _ := entry["seasons"].([]any)
	found := map[int]bool{}
	for _, raw := range seasons {
		season, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		number, ok := season["seasonNumber"].(float64)
		if !ok {
			continue
		}
		wanted := selected[int(number)]
		season["monitored"] = wanted
		if wanted {
			found[int(number)] = true
		}
	}
	if len(found) != len(selected) {
		return fmt.Errorf("mùa được chọn không có trong catalog")
	}
	entry["seasons"] = seasons
	return nil
}

func (s *arrSource) requestExistingSeason(
	ctx context.Context, entry map[string]any, options RequestOptions,
) error {
	nativeID, _ := entry["id"].(float64)
	var series map[string]any
	path := s.route() + "/" + strconv.Itoa(int(nativeID))
	if err := s.do(ctx, http.MethodGet, path, nil, &series); err != nil {
		return err
	}
	selected := map[int]bool{}
	for _, season := range options.Seasons {
		selected[season] = true
	}
	seasons, _ := series["seasons"].([]any)
	found := map[int]bool{}
	for _, raw := range seasons {
		if season, ok := raw.(map[string]any); ok {
			number, _ := season["seasonNumber"].(float64)
			if selected[int(number)] {
				season["monitored"] = true
				found[int(number)] = true
			}
		}
	}
	if len(found) != len(selected) {
		return fmt.Errorf("mùa được chọn không có trong series")
	}
	series["monitored"] = true
	if err := s.do(ctx, http.MethodPut, path, series, nil); err != nil {
		return err
	}
	for season := range selected {
		command := map[string]any{"name": "SeasonSearch", "seriesId": int(nativeID), "seasonNumber": season}
		if err := s.do(ctx, http.MethodPost, "/api/v3/command", command, nil); err != nil {
			return err
		}
	}
	return nil
}
