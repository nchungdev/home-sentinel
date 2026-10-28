package mediasearch

import (
	"context"

	"github.com/nchungdev/home-sentinel/internal/contracts"
)

func (s *Service) SearchSource(
	ctx context.Context, sourceID, query string,
) contracts.MediaSourceResults {
	for index, source := range s.sources {
		if source.Info().ID != sourceID {
			continue
		}
		updates := make(chan sourceUpdate, 2)
		s.searchSource(ctx, index, query, updates)
		<-updates
		result := <-updates
		return *result.result
	}

	return contracts.MediaSourceResults{
		SourceID: sourceID, Status: "skipped", ErrorCode: "source_unavailable",
		Items: []contracts.MediaItem{},
	}
}
