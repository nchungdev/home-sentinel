package catalog

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/nchungdev/home-sentinel/internal/cache"
)

type Service struct {
	Sources map[string]Source
	Cache   cache.Store
}

func (s *Service) source(id string) (Source, error) {
	name, _, found := strings.Cut(id, ":")
	if !found || len(id) > 2048 {
		return nil, ErrInvalidID
	}
	source, found := s.Sources[name]
	if !found {
		return nil, ErrUnavailable
	}

	return source, nil
}

func (s *Service) GetTitle(ctx context.Context, id string) (Title, error) {
	source, err := s.source(id)
	if err != nil {
		return Title{}, err
	}
	key := cache.Key("catalog-title-v1", source.Identity(), id)
	if s.Cache != nil {
		cacheContext, cancel := context.WithTimeout(ctx, 150*time.Millisecond)
		value, cacheErr := s.Cache.Get(cacheContext, key)
		cancel()
		var title Title
		if cacheErr == nil && json.Unmarshal(value, &title) == nil && title.ID == id {
			return title, nil
		}
	}
	title, err := source.GetTitle(ctx, id)
	if err == nil && s.Cache != nil {
		value, _ := json.Marshal(title)
		cacheContext, cancel := context.WithTimeout(ctx, 150*time.Millisecond)
		_ = s.Cache.Set(cacheContext, key, value, 15*time.Minute)
		cancel()
	}

	return title, err
}

func (s *Service) ResolveEpisode(ctx context.Context, id string) (Stream, error) {
	source, err := s.source(id)
	if err != nil {
		return Stream{}, err
	}

	return source.ResolveEpisode(ctx, id)
}
