package mediasearch

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/nchungdev/home-sentinel/internal/contracts"
)

type Browser interface {
	Browse(context.Context, string) ([]contracts.MediaItem, error)
}

type plexEntry struct {
	ID          string `json:"ratingKey"`
	Title       string `json:"title"`
	Type        string `json:"type"`
	Index       int    `json:"index"`
	ParentIndex int    `json:"parentIndex"`
	Summary     string `json:"summary"`
}

func (s *Service) Browse(ctx context.Context, sourceID, itemID string) ([]contracts.MediaItem, error) {
	for _, source := range s.sources {
		if source.Info().ID != sourceID {
			continue
		}
		if browser, ok := source.(Browser); ok {
			return browser.Browse(ctx, itemID)
		}
	}
	return nil, fmt.Errorf("nguồn không hỗ trợ danh sách mùa/tập")
}

func browseAction(sourceID, itemID, label string) contracts.MediaAction {
	return contracts.MediaAction{Type: "browse", Label: label, SourceID: sourceID, ItemID: itemID, Enabled: true}
}

func libraryActions(sourceID, itemID, kind, target string) []contracts.MediaAction {
	if kind == "show" {
		return []contracts.MediaAction{browseAction(sourceID, itemID, "Chọn mùa")}
	}
	if kind == "season" {
		return []contracts.MediaAction{browseAction(sourceID, itemID, "Chọn tập")}
	}
	return []contracts.MediaAction{{Type: "play_library", Label: "Phát trực tiếp",
		SourceID: sourceID, ItemID: itemID, Enabled: true}, linkAction("Mở thư viện", target)}
}

func (s *plexSource) Browse(ctx context.Context, itemID string) ([]contracts.MediaItem, error) {
	if id, err := strconv.Atoi(itemID); err != nil || id <= 0 {
		return nil, fmt.Errorf("invalid_item_id")
	}
	var response struct {
		MediaContainer struct {
			Metadata []plexEntry `json:"Metadata"`
		} `json:"MediaContainer"`
	}
	if err := s.do(ctx, http.MethodGet, "/library/metadata/"+itemID+"/children", nil, &response); err != nil {
		return nil, err
	}
	items := []contracts.MediaItem{}
	for _, entry := range response.MediaContainer.Metadata {
		if entry.Type != "season" && entry.Type != "episode" {
			continue
		}
		season := entry.Index
		var episode *int
		if entry.Type == "episode" {
			season = entry.ParentIndex
			number := entry.Index
			episode = &number
		}
		target := s.itemLink(ctx, entry.ID)
		items = append(items, contracts.MediaItem{ID: entry.ID, Title: entry.Title, Kind: entry.Type,
			Season: &season, Episode: episode, ParentID: itemID, Summary: entry.Summary, Availability: "in_library",
			Actions: libraryActions(s.id, entry.ID, entry.Type, target)})
	}
	return items, nil
}

type jellyfinEntry struct {
	ID          string `json:"Id"`
	Name        string `json:"Name"`
	Type        string `json:"Type"`
	SeriesID    string `json:"SeriesId"`
	Index       int    `json:"IndexNumber"`
	ParentIndex int    `json:"ParentIndexNumber"`
	Overview    string `json:"Overview"`
}

func (s *jellyfinSource) Browse(ctx context.Context, itemID string) ([]contracts.MediaItem, error) {
	if !safeJellyfinID(itemID) {
		return nil, fmt.Errorf("invalid_item_id")
	}
	var parent jellyfinEntry
	if err := s.do(ctx, http.MethodGet, "/Items/"+itemID, nil, &parent); err != nil {
		return nil, err
	}
	path := "/Shows/" + itemID + "/Seasons"
	if parent.Type == "Season" {
		if !safeJellyfinID(parent.SeriesID) {
			return nil, fmt.Errorf("invalid_series_id")
		}
		path = "/Shows/" + parent.SeriesID + "/Episodes?SeasonId=" + url.QueryEscape(itemID)
	} else if parent.Type != "Series" {
		return nil, fmt.Errorf("mục không phải phim bộ hoặc mùa")
	}
	var response struct {
		Items []jellyfinEntry `json:"Items"`
	}
	if err := s.do(ctx, http.MethodGet, path, nil, &response); err != nil {
		return nil, err
	}
	items := []contracts.MediaItem{}
	for _, entry := range response.Items {
		kind := strings.ToLower(entry.Type)
		season := entry.Index
		var episode *int
		if kind == "episode" {
			season = entry.ParentIndex
			number := entry.Index
			episode = &number
		}
		target := s.link("/web/index.html#!/details?id=" + url.QueryEscape(entry.ID))
		items = append(items, contracts.MediaItem{ID: entry.ID, Title: entry.Name, Kind: kind, Season: &season,
			Episode: episode, ParentID: itemID, Summary: entry.Overview, Availability: "in_library",
			Actions: libraryActions(s.id, entry.ID, kind, target)})
	}
	return items, nil
}

func safeJellyfinID(itemID string) bool {
	if len(itemID) != 32 {
		return false
	}
	for _, char := range itemID {
		if !(char >= '0' && char <= '9' || char >= 'a' && char <= 'f' || char >= 'A' && char <= 'F') {
			return false
		}
	}
	return true
}
