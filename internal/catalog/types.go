package catalog

import (
	"context"
	"errors"
)

var (
	ErrInvalidID   = errors.New("invalid_catalog_id")
	ErrUnavailable = errors.New("catalog_source_unavailable")
	ErrInvalidData = errors.New("invalid_catalog_response")
	ErrNotFound    = errors.New("catalog_item_not_found")
	ErrUnsupported = errors.New("episode_playback_unsupported")
)

type Title struct {
	ID       string    `json:"id"`
	SourceID string    `json:"source_id"`
	Name     string    `json:"name"`
	Kind     string    `json:"kind"`
	Year     int       `json:"year,omitempty"`
	Seasons  []Season  `json:"seasons"`
	Episodes []Episode `json:"episodes"`
}

type Season struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Order int    `json:"order"`
}

type Episode struct {
	ID       string `json:"id"`
	TitleID  string `json:"title_id"`
	Label    string `json:"label"`
	Group    string `json:"group"`
	Order    int    `json:"order"`
	SeasonID string `json:"season_id,omitempty"`
	Playable bool   `json:"playable"`
}

type Stream struct {
	Episode Episode
	Title   string
	URL     string
}

type Source interface {
	Identity() string
	GetTitle(context.Context, string) (Title, error)
	ResolveEpisode(context.Context, string) (Stream, error)
}
