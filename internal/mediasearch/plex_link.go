package mediasearch

import (
	"context"
	"net/http"
	"net/url"
	"sync"

	"github.com/nchungdev/home-sentinel/internal/contracts"
)

type plexIdentity struct {
	mutex     sync.Mutex
	machineID string
}

func (s *plexSource) machineIdentifier(ctx context.Context) string {
	s.identity.mutex.Lock()
	defer s.identity.mutex.Unlock()
	if s.identity.machineID != "" {
		return s.identity.machineID
	}

	var response struct {
		MediaContainer struct {
			MachineID string `json:"machineIdentifier"`
		} `json:"MediaContainer"`
	}
	if err := s.do(ctx, http.MethodGet, "/identity", nil, &response); err != nil {
		return ""
	}
	s.identity.machineID = response.MediaContainer.MachineID

	return s.identity.machineID
}

func (s *plexSource) appLink(ctx context.Context, ratingKey string) string {
	machineID := s.machineIdentifier(ctx)
	if machineID == "" || ratingKey == "" {
		return ""
	}
	return "plex://server://" + machineID +
		"/com.plexapp.plugins.library/library/metadata/" + url.PathEscape(ratingKey)
}

func appAction(link string) contracts.MediaAction {
	return contracts.MediaAction{Type: "open_app", Label: "Mở trong Plex", URL: link, Enabled: true}
}

func (s *plexSource) itemLink(ctx context.Context, ratingKey string) string {
	key := url.QueryEscape("/library/metadata/" + ratingKey)
	machineID := s.machineIdentifier(ctx)
	if machineID == "" {
		return s.link("/web/index.html#!/details?key=" + key)
	}

	return s.link("/web/index.html#!/server/" + url.PathEscape(machineID) + "/details?key=" + key)
}
