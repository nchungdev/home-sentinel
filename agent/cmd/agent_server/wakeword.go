package main

import (
	"strings"
)

func isWhisperHallucination(s string) bool {
	return strings.Contains(s, "subscribe") ||
		strings.Contains(s, "đăng ký") ||
		strings.Contains(s, "ghiền mì gõ") ||
		strings.Contains(s, "mì gõ") ||
		strings.Contains(s, "video hấp dẫn") ||
		strings.Contains(s, "nói nơi") ||
		strings.Contains(s, "cảm ơn các bạn đã theo dõi") ||
		strings.Contains(s, "hẹn gặp lại") ||
		strings.Contains(s, "chúc các bạn") ||
		strings.Contains(s, "like và chia sẻ")
}

func matchesWakeWord(text string) (bool, string) {
	clean := strings.ToLower(strings.TrimSpace(text))
	clean = strings.ReplaceAll(clean, ".", "")
	clean = strings.ReplaceAll(clean, ",", "")
	clean = strings.ReplaceAll(clean, "!", "")
	clean = strings.ReplaceAll(clean, "?", "")

	wakeWords := []string{
		"duinch ơi", "duinch oi", "duinch",
		"đu in ơi", "đu in oi", "đu in",
		"đuyn ơi", "đuyn oi", "đuyn",
		"đo in ơi", "đo in",
		"đui ơi", "đui",
		"do in", "du in", "dương ơi", "duyên ơi",
		"nui ơi", "nui nui", "núi ơi", "nu in",
	}

	for _, w := range wakeWords {
		if clean == w {
			return true, ""
		}
		if strings.HasPrefix(clean, w+" ") {
			query := strings.TrimSpace(strings.TrimPrefix(clean, w))
			return true, query
		}
	}
	return false, ""
}
