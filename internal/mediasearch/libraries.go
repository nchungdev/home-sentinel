package mediasearch

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"github.com/nchungdev/home-sentinel/internal/contracts"
)

type plexSource struct {
	*httpSource
	identity plexIdentity
}

func (s *plexSource) Search(ctx context.Context, query string) ([]contracts.MediaItem, error) {
	var response struct {
		MediaContainer struct {
			Hub []struct {
				Metadata []struct {
					Title   string `json:"title"`
					Year    int    `json:"year"`
					ID      string `json:"ratingKey"`
					Type    string `json:"type"`
					Summary string `json:"summary"`
					Thumb   string `json:"thumb"`
					Origin  string `json:"originalTitle"`
				} `json:"Metadata"`
			} `json:"Hub"`
		} `json:"MediaContainer"`
	}
	path := "/hubs/search?query=" + url.QueryEscape(query) + "&limit=" + strconv.Itoa(resultLimit)
	if err := s.do(ctx, http.MethodGet, path, nil, &response); err != nil {
		return nil, err
	}
	items := []contracts.MediaItem{}
	seen := map[string]bool{}
	for _, hub := range response.MediaContainer.Hub {
		for _, entry := range hub.Metadata {
			if (entry.Type != "movie" && entry.Type != "show") || entry.ID == "" || seen[entry.ID] {
				continue
			}
			seen[entry.ID] = true
			target := s.itemLink(ctx, entry.ID)
			actions := libraryActions(s.id, entry.ID, entry.Type, target)
			if app := s.appLink(ctx, entry.ID); app != "" {
				actions = append([]contracts.MediaAction{appAction(app)}, actions...)
			}
			items = append(items, contracts.MediaItem{
				ID: entry.ID, Title: entry.Title, Year: entry.Year, Kind: entry.Type,
				Summary: entry.Summary, URL: target, Poster: s.poster(entry.Thumb),
				Original:     entry.Origin,
				Availability: "in_library", Actions: actions,
			})
			if len(items) == resultLimit {
				return items, nil
			}
		}
	}
	return items, nil
}

type jellyfinSource struct{ *httpSource }

func (s *jellyfinSource) Search(ctx context.Context, query string) ([]contracts.MediaItem, error) {
	var response struct {
		Items []struct {
			ID       string `json:"Id"`
			Name     string `json:"Name"`
			Year     int    `json:"ProductionYear"`
			Type     string `json:"Type"`
			Overview string `json:"Overview"`
		} `json:"Items"`
	}
	values := url.Values{"SearchTerm": {query}, "Recursive": {"true"},
		"IncludeItemTypes": {"Movie,Series"}, "Fields": {"Overview"}, "Limit": {"8"}}
	if err := s.do(ctx, http.MethodGet, "/Items?"+values.Encode(), nil, &response); err != nil {
		return nil, err
	}
	items := []contracts.MediaItem{}
	for _, entry := range response.Items {
		target := s.link("/web/index.html#!/details?id=" + url.QueryEscape(entry.ID))
		kind := "movie"
		if entry.Type == "Series" {
			kind = "show"
		}
		actions := libraryActions(s.id, entry.ID, kind, target)
		items = append(items, contracts.MediaItem{ID: entry.ID, Title: entry.Name, Year: entry.Year,
			Kind: kind, Summary: entry.Overview, URL: target, Availability: "in_library", Actions: actions})
	}
	return items, nil
}
