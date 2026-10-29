package mediasearch

import (
	"context"

	"github.com/nchungdev/home-sentinel/internal/contracts"
)

func (s *Service) SearchSource(
	ctx context.Context, sourceID, query string,
) contracts.MediaSourceResults {
	return s.searchSourceByID(ctx, sourceID, query, false)
}

// SearchSourceWide searches with the source's larger identity-matching pool when it has one,
// falling back to the regular display-limited search otherwise.
func (s *Service) SearchSourceWide(
	ctx context.Context, sourceID, query string,
) contracts.MediaSourceResults {
	return s.searchSourceByID(ctx, sourceID, query, true)
}

func (s *Service) searchSourceByID(
	ctx context.Context, sourceID, query string, wide bool,
) contracts.MediaSourceResults {
	for index, source := range s.sources {
		if source.Info().ID != sourceID {
			continue
		}
		updates := make(chan sourceUpdate, 2)
		s.searchSource(ctx, index, query, wide, updates)
		<-updates
		result := <-updates
		return *result.result
	}

	return contracts.MediaSourceResults{
		SourceID: sourceID, Status: "skipped", ErrorCode: "source_unavailable",
		Items: []contracts.MediaItem{},
	}
}
