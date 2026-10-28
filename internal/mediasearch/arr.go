package mediasearch

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/nchungdev/home-sentinel/internal/contracts"
)

type arrSource struct {
	*httpSource
	profileID  int
	rootFolder string
}

func (s *arrSource) route() string {
	if s.id == "sonarr" {
		return "/api/v3/series"
	}
	return "/api/v3/movie"
}

func (s *arrSource) lookup(ctx context.Context, query string) ([]map[string]any, error) {
	var entries []map[string]any
	err := s.do(ctx, http.MethodGet, s.route()+"/lookup?term="+url.QueryEscape(query), nil, &entries)
	return entries, err
}

func (s *arrSource) Search(ctx context.Context, query string) ([]contracts.MediaItem, error) {
	entries, err := s.lookup(ctx, query)
	if err != nil {
		return nil, err
	}
	items := []contracts.MediaItem{}
	for _, entry := range entries {
		item, valid := s.item(entry)
		if !valid {
			continue
		}
		items = append(items, item)
		if len(items) == resultLimit {
			break
		}
	}
	return items, nil
}

func (s *arrSource) item(entry map[string]any) (contracts.MediaItem, bool) {
	kind, externalKey := "movie", "tmdbId"
	if s.id == "sonarr" {
		kind, externalKey = "show", "tvdbId"
	}
	number, _ := entry[externalKey].(float64)
	title, _ := entry["title"].(string)
	if number <= 0 || title == "" {
		return contracts.MediaItem{}, false
	}
	itemID := strconv.Itoa(int(number))
	year, _ := entry["year"].(float64)
	overview, _ := entry["overview"].(string)
	nativeID, _ := entry["id"].(float64)
	availability := "requestable"
	actions := []contracts.MediaAction{{Type: "request_library", Label: "Yêu cầu thêm thư viện",
		SourceID: s.id, ItemID: itemID, Enabled: true}}
	if nativeID > 0 {
		availability = "already_requested"
		actions = []contracts.MediaAction{}
	}
	if kind == "show" {
		actions = []contracts.MediaAction{browseAction(s.id, itemID, "Chọn mùa để yêu cầu")}
	}
	return contracts.MediaItem{ID: itemID, Title: title, Year: int(year), Summary: overview, Kind: kind,
		ExternalIDs: map[string]string{externalKey: itemID}, Availability: availability, Actions: actions}, true
}

func (s *arrSource) defaults(ctx context.Context) (int, string, error) {
	var profiles []struct {
		ID int `json:"id"`
	}
	var folders []struct {
		Path string `json:"path"`
	}
	if err := s.do(ctx, http.MethodGet, "/api/v3/qualityprofile", nil, &profiles); err != nil {
		return 0, "", err
	}
	if err := s.do(ctx, http.MethodGet, "/api/v3/rootfolder", nil, &folders); err != nil {
		return 0, "", err
	}
	profileID, folder := s.profileID, s.rootFolder
	if profileID == 0 && len(profiles) == 1 {
		profileID = profiles[0].ID
	}
	if folder == "" && len(folders) == 1 {
		folder = folders[0].Path
	}
	validProfile, validFolder := false, false
	for _, profile := range profiles {
		if profile.ID == profileID {
			validProfile = true
		}
	}
	for _, root := range folders {
		if root.Path == folder {
			validFolder = true
		}
	}
	if !validProfile || !validFolder {
		return 0, "", fmt.Errorf("cần cấu hình profile chất lượng và thư mục mặc định cho %s", s.name)
	}
	return profileID, folder, nil
}

func (s *arrSource) Request(ctx context.Context, itemID string, options RequestOptions) (contracts.MediaItem, error) {
	number, err := strconv.Atoi(itemID)
	if err != nil || number <= 0 {
		return contracts.MediaItem{}, fmt.Errorf("invalid_item_id")
	}
	prefix := "tmdb:"
	if s.id == "sonarr" {
		prefix = "tvdb:"
	}
	entries, err := s.lookup(ctx, prefix+itemID)
	if err != nil {
		return contracts.MediaItem{}, err
	}
	for _, entry := range entries {
		item, valid := s.item(entry)
		if !valid || item.ID != itemID {
			continue
		}
		if s.id == "sonarr" {
			if len(options.Seasons) == 0 {
				return contracts.MediaItem{}, fmt.Errorf("hãy chọn mùa")
			}
			if item.Availability == "already_requested" {
				if err := s.requestExistingSeason(ctx, entry, options); err != nil {
					return contracts.MediaItem{}, err
				}
				item.Availability, item.Actions = "requested", []contracts.MediaAction{}
				return item, nil
			}
			if err := selectSeasons(entry, options); err != nil {
				return contracts.MediaItem{}, err
			}
		} else if item.Availability == "already_requested" {
			return item, nil
		}
		profileID, folder, err := s.defaults(ctx)
		if err != nil {
			return contracts.MediaItem{}, err
		}
		entry["qualityProfileId"], entry["rootFolderPath"], entry["monitored"] = profileID, folder, true
		options := map[string]bool{"searchForMovie": true}
		if s.id == "sonarr" {
			options = map[string]bool{"searchForMissingEpisodes": true}
		}
		entry["addOptions"] = options
		var added map[string]any
		if err := s.do(ctx, http.MethodPost, s.route(), entry, &added); err != nil {
			return contracts.MediaItem{}, err
		}
		item.Availability, item.Actions = "requested", []contracts.MediaAction{}
		return item, nil
	}
	return contracts.MediaItem{}, fmt.Errorf("không tìm thấy đúng mục trong catalog")
}

func (s *arrSource) Capability(ctx context.Context) contracts.MediaSourceCapability {
	capability := s.httpSource.Capability(ctx)
	if capability.Available {
		capability.Actions = []string{"request_library"}
	}
	return capability
}
