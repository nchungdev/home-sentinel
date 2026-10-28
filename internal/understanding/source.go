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
			`(?:trên|từ|ở|trong)\s+(?:nas|máy|server|thư\s+viện)|plex)([^\p{L}\p{N}]|$)`,
	)
	tvTargetPattern = regexp.MustCompile(
		`(?i)(^|[^\p{L}\p{N}])(?:trên|lên|ở|qua)\s+(?:tv|tivi|ti\s+vi)([^\p{L}\p{N}]|$)`,
	)
	libraryQuestionPattern = regexp.MustCompile(
		`(?i)^(?:có|còn)\s+(?:phim|chương\s+trình|show)\s+(?:gì|nào)`,
	)
)

func ExtractExplicitSource(rawText string) (string, string) {
	rawText = tvTargetPattern.ReplaceAllString(rawText, "${1} ${2}")
	if youtubePattern.MatchString(rawText) {
		cleaned := youtubePattern.ReplaceAllString(rawText, "${1} ${2}")
		return SourceYouTube, strings.Join(strings.Fields(cleaned), " ")
	}

	if plexPattern.MatchString(rawText) {
		cleaned := plexPattern.ReplaceAllString(rawText, "${1} ${2}")
		return SourcePlex, strings.Join(strings.Fields(cleaned), " ")
	}

	normalized := strings.Join(strings.Fields(rawText), " ")
	if libraryQuestionPattern.MatchString(normalized) {
		return SourcePlex, normalized
	}
	return "", normalized
}
