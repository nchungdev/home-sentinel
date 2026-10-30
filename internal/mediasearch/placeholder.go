package mediasearch

import (
	"context"
	"errors"

	"github.com/nchungdev/home-sentinel/internal/contracts"
)

var ErrNotImplemented = errors.New("not_implemented")

type placeholderSource struct {
	id   string
	name string
	kind string
}

func PlaceholderSources() []Source {
	return append([]Source{
		placeholderSource{"plex", "Plex", "library"},
		placeholderSource{"jellyfin", "Jellyfin", "library"},
		placeholderSource{"radarr", "Radarr", "catalog"},
		placeholderSource{"sonarr", "Sonarr", "catalog"},
		placeholderSource{"youtube", "YouTube", "video"},
		placeholderSource{"web", "Internet", "web"},
	}, pendingCatalogSources()...)
}

func pendingCatalogSources() []Source {
	return []Source{
		placeholderSource{"phimapi", "PhimAPI", "catalog"},
		placeholderSource{"ophim", "OPhim", "catalog"},
		placeholderSource{"nguonc", "NguonC", "catalog"},
		placeholderSource{"motchill", "Motchill", "website"},
		placeholderSource{"rophim", "RoPhim", "website"},
		placeholderSource{"phimtuoitho", "Phim Tuổi Thơ", "website"},
	}
}

func (s placeholderSource) Info() contracts.MediaSourceCapability {
	return contracts.MediaSourceCapability{
		ID: s.id, Name: s.name, Kind: s.kind, Status: "not_implemented",
		PlaybackMode: 0,
		Message:      "Adapter chưa triển khai; chưa kiểm tra kết nối nguồn.",
		Actions:      []string{},
	}
}

func (s placeholderSource) Capability(context.Context) contracts.MediaSourceCapability {
	return s.Info()
}

func (s placeholderSource) Search(context.Context, string) ([]contracts.MediaItem, error) {
	return nil, ErrNotImplemented
}

func (s placeholderSource) Browse(context.Context, string) ([]contracts.MediaItem, error) {
	return nil, ErrNotImplemented
}

func (s placeholderSource) Request(
	context.Context, string, RequestOptions,
) (contracts.MediaItem, error) {
	return contracts.MediaItem{}, ErrNotImplemented
}
