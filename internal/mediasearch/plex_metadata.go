package mediasearch

import (
	"context"
	"net/http"
	"net/url"
)

type plexSearchEntry struct {
	Title   string            `json:"title"`
	Year    int               `json:"year"`
	ID      string            `json:"ratingKey"`
	Type    string            `json:"type"`
	Summary string            `json:"summary"`
	Thumb   string            `json:"thumb"`
	Origin  string            `json:"originalTitle"`
	GUID    string            `json:"guid"`
	GUIDs   []externalIDEntry `json:"Guid"`
	Media   []struct {
		Part []struct {
			File string `json:"file"`
		} `json:"Part"`
	} `json:"Media"`
}

func (entry plexSearchEntry) externalIDs() map[string]string {
	externalIDs := plexExternalIDs(entry.GUIDs)
	for _, media := range entry.Media {
		for _, part := range media.Part {
			for provider, value := range parseFileExternalIDs(part.File) {
				if externalIDs[provider] == "" {
					externalIDs[provider] = value
				}
			}
		}
	}

	return externalIDs
}

func (s *plexSource) searchEntryExternalIDs(
	ctx context.Context, entry plexSearchEntry,
) map[string]string {
	externalIDs := entry.externalIDs()
	if len(externalIDs) > 0 {
		return externalIDs
	}

	var response struct {
		MediaContainer struct {
			Metadata []plexSearchEntry `json:"Metadata"`
		} `json:"MediaContainer"`
	}
	path := "/library/metadata/" + url.PathEscape(entry.ID) + "?includeGuids=1"
	if err := s.do(ctx, http.MethodGet, path, nil, &response); err != nil {
		return externalIDs
	}
	for _, detail := range response.MediaContainer.Metadata {
		if detail.ID == entry.ID && detail.Type == entry.Type {
			return detail.externalIDs()
		}
	}

	return externalIDs
}
