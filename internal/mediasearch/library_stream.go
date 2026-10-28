package mediasearch

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

func (s *Service) LibraryStream(
	ctx context.Context, sourceID, itemID string,
) (string, http.Header, error) {
	for _, source := range s.sources {
		switch library := source.(type) {
		case *plexSource:
			if sourceID == library.id {
				return library.stream(ctx, itemID)
			}
		case *jellyfinSource:
			if sourceID != library.id {
				continue
			}
			if !safeJellyfinID(itemID) {
				return "", nil, fmt.Errorf("invalid_item_id")
			}
			endpoint := library.baseURL + "/Videos/" + url.PathEscape(itemID) + "/stream?static=true"
			return endpoint, http.Header{"X-Emby-Token": {library.apiKey}}, nil
		}
	}
	return "", nil, fmt.Errorf("nguồn không hỗ trợ phát trực tiếp")
}

func (s *plexSource) stream(ctx context.Context, itemID string) (string, http.Header, error) {
	if id, err := strconv.Atoi(itemID); err != nil || id <= 0 {
		return "", nil, fmt.Errorf("invalid_item_id")
	}
	var response struct {
		MediaContainer struct {
			Metadata []struct {
				Media []struct {
					Part []struct {
						Key string `json:"key"`
					} `json:"Part"`
				} `json:"Media"`
			} `json:"Metadata"`
		} `json:"MediaContainer"`
	}
	if err := s.do(ctx, http.MethodGet, "/library/metadata/"+itemID, nil, &response); err != nil {
		return "", nil, err
	}
	for _, entry := range response.MediaContainer.Metadata {
		for _, media := range entry.Media {
			for _, part := range media.Part {
				parsed, err := url.Parse(part.Key)
				if err != nil || parsed.IsAbs() || parsed.Host != "" || parsed.Path == "" {
					continue
				}
				return s.baseURL + parsed.Path, http.Header{"X-Plex-Token": {s.apiKey}}, nil
			}
		}
	}
	return "", nil, fmt.Errorf("không có tệp media khả dụng")
}
