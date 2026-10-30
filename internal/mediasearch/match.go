package mediasearch

import (
	"context"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"github.com/nchungdev/home-sentinel/internal/contracts"
	"golang.org/x/text/unicode/norm"
)

var identitySources = []string{"plex", "jellyfin", "radarr", "sonarr", "phimapi", "ophim", "nguonc", "motchill", "rophim", "phimtuoitho"}

func (s *Service) SearchMatching(
	ctx context.Context,
	title, kind string,
	year int,
	aliases []string,
	externalIDs map[string]string,
) []contracts.MediaSourceResults {
	return s.SearchMatchingStream(ctx, title, kind, year, aliases, externalIDs, nil)
}

func (s *Service) SearchMatchingStream(
	ctx context.Context,
	title, kind string,
	year int,
	aliases []string,
	externalIDs map[string]string,
	onSource func(contracts.MediaSourceResults),
) []contracts.MediaSourceResults {
	targetSources := make([]string, 0, len(identitySources))
	for _, sourceID := range identitySources {
		if kind == "movie" && sourceID == "sonarr" {
			continue
		}
		if kind == "show" && sourceID == "radarr" {
			continue
		}
		targetSources = append(targetSources, sourceID)
	}

	results := make([]contracts.MediaSourceResults, len(targetSources))
	var workers sync.WaitGroup
	var mutex sync.Mutex

	for index, sourceID := range targetSources {
		workers.Add(1)
		go func(idx int, id string) {
			defer workers.Done()
			var res contracts.MediaSourceResults
			if id == "phimapi" {
				res = s.SearchPhimAPI(ctx, title, kind, year, aliases, externalIDs)
			} else if id == "plex" || id == "jellyfin" {
				res = s.searchLibrarySourceMatching(ctx, id, title, kind, year, aliases, externalIDs)
			} else {
				res = s.searchArrSourceMatching(ctx, id, title, kind, externalIDs)
			}

			mutex.Lock()
			results[idx] = res
			mutex.Unlock()

			if onSource != nil {
				onSource(res)
			}
		}(index, sourceID)
	}
	workers.Wait()

	return results
}

func (s *Service) searchLibrarySourceMatching(
	ctx context.Context,
	sourceID, title, kind string,
	year int,
	aliases []string,
	externalIDs map[string]string,
) contracts.MediaSourceResults {
	queries := identityQueries(title, aliases)
	var firstResult contracts.MediaSourceResults
	matches := []contracts.MediaItem{}
	seenItems := map[string]bool{}

	expectedTitles := make(map[string]struct{}, len(aliases)+1)
	expectedTitles[normalizeIdentityTitle(title)] = struct{}{}
	for _, alias := range aliases {
		if normalized := normalizeIdentityTitle(alias); normalized != "" {
			expectedTitles[normalized] = struct{}{}
		}
	}

	for _, query := range queries {
		result := s.SearchSource(ctx, sourceID, query)
		if firstResult.Status == "" {
			firstResult = result
		}
		for _, item := range result.Items {
			if seenItems[item.ID] || item.Kind != kind {
				continue
			}
			if sharesExternalID(item.ExternalIDs, externalIDs) {
				item.MatchMethod = "external_id"
				seenItems[item.ID] = true
				matches = append(matches, item)
			} else if !hasConflictingExternalID(item.ExternalIDs, externalIDs) &&
				year > 0 && item.Year == year && matchesAnyTitle(item, expectedTitles) {
				item.MatchMethod = "title_year"
				seenItems[item.ID] = true
				matches = append(matches, item)
			}
		}
		if len(matches) > 0 {
			break
		}
	}
	firstResult.SourceID = sourceID
	firstResult.Items = matches
	if len(matches) > 0 {
		firstResult.Status = "ok"
		firstResult.ErrorCode = ""
	} else if firstResult.Status == "ok" {
		firstResult.Status = "empty"
		firstResult.ErrorCode = "identity_not_verified"
	}
	return firstResult
}

func (s *Service) searchArrSourceMatching(
	ctx context.Context,
	sourceID, title, kind string,
	externalIDs map[string]string,
) contracts.MediaSourceResults {
	result := s.SearchSource(
		ctx, sourceID, identitySearchQuery(sourceID, title, externalIDs),
	)
	if result.SourceID == "" {
		result.SourceID = sourceID
	}
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
	return result
}

