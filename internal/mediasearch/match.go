package mediasearch

import (
	"context"
	"strings"
	"sync"
	"unicode"

	"github.com/nchungdev/home-sentinel/internal/contracts"
	"golang.org/x/text/unicode/norm"
)

var identitySources = []string{"plex", "jellyfin", "radarr", "sonarr", "phimapi"}

func (s *Service) SearchMatching(
	ctx context.Context,
	title, kind string,
	year int,
	aliases []string,
	externalIDs map[string]string,
) []contracts.MediaSourceResults {
	results := make([]contracts.MediaSourceResults, len(identitySources))
	var workers sync.WaitGroup
	for index, sourceID := range identitySources {
		workers.Add(1)
		go func() {
			defer workers.Done()
			if sourceID == "phimapi" {
				results[index] = s.SearchPhimAPI(
					ctx, title, kind, year, aliases, externalIDs,
				)
				return
			}

			result := s.SearchSource(
				ctx, sourceID, identitySearchQuery(sourceID, title, externalIDs),
			)
			matches := make([]contracts.MediaItem, 0, len(result.Items))
			for _, item := range result.Items {
				if item.Kind == kind && sharesExternalID(item.ExternalIDs, externalIDs) {
					item.MatchMethod = "external_id"
					matches = append(matches, item)
				}
			}
			result.Items = matches
			if result.Status == "ok" && len(matches) == 0 {
				result.Status = "empty"
				result.ErrorCode = "identity_not_verified"
			}
			results[index] = result
		}()
	}
	workers.Wait()

	return results
}

func matchPhimAPIItems(
	items []contracts.MediaItem,
	title string,
	aliases []string,
	year int,
	kind string,
	externalIDs map[string]string,
) []contracts.MediaItem {
	exactMatches := make([]contracts.MediaItem, 0)
	titleCandidates := make([]contracts.MediaItem, 0)
	expectedTitles := make(map[string]struct{}, len(aliases)+1)
	expectedTitles[normalizeIdentityTitle(title)] = struct{}{}
	for _, alias := range aliases {
		if normalized := normalizeIdentityTitle(alias); normalized != "" {
			expectedTitles[normalized] = struct{}{}
		}
	}

	for _, item := range items {
		if item.Kind != kind {
			continue
		}
		if sharesExternalID(item.ExternalIDs, externalIDs) {
			item.MatchMethod = "external_id"
			exactMatches = append(exactMatches, item)
			continue
		}
		if hasConflictingExternalID(item.ExternalIDs, externalIDs) ||
			year == 0 || item.Year != year {
			continue
		}
		if matchesAnyTitle(item, expectedTitles) {
			titleCandidates = append(titleCandidates, item)
		}
	}
	if len(exactMatches) > 0 {
		return exactMatches
	}
	if len(titleCandidates) != 1 {
		return []contracts.MediaItem{}
	}
	titleCandidates[0].MatchMethod = "title_year"

	return titleCandidates
}

func matchesAnyTitle(item contracts.MediaItem, expected map[string]struct{}) bool {
	candidates := append([]string{item.Title, item.Original}, item.AlternativeTitles...)
	for _, candidate := range candidates {
		if normalized := normalizeIdentityTitle(candidate); normalized != "" {
			if _, found := expected[normalized]; found {
				return true
			}
		}
	}

	return false
}

func normalizeIdentityTitle(title string) string {
	var normalized strings.Builder
	lastWasSeparator := true
	for _, character := range norm.NFD.String(norm.NFKC.String(strings.ToLower(title))) {
		if unicode.Is(unicode.Mn, character) {
			continue
		}
		if unicode.IsLetter(character) || unicode.IsNumber(character) {
			normalized.WriteRune(character)
			lastWasSeparator = false
		} else if !lastWasSeparator {
			normalized.WriteByte(' ')
			lastWasSeparator = true
		}
	}

	return strings.TrimSpace(normalized.String())
}

func hasConflictingExternalID(itemIDs, expectedIDs map[string]string) bool {
	for key, expected := range expectedIDs {
		if expected == "" {
			continue
		}
		for candidateKey, candidate := range itemIDs {
			if strings.EqualFold(candidateKey, key) && candidate != expected {
				return true
			}
		}
	}

	return false
}

func identitySearchQuery(sourceID, title string, externalIDs map[string]string) string {
	switch sourceID {
	case "radarr":
		if id := externalIDs["tmdb"]; id != "" {
			return "tmdb:" + id
		}
	case "sonarr":
		if id := externalIDs["tvdb"]; id != "" {
			return "tvdb:" + id
		}
	}

	return title
}

func sharesExternalID(itemIDs, expectedIDs map[string]string) bool {
	matched := false
	for key, expected := range expectedIDs {
		if key == "type" || expected == "" {
			continue
		}
		for candidateKey, candidate := range itemIDs {
			if !strings.EqualFold(candidateKey, key) {
				continue
			}
			if candidate != expected {
				return false
			}
			matched = true
		}
	}

	return matched
}
