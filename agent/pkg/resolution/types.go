package resolution

import "context"

type PlexMediaItem struct {
	Title     string `json:"title"`
	Year      int    `json:"year,omitempty"`
	RatingKey string `json:"rating_key"`
	Type      string `json:"type,omitempty"`
	Summary   string `json:"summary,omitempty"`
	Thumb     string `json:"thumb,omitempty"`
	DeepLink  string `json:"deep_link"`
}

type HAEntityMatch struct {
	EntityID    string         `json:"entity_id"`
	Domain      string         `json:"domain"`
	Service     string         `json:"service"`
	Name        string         `json:"name,omitempty"`
	ServiceData map[string]any `json:"service_data,omitempty"`
}

type PlexSearcher interface {
	SearchFirst(ctx context.Context, query string) (*PlexMediaItem, error)
	GetRecent(ctx context.Context, limit int) ([]*PlexMediaItem, error)
}

type HAEntityResolver interface {
	ResolveEntity(
		ctx context.Context,
		targetLabel string,
		action string,
	) (*HAEntityMatch, error)
}
