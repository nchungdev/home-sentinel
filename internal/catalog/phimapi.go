package catalog

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type PhimAPI struct {
	BaseURL string
	Client  *http.Client
}

type apiEpisode struct {
	Name   string `json:"name"`
	Slug   string `json:"slug"`
	Stream string `json:"link_m3u8"`
}

type apiGroup struct {
	Name     string       `json:"server_name"`
	Episodes []apiEpisode `json:"server_data"`
}

type apiDetail struct {
	Status bool `json:"status"`
	Movie  struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
		Type string `json:"type"`
		Year int    `json:"year"`
	} `json:"movie"`
	Groups []apiGroup `json:"episodes"`
}

func NewPhimAPI(baseURL string) *PhimAPI {
	return &PhimAPI{BaseURL: strings.TrimRight(baseURL, "/"),
		Client: &http.Client{Timeout: 7 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			}}}
}

func (p *PhimAPI) Identity() string { return p.BaseURL }

func (p *PhimAPI) read(ctx context.Context, slug string) (apiDetail, error) {
	var detail apiDetail
	request, err := http.NewRequestWithContext(ctx, http.MethodGet,
		p.BaseURL+"/phim/"+url.PathEscape(slug), nil)
	if err != nil {
		return detail, ErrUnavailable
	}
	request.Header.Set("Accept", "application/json")
	response, err := p.Client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return detail, ctx.Err()
		}
		return detail, ErrUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return detail, ErrNotFound
	}
	if response.StatusCode != http.StatusOK {
		return detail, ErrUnavailable
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, (4<<20)+1))
	if err != nil || len(payload) > 4<<20 || json.Unmarshal(payload, &detail) != nil {
		return detail, ErrInvalidData
	}
	if !detail.Status {
		return detail, ErrNotFound
	}
	if detail.Movie.Name == "" || detail.Movie.Slug != slug {
		return detail, ErrInvalidData
	}

	return detail, nil
}
