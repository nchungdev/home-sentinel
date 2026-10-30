package contracts

type MediaAction struct {
	Type     string `json:"type"`
	Label    string `json:"label"`
	URL      string `json:"url,omitempty"`
	SourceID string `json:"source_id,omitempty"`
	ItemID   string `json:"item_id,omitempty"`
	Enabled  bool   `json:"enabled"`
	Reason   string `json:"reason,omitempty"`
	Season   *int   `json:"season,omitempty"`
}

type CatalogEpisode struct {
	ID       string `json:"id"`
	TitleID  string `json:"title_id,omitempty"`
	Label    string `json:"label"`
	Group    string `json:"group,omitempty"`
	Order    int    `json:"order,omitempty"`
	SeasonID string `json:"season_id,omitempty"`
	Playable bool   `json:"playable"`
	URL      string `json:"url,omitempty"`
}

type CatalogServer struct {
	Name     string           `json:"server_name"`
	Episodes []CatalogEpisode `json:"episodes"`
}

type MediaItem struct {
	ID                string            `json:"id"`
	Title             string            `json:"title"`
	Kind              string            `json:"kind"`
	Year              int               `json:"year,omitempty"`
	Summary           string            `json:"summary,omitempty"`
	URL               string            `json:"url,omitempty"`
	Poster            string            `json:"poster,omitempty"`
	Original          string            `json:"original_title,omitempty"`
	AlternativeTitles []string          `json:"alternative_titles,omitempty"`
	MatchMethod       string            `json:"match_method,omitempty"`
	ParentID          string            `json:"parent_id,omitempty"`
	Season            *int              `json:"season,omitempty"`
	Episode           *int              `json:"episode,omitempty"`
	ExternalIDs       map[string]string `json:"external_ids,omitempty"`
	Availability      string            `json:"availability"`
	Servers           []CatalogServer   `json:"servers,omitempty"`
	Episodes          []CatalogEpisode  `json:"episodes,omitempty"`
	Playback          any               `json:"playback,omitempty"`
	Actions           []MediaAction     `json:"actions,omitempty"`
}

type MediaSourceCapability struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	Detected  bool     `json:"detected"`
	Available bool     `json:"available"`
	Status    string   `json:"status"`
	Message   string   `json:"message,omitempty"`
	Actions   []string `json:"actions"`
}

type MediaSourceResults struct {
	SourceID        string      `json:"source_id"`
	Status          string      `json:"status"`
	Searched        bool        `json:"searched"`
	Cache           string      `json:"cache,omitempty"`
	Items           []MediaItem `json:"items"`
	ErrorCode       string      `json:"error_code,omitempty"`
	DiscoveryMethod string      `json:"discovery_method,omitempty"`
}

type MediaContentSection struct {
	SourceID  string      `json:"source_id"`
	Status    string      `json:"status"`
	Searched  bool        `json:"searched"`
	Cache     string      `json:"cache,omitempty"`
	Items     []MediaItem `json:"items"`
	ErrorCode string      `json:"error_code,omitempty"`
}

type MediaContentType struct {
	Type     string                `json:"type"`
	Sections []MediaContentSection `json:"sections"`
}

type MediaSearchResponse struct {
	RequestID    string                  `json:"request_id"`
	Status       string                  `json:"status"`
	Version      int                     `json:"version"`
	Query        string                  `json:"query"`
	Capabilities []MediaSourceCapability `json:"capabilities"`
	Sources      []MediaSourceResults    `json:"sources"`
	ContentTypes []MediaContentType      `json:"content_types"`
}
