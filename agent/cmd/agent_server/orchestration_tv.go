package main

import (
	"regexp"
	"strings"
)

func extractTVMessage(text string) (string, bool) {
	if idx := strings.Index(text, ":"); idx > 0 && idx < len(text)-1 {
		msg := strings.TrimSpace(text[idx+1:])
		if msg != "" {
			return msg, true
		}
	}

	sepPattern := regexp.MustCompile(
		`(?i)(?:nói|nhắn|gửi|bảo)\s+(?:với|lên|cho|tới|qua)?\s*(?:tv|tivi|màn\s+hình)\s+(?:là|rằng|với\s+nội\s+dung\s+là|nội\s+dung\s+là|nội\s+dung)\s*:?\s*(.+)`,
	)
	if m := sepPattern.FindStringSubmatch(text); len(m) > 1 {
		msg := strings.TrimSpace(m[1])
		if msg != "" {
			return msg, true
		}
	}

	directPattern := regexp.MustCompile(
		`(?i)(?:nói|nhắn|bảo)\s+(?:với|lên|cho|tới|qua)\s+(?:tv|tivi|màn\s+hình)\s+(.+)`,
	)
	if m := directPattern.FindStringSubmatch(text); len(m) > 1 {
		msg := strings.TrimSpace(m[1])
		if !isMetaInstruction(strings.ToLower(msg)) {
			return msg, true
		}
	}

	midPattern := regexp.MustCompile(
		`(?i)(?:gửi\s+(?:tin\s+nhắn|thông\s+báo|lời\s+nhắn))\s+(.+)\s+(?:lên|với|tới|cho|qua)\s+(?:tv|tivi|màn\s+hình)`,
	)
	if m := midPattern.FindStringSubmatch(text); len(m) > 1 {
		msg := strings.TrimSpace(m[1])
		if !isMetaInstruction(strings.ToLower(msg)) {
			return msg, true
		}
	}

	return "", false
}

func isMetaInstruction(s string) bool {
	ignoreList := []string{
		"của tôi", "đi", "nào", "nhé", "với", "hộ tôi", "giúp tôi",
		"gì đó", "câu gì đó", "lời chào", "câu chào", "tin nhắn",
	}
	for _, ign := range ignoreList {
		if s == ign || strings.HasPrefix(s, ign+" ") || strings.HasPrefix(s, "1 "+ign) || strings.HasPrefix(s, "một "+ign) {
			return true
		}
	}
	return false
}
