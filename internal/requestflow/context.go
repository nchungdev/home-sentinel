package requestflow

import (
	"regexp"
	"strings"
)

var queryPrefix = regexp.MustCompile(
	`(?i)^(?:tìm thông tin về|tìm kiếm|tìm|mở|phát|xem|cho xem)\s+(?:phim\s+)?`,
)
var metadataPrefix = regexp.MustCompile(
	`(?i)^(?:ai (?:là )?đạo diễn|đạo diễn của|thông tin (?:về )?phim)\s+(?:phim\s+)?`,
)

var libraryQuestion = regexp.MustCompile(
	`(?i)^(?:trong|trên) (?:plex|jellyfin) có (?:phim )?(.+?) (?:không|ko)\??$`,
)

var knownSources = []string{
	"plex", "jellyfin", "radarr", "sonarr", "phimapi", "ophim", "nguonc",
	"motchill", "rophim", "phimtuoitho", "youtube", "internet", "tmdb", "official_docs",
}

func includesSource(sources []string, candidate string) bool {
	for _, source := range sources {
		if source == candidate {
			return true
		}
	}

	return false
}

func enrichUnderstanding(input Input, result Understanding) Understanding {
	for _, source := range input.Context.Sources {
		if includesSource(knownSources, source) && !includesSource(result.Sources, source) {
			result.Sources = append(result.Sources, source)
		}
	}
	lower := strings.ToLower(input.Text)
	for _, source := range knownSources {
		if strings.Contains(lower, "trên "+source) || strings.Contains(lower, "trong "+source) {
			result.Sources = []string{source}
			break
		}
	}
	refersToTitle := containsAny(lower, "phim này", "phim đó", "bộ này") || lower == "để xem"
	if result.Domain == "media" && refersToTitle && input.Context.Title == "" {
		result.NeedsInput = true
		result.Question = "Bạn đang muốn nói đến phim nào?"
		return result
	}
	if input.Context.Title != "" && refersToTitle {
		result.Entities["title"] = strings.TrimSpace(input.Context.Title)
	} else if result.Domain == "media" {
		query := metadataPrefix.ReplaceAllString(result.Query, "")
		query = queryPrefix.ReplaceAllString(query, "")
		if match := libraryQuestion.FindStringSubmatch(result.Query); len(match) == 2 {
			query = match[1]
		}
		for _, source := range knownSources {
			for _, prefix := range []string{" trên ", " trong "} {
				if index := strings.Index(strings.ToLower(query), prefix+source); index >= 0 {
					query = query[:index]
				}
			}
		}
		query = strings.TrimSuffix(strings.TrimSpace(query), "để xem")
		result.Entities["title"] = strings.Trim(query, " ?.!\t")
	}

	return result
}

func taskQuery(understanding Understanding, skill string) string {
	if skill != "web.search" && skill != "documentation.search" &&
		understanding.Entities["title"] != "" {
		return understanding.Entities["title"]
	}

	return understanding.Query
}

func ValidContext(value InputContext) bool {
	if len(value.Title) > 500 || len(value.Sources) > len(knownSources) {
		return false
	}
	for _, source := range value.Sources {
		if !includesSource(knownSources, source) {
			return false
		}
	}
	return true
}

func isControlRequest(text string) bool {
	for _, command := range []string{"tạm dừng", "phát tiếp", "tắt tiếng", "tua ", "lùi lại"} {
		if strings.HasPrefix(text, command) {
			return true
		}
	}
	verbs := []string{"bật ", "tắt ", "mở ", "đóng ", "khóa ", "đặt ", "tăng ", "giảm "}
	for _, verb := range verbs {
		if strings.HasPrefix(text, verb) && containsAny(text, "đèn", "rèm", "cửa", "camera",
			"điều hòa", "quạt", "âm lượng", "phụ đề", "youtube", "netflix", "plex") {
			return true
		}
	}
	return false
}