func identityQueries(title string, aliases []string) []string {
	candidates := append([]string{title}, aliases...)
	seen := make(map[string]bool)
	queries := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" || seen[candidate] {
			continue
		}
		seen[candidate] = true
		queries = append(queries, candidate)
		if len(queries) == 5 {
			break
		}
	}
	return queries
}

var seasonSuffixRegex = regexp.MustCompile(`(?i)\s*[\(\[\{-]?\s*(phan|mua|season|part|p|s)\s*(\d+)[\)\]\}]?\s*$`)

func matchTitleOrSeason(item contracts.MediaItem, expected map[string]struct{}) (bool, int) {
	candidates := append([]string{item.Title, item.Original}, item.AlternativeTitles...)
	for _, candidate := range candidates {
		normalized := normalizeIdentityTitle(candidate)
		if normalized == "" {
			continue
		}
		if _, found := expected[normalized]; found {
			return true, 0
		}
		matches := seasonSuffixRegex.FindStringSubmatch(normalized)
		if len(matches) == 3 {
			base := strings.TrimSpace(normalized[:len(normalized)-len(matches[0])])
			if _, found := expected[base]; found {
				seasonNum, _ := strconv.Atoi(matches[2])
				return true, seasonNum
			}
		}
	}
	return false, 0
}

func matchPhimAPIItems(
	items []contracts.MediaItem,
	title string,
	aliases []string,
	year int,
	kind string,
	externalIDs map[string]string,
) []contracts.MediaItem {
	expectedTitles := make(map[string]struct{}, len(aliases)+1)
	expectedTitles[normalizeIdentityTitle(title)] = struct{}{}
	for _, alias := range aliases {
		if normalized := normalizeIdentityTitle(alias); normalized != "" {
			expectedTitles[normalized] = struct{}{}
		}
	}

	matchedItems := make([]contracts.MediaItem, 0)
	seenIDs := make(map[string]bool)

	// Step 1: External ID matches
	for _, item := range items {
		if item.Kind != kind {
			continue
		}
		if sharesExternalID(item.ExternalIDs, externalIDs) {
			item.MatchMethod = "external_id"
			seenIDs[item.ID] = true
			matchedItems = append(matchedItems, item)
		}
	}

	// Step 2: Title / Season matches
	titleCandidates := make([]contracts.MediaItem, 0)
	hasSeasonSuffixes := false

	for _, item := range items {
		if item.Kind != kind || seenIDs[item.ID] {
			continue
		}
		if hasConflictingExternalID(item.ExternalIDs, externalIDs) {
			continue
		}
		if kind == "movie" && (year == 0 || item.Year != year) {
			continue
		}

		matched, extractedSeason := matchTitleOrSeason(item, expectedTitles)
		if !matched {
			continue
		}

		if extractedSeason > 0 {
			hasSeasonSuffixes = true
			if item.Season == nil {
				item.Season = &extractedSeason
			}
		} else {
			if year == 0 || item.Year != year {
				continue
			}
		}

		item.MatchMethod = "title_year"
		titleCandidates = append(titleCandidates, item)
	}

	if hasSeasonSuffixes || len(matchedItems) == 0 {
		if hasSeasonSuffixes || len(titleCandidates) == 1 {
			for _, candidate := range titleCandidates {
				if !seenIDs[candidate.ID] {
					seenIDs[candidate.ID] = true
					matchedItems = append(matchedItems, candidate)
				}
			}
		}
	}

	sort.SliceStable(matchedItems, func(i, j int) bool {
		s1, s2 := 0, 0
		if matchedItems[i].Season != nil {
			s1 = *matchedItems[i].Season
		}
		if matchedItems[j].Season != nil {
			s2 = *matchedItems[j].Season
		}
		if s1 != s2 {
			return s1 < s2
		}
		if matchedItems[i].Year != matchedItems[j].Year {
			return matchedItems[i].Year < matchedItems[j].Year
		}
		return matchedItems[i].Title < matchedItems[j].Title
	})

	return matchedItems
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
	if hasConflictingExternalID(itemIDs, expectedIDs) {
		return false
	}
	for key, expected := range expectedIDs {
		if key == "type" || expected == "" {
			continue
		}
		for candidateKey, candidate := range itemIDs {
			if strings.EqualFold(candidateKey, key) && candidate != "" && candidate == expected {
				return true
			}
		}
	}
	return false
}
