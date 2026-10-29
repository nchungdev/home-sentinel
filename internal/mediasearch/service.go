package mediasearch

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/nchungdev/home-sentinel/internal/cache"
	"github.com/nchungdev/home-sentinel/internal/contracts"
)

const (
	probeTimeout  = 4 * time.Second
	searchTimeout = 15 * time.Second
)

var ErrNotConfigured = errors.New("not_configured")
var ErrAuthRequired = errors.New("auth_required")

type Source interface {
	Info() contracts.MediaSourceCapability
	Capability(context.Context) contracts.MediaSourceCapability
	Search(context.Context, string) ([]contracts.MediaItem, error)
}

type Requester interface {
	Request(context.Context, string, RequestOptions) (contracts.MediaItem, error)
}

type capabilityCache struct {
	capability contracts.MediaSourceCapability
	checked    time.Time
}

type Service struct {
	sources     []Source
	mutex       sync.Mutex
	cache       map[int]capabilityCache
	searchCache cache.Store
	canPlay     func(string) bool
}

func New(sources ...Source) *Service {
	return &Service{sources: sources, cache: make(map[int]capabilityCache)}
}

func (s *Service) inspect(ctx context.Context, index int, refresh bool) contracts.MediaSourceCapability {
	s.mutex.Lock()
	cached, found := s.cache[index]
	s.mutex.Unlock()
	if !refresh && found && time.Since(cached.checked) < 30*time.Second {
		return cached.capability
	}
	capability := s.probe(ctx, index)
	if capability.Status == "unreachable" && ctx.Err() == nil {
		capability = s.probe(ctx, index)
	}
	if ctx.Err() == nil {
		s.mutex.Lock()
		s.cache[index] = capabilityCache{capability: capability, checked: time.Now()}
		s.mutex.Unlock()
	}
	return capability
}

func (s *Service) probe(ctx context.Context, index int) contracts.MediaSourceCapability {
	probeContext, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	return s.sources[index].Capability(probeContext)
}

func (s *Service) Discover(ctx context.Context, refresh bool) []contracts.MediaSourceCapability {
	capabilities := make([]contracts.MediaSourceCapability, len(s.sources))
	var workers sync.WaitGroup
	for index := range s.sources {
		workers.Add(1)
		go func() { defer workers.Done(); capabilities[index] = s.inspect(ctx, index, refresh) }()
	}
	workers.Wait()
	return capabilities
}

type sourceUpdate struct {
	index      int
	capability *contracts.MediaSourceCapability
	result     *contracts.MediaSourceResults
}

func (s *Service) searchSource(ctx context.Context, index int, query string, updates chan<- sourceUpdate) {
	capability := s.inspect(ctx, index, false)
	updates <- sourceUpdate{index: index, capability: &capability}
	result := contracts.MediaSourceResults{
		SourceID: capability.ID, Status: "skipped", Cache: "bypassed",
		Items: []contracts.MediaItem{},
	}
	if !capability.Available {
		result.ErrorCode = capability.Status
		updates <- sourceUpdate{index: index, result: &result}
		return
	}
	sourceContext, cancel := context.WithTimeout(ctx, searchTimeout)
	defer cancel()
	var items []contracts.MediaItem
	cached := false
	cacheKey := ""
	if s.searchCache != nil {
		result.Cache = "miss"
		cacheKey = cache.Key("media:v2", sourceCacheIdentity(s.sources[index]), query, "vi")
		items, cached = getSearchCache(sourceContext, s.searchCache, cacheKey)
	}
	if cached {
		result.Cache = "hit"
	}
	var err error
	if !cached {
		items, err = s.sources[index].Search(sourceContext, query)
		if err == nil && s.searchCache != nil {
			putSearchCache(sourceContext, s.searchCache, cacheKey, items)
		}
	}
	if items == nil {
		items = []contracts.MediaItem{}
	}
	filterPlayableActions(items, s.canPlay)
	result.Searched = true
	result.Items = items
	if err != nil {
		result.Status, result.ErrorCode = "error", "search_failed"
		if errors.Is(err, ErrAuthRequired) {
			result.ErrorCode = "auth_required"
		}
		if errors.Is(err, ErrNotImplemented) {
			result.ErrorCode = "not_implemented"
		}
		if errors.Is(err, context.DeadlineExceeded) {
			result.ErrorCode = "timeout"
		}
	} else {
		result.Status = "empty"
		if len(items) > 0 {
			result.Status = "ok"
		}
	}
	updates <- sourceUpdate{index: index, result: &result}
}

