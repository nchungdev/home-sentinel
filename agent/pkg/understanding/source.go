package understanding

import (
	"regexp"
	"strings"
)

var (
	youtubePattern = regexp.MustCompile(
		`(?i)(^|[^\p{L}\p{N}])(?:(?:trên|bằng|ở|qua|từ)\s+youtube|` +
			`youtube)([^\p{L}\p{N}]|$)`,
	)
	plexPattern = regexp.MustCompile(
		`(?i)(^|[^\p{L}\p{N}])(?:(?:trên|bằng|ở|qua|từ)\s+plex|` +
			`(?:trên|từ|ở)\s+nas|plex)([^\p{L}\p{N}]|$)`,
	)
)

func ExtractExplicitSource(rawText string) (string, string) {
	if youtubePattern.MatchString(rawText) {
		cleaned := youtubePattern.ReplaceAllString(rawText, "${1} ${2}")
		return SourceYouTube, strings.Join(strings.Fields(cleaned), " ")
	}

	if plexPattern.MatchString(rawText) {
		cleaned := plexPattern.ReplaceAllString(rawText, "${1} ${2}")
		return SourcePlex, strings.Join(strings.Fields(cleaned), " ")
	}

	return "", strings.Join(strings.Fields(rawText), " ")
}
