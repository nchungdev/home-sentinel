package main

import (
	"strings"
)

func isWhisperHallucination(s string) bool {
	clean := strings.TrimSpace(strings.Trim(s, ".!,? "))
	if clean == "cảm ơn" || clean == "cam on" || clean == "cảm ơn bạn" {
		return true
	}
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

// NormalizeSmartHomeQuery đoán ý và nắn chỉnh các lỗi phát âm / STT phổ biến về câu lệnh chuẩn
func NormalizeSmartHomeQuery(text string) string {
	s := strings.TrimSpace(text)
	lower := strings.ToLower(s)
	lower = strings.ReplaceAll(lower, ".", "")
	lower = strings.ReplaceAll(lower, ",", "")
	lower = strings.ReplaceAll(lower, "!", "")
	lower = strings.ReplaceAll(lower, "?", "")

	// 1. Nhận diện ý "tắt đèn": người dùng nói "tắt đầy", "tắt này", "tất đầy", "tất đèn", "tắt đè"
	if strings.Contains(lower, "tất đầy") ||
		strings.Contains(lower, "tắt đầy") ||
		strings.Contains(lower, "tắt này") ||
		strings.Contains(lower, "tất này") ||
		strings.Contains(lower, "tất đèn") ||
		strings.Contains(lower, "tắt đè") {
		return "tắt đèn"
	}

	// 2. Nhận diện ý "bật đèn": "bật đè", "bật đem", "mở đền", "bật đền"
	if strings.Contains(lower, "bật đè") ||
		strings.Contains(lower, "bật đem") ||
		strings.Contains(lower, "mở đền") ||
		strings.Contains(lower, "bật đền") {
		return "bật đèn"
	}

	// 3. Nhận diện ý "mở TV / bật TV"
	if strings.Contains(lower, "mở ti vi") ||
		strings.Contains(lower, "mở tivi") ||
		strings.Contains(lower, "bật ti vi") ||
		strings.Contains(lower, "bật tivi") {
		return "mở TV"
	}

	return s
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
		"tất cả ơi", "tất cả", "tất ca",
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
