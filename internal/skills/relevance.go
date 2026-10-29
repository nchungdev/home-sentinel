package skills

import (
	"strings"

	"github.com/nchungdev/home-sentinel/internal/contracts"
	"github.com/nchungdev/home-sentinel/internal/integrations/haclient"
)

const minWordLength = 3

var strictSources = map[string]bool{"plex": true, "jellyfin": true}

func relevantFirst(source, query string, items []contracts.MediaItem) []contracts.MediaItem {
	words := significantWords(query)
	if len(words) == 0 {
		return items
	}
	matched := make([]contracts.MediaItem, 0, len(items))
	for _, item := range items {
		if titleMatches(item.Title, words) || titleMatches(item.Original, words) {
			matched = append(matched, item)
		}
	}
	if len(matched) == 0 && !strictSources[source] {
		return items
	}
	return matched
}

func significantWords(text string) []string {
	words := []string{}
	for _, word := range strings.Fields(haclient.Normalize(text)) {
		if len([]rune(word)) >= minWordLength {
			words = append(words, word)
		}
	}
	return words
}

func titleMatches(title string, queryWords []string) bool {
	titleWords := strings.Fields(haclient.Normalize(title))
	for _, query := range queryWords {
		for _, word := range titleWords {
			if similarWord(strings.Trim(word, ".,:;!?()[]\"'"), query) {
				return true
			}
		}
	}
	return false
}

func similarWord(word, query string) bool {
	if word == query || (len(query) >= 4 && strings.HasPrefix(word, query)) {
		return true
	}
	allowed := 1
	if len(query) >= 8 {
		allowed = 2
	}
	return len(query) >= 4 && editDistance(word, query) <= allowed
}

func editDistance(left, right string) int {
	a, b := []rune(left), []rune(right)
	previous := make([]int, len(b)+1)
	for j := range previous {
		previous[j] = j
	}
	for i := 1; i <= len(a); i++ {
		current := make([]int, len(b)+1)
		current[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			current[j] = min(previous[j]+1, current[j-1]+1, previous[j-1]+cost)
		}
		previous = current
	}
	return previous[len(b)]
}
