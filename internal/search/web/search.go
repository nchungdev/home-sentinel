package websearch

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nchungdev/home-sentinel/internal/cache"
)

var (
	ErrNotConfigured = errors.New("not_configured")
	ErrAuthRequired  = errors.New("auth_required")
	ErrProvider      = errors.New("provider_unavailable")
	ErrInvalid       = errors.New("invalid_source_response")
)

type Link struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Content string `json:"content"`
}

type Searcher struct {
	BaseURL string
	Client  *http.Client
	Cache   cache.Store
}

func New(baseURL string, store cache.Store) *Searcher {
	return &Searcher{BaseURL: strings.TrimRight(baseURL, "/"), Cache: store,
		Client: &http.Client{Timeout: 5 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			}}}
}

func (s *Searcher) Search(ctx context.Context, query string) ([]Link, string, error) {
	base, err := url.Parse(s.BaseURL)
	if err != nil || !ValidURL(s.BaseURL) || base.RawQuery != "" || base.Fragment != "" {
		return nil, "bypassed", ErrNotConfigured
	}
	key := cache.Key(s.BaseURL, query, "vi", "general")
	if s.Cache != nil {
		cacheCtx, cancel := context.WithTimeout(ctx, 150*time.Millisecond)
		cached, cacheErr := s.Cache.Get(cacheCtx, key)
		cancel()
		var links []Link
		if cacheErr == nil && json.Unmarshal(cached, &links) == nil {
			return links, "hit", nil
		}
	}
	endpoint := s.BaseURL + "/search?" + url.Values{
		"q": {query}, "format": {"json"}, "language": {"vi"}, "safesearch": {"1"},
	}.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, "miss", ErrNotConfigured
	}
	request.Header.Set("Accept", "application/json")
	response, err := s.Client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return nil, "miss", ctx.Err()
		}
		return nil, "miss", ErrProvider
	}
	defer response.Body.Close()
	if response.StatusCode == 401 || response.StatusCode == 403 {
		return nil, "miss", ErrAuthRequired
	}
	if response.StatusCode != http.StatusOK {
		return nil, "miss", ErrProvider
	}
	links, err := parseResults(response.Body)
	if err == nil && s.Cache != nil {
		value, _ := json.Marshal(links)
		cacheCtx, cancel := context.WithTimeout(ctx, 150*time.Millisecond)
		_ = s.Cache.Set(cacheCtx, key, value, 2*time.Minute)
		cancel()
	}

	return links, "miss", err
}

func ValidURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.User == nil && parsed.Hostname() != "" &&
		(parsed.Scheme == "https" || parsed.Scheme == "http")
}

func parseResults(body io.Reader) ([]Link, error) {
	data, err := io.ReadAll(io.LimitReader(body, (2<<20)+1))
	if err != nil || len(data) > 2<<20 {
		return nil, ErrInvalid
	}
	var payload struct {
		Results *[]Link `json:"results"`
	}
	if json.Unmarshal(data, &payload) != nil || payload.Results == nil {
		return nil, ErrInvalid
	}
	links, seen := []Link{}, map[string]bool{}
	for _, link := range *payload.Results {
		if !ValidURL(link.URL) || seen[link.URL] || strings.TrimSpace(link.Title) == "" {
			continue
		}
		seen[link.URL] = true
		links = append(links, link)
		if len(links) == 8 {
			break
		}
	}

	return links, nil
}
