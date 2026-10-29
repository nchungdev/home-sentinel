package mediasearch

import (
	"context"
	"encoding/json"
	"time"

	"github.com/nchungdev/home-sentinel/internal/cache"
	"github.com/nchungdev/home-sentinel/internal/contracts"
)

const (
	searchCacheTTL     = 2 * time.Minute
	searchCacheMaxSize = 128 << 10
)

type cacheIdentity interface {
	CacheIdentity() string
}

func sourceCacheIdentity(source Source) string {
	if identity, ok := source.(cacheIdentity); ok {
		return identity.CacheIdentity()
	}

	return source.Info().ID
}

func getSearchCache(
	ctx context.Context,
	store cache.Store,
	key string,
) ([]contracts.MediaItem, bool) {
	if store == nil {
		return nil, false
	}

	cacheContext, cancel := context.WithTimeout(ctx, 150*time.Millisecond)
	defer cancel()
	data, err := store.Get(cacheContext, key)
	if err != nil {
		return nil, false
	}
	var items []contracts.MediaItem
	if json.Unmarshal(data, &items) != nil || items == nil {
		return nil, false
	}

	return items, true
}

func putSearchCache(
	ctx context.Context,
	store cache.Store,
	key string,
	items []contracts.MediaItem,
) {
	if store == nil {
		return
	}
	data, err := json.Marshal(items)
	if err != nil || len(data) > searchCacheMaxSize {
		return
	}

	cacheContext, cancel := context.WithTimeout(ctx, 150*time.Millisecond)
	defer cancel()
	_ = store.Set(cacheContext, key, data, searchCacheTTL)
}

func filterPlayableActions(items []contracts.MediaItem, canPlay func(string) bool) {
	for itemIndex := range items {
		actions := make([]contracts.MediaAction, 0, len(items[itemIndex].Actions))
		for _, action := range items[itemIndex].Actions {
			if action.Type == "play" && (canPlay == nil || !canPlay(action.URL)) {
				continue
			}
			actions = append(actions, action)
		}
		items[itemIndex].Actions = actions
	}
}
