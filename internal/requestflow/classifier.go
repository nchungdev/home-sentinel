package requestflow

import (
	"context"
	"net/url"
	"strings"
)

const maxBareTitleWords = 6

type RuleClassifier struct{}

func (RuleClassifier) Understand(ctx context.Context, input Input) (Understanding, error) {
	if err := ctx.Err(); err != nil {
		return Understanding{}, err
	}

	query := strings.Join(strings.Fields(input.Text), " ")
	lower := strings.ToLower(query)
	result := Understanding{
		Domain: "general", Intent: "clarify", Query: query,
		Entities: map[string]string{}, Method: "rules",
		NeedsInput: true, Question: "Bạn muốn tra cứu thông tin hay tìm nội dung để xem/nghe?",
	}
	for _, word := range strings.Fields(query) {
		parsed, err := url.Parse(word)
		if err == nil && parsed.Hostname() != "" && parsed.User == nil &&
			(parsed.Scheme == "https" || parsed.Scheme == "http") {
			result.Entities["url"] = word
			result.Domain, result.Intent = "web", "inspect_url"
			if containsAny(lower, "tóm tắt", "đọc bài", "nội dung bài") {
				result.Intent = "summarize_url"
			}
			result.NeedsInput, result.Question = false, ""
			return enrichUnderstanding(input, result), nil
		}
	}

	switch {
	case isControlRequest(lower):
		result.Domain, result.Intent = "device", "control"
	case containsAny(lower, "đạo diễn", "diễn viên", "đánh giá phim",
		"nội dung phim", "thông tin phim") ||
		(strings.Contains(lower, "phim") && containsAny(lower,
			"thông tin", "tóm tắt", "đánh giá", "ra mắt", "năm nào")):
		result.Domain, result.Intent = "media", "lookup"
	case containsAny(lower, "phim", "bài hát", "video", "nhạc", "để xem", "để nghe"):
		result.Domain, result.Intent = "media", "find_to_watch"
		if strings.HasPrefix(lower, "mở ") || strings.HasPrefix(lower, "phát ") {
			result.Intent = "play"
		}
		if containsAny(lower, "thêm", "yêu cầu tải") && strings.Contains(lower, "thư viện") {
			result.Intent = "request_library"
		}
		if strings.HasPrefix(lower, "tìm cách") || strings.Contains(lower, "lỗi") {
			result.Domain, result.Intent = "technical", "lookup"
		}
	case containsAny(lower, "sửa lỗi", "windows", "linux", "docker",
		"lập trình", " api", "tìm cách"):
		result.Domain, result.Intent = "technical", "lookup"
	case containsAny(lower, "tìm ", "tra cứu", "tin tức", "thời tiết", "là gì", "ai là"):
		result.Domain, result.Intent = "general", "lookup"
	case lower == "xin chào" || lower == "chào" || lower == "hello":
		result.Intent = "chat"
	case isBareTitle(query):
		result.Domain, result.Intent = "media", "find_to_watch"
	default:
		return enrichUnderstanding(input, result), nil
	}

	result.NeedsInput, result.Question = false, ""
	return enrichUnderstanding(input, result), nil
}

func isBareTitle(query string) bool {
	words := strings.Fields(query)
	if len(words) == 0 || len(words) > maxBareTitleWords {
		return false
	}

	return !strings.ContainsAny(query, "?!")
}

func containsAny(text string, phrases ...string) bool {
	for _, phrase := range phrases {
		if strings.Contains(text, phrase) {
			return true
		}
	}

	return false
}