func (s *Service) Search(ctx context.Context, query string) contracts.MediaSearchResponse {
	response := contracts.MediaSearchResponse{Version: 1, Query: query,
		RequestID: rand.Text(), Status: "running",
		Capabilities: make([]contracts.MediaSourceCapability, len(s.sources)),
		Sources:      make([]contracts.MediaSourceResults, len(s.sources)),
		ContentTypes: []contracts.MediaContentType{}}
	for index, source := range s.sources {
		response.Capabilities[index] = source.Info()
		response.Sources[index] = contracts.MediaSourceResults{
			SourceID: source.Info().ID, Status: "checking", Items: []contracts.MediaItem{}}
	}
	sequence := 0
	emit := func(event contracts.MediaSearchEvent) {
		response.ContentTypes = contentTypes(response.Sources)
		sequence++
		event.Version, event.RequestID, event.Sequence = 1, response.RequestID, sequence
		event.Status, event.Snapshot = response.Status, &response
		contracts.EmitMediaSearch(ctx, event)
	}
	emit(contracts.MediaSearchEvent{Type: "search_started", Query: query,
		Sources: append([]contracts.MediaSourceCapability(nil), response.Capabilities...)})
	updates := make(chan sourceUpdate, len(s.sources)*2)
	for index := range s.sources {
		go s.searchSource(ctx, index, query, updates)
	}
	for completed := 0; completed < len(s.sources); {
		var update sourceUpdate
		select {
		case update = <-updates:
		case <-ctx.Done():
			response.Status = "cancelled"
			for index := range response.Sources {
				if response.Sources[index].Status == "checking" {
					response.Sources[index].Status = "cancelled"
				}
			}
			emit(contracts.MediaSearchEvent{Type: "search_completed"})
			return response
		}
		event := contracts.MediaSearchEvent{SourceID: response.Sources[update.index].SourceID, Snapshot: &response}
		if update.capability != nil {
			response.Capabilities[update.index] = *update.capability
			event.Type, event.Capability = "source_capability", update.capability
		} else {
			response.Sources[update.index] = *update.result
			event.Type, event.Result = "source_results", update.result
			completed++
		}
		emit(event)
	}
	response.Status = "completed"
	if ctx.Err() != nil {
		response.Status = "cancelled"
	}
	emit(contracts.MediaSearchEvent{Type: "search_completed"})
	return response
}

func (s *Service) Request(ctx context.Context, sourceID, itemID string, options RequestOptions) (contracts.MediaItem, error) {
	for index, source := range s.sources {
		if source.Info().ID != sourceID {
			continue
		}
		capability := s.inspect(ctx, index, true)
		if capability.Status == "not_implemented" {
			return contracts.MediaItem{}, ErrNotImplemented
		}
		if !capability.Available {
			return contracts.MediaItem{}, ErrNotConfigured
		}
		if requester, ok := source.(Requester); ok {
			return requester.Request(ctx, itemID, options)
		}
		break
	}
	return contracts.MediaItem{}, fmt.Errorf("nguồn không hỗ trợ yêu cầu thư viện")
}

func (s *Service) SetCache(store cache.Store) { s.searchCache = store }

func (s *Service) SetPlayable(check func(string) bool) { s.canPlay = check }
