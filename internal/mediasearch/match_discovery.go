package mediasearch

import "github.com/nchungdev/home-sentinel/internal/contracts"

func matchPhimAPIItemsWithDiscovery(
	items []contracts.MediaItem,
	title string,
	tmdbAliases, aiQueries []string,
	year int,
	kind string,
	externalIDs map[string]string,
) []contracts.MediaItem {
	trustedMatches := matchPhimAPIItems(
		items, title, tmdbAliases, year, kind, externalIDs,
	)
	if len(trustedMatches) > 0 {
		return trustedMatches
	}
	if year == 0 {
		return []contracts.MediaItem{}
	}
	expectedAIQueries := make(map[string]struct{}, len(aiQueries))
	for _, query := range aiQueries {
		if normalized := normalizeIdentityTitle(query); normalized != "" {
			expectedAIQueries[normalized] = struct{}{}
		}
	}
	candidates := make([]contracts.MediaItem, 0)
	for _, item := range items {
		if item.Kind != kind || item.Year != year ||
			hasConflictingExternalID(item.ExternalIDs, externalIDs) ||
			!matchesAnyTitle(item, expectedAIQueries) {
			continue
		}
		item.MatchMethod = "ai_candidate"
		candidates = append(candidates, item)
	}

	return candidates
}
