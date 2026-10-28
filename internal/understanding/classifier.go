package understanding

import (
	"regexp"
	"strings"

	"github.com/nchungdev/home-sentinel/internal/contracts"
)

var (
	systemStatusPattern = regexp.MustCompile(
		`(?i)(trạng\s+thái|thông\s+tin|kiểm\s+tra|dung\s+lượng\s+ổ\s+cứng|` +
			`ổ\s+cứng|status)\s+(nas|docker|hệ\s+thống|system)|` +
			`(?i)(nas|docker)\s+(status|trạng\s+thái|còn|thế\s+nào|ra\s+sao)|` +
			`(?i)(ổ\s+cứng|ổ\s+đĩa|dung\s+lượng)\s+` +
			`(còn|trống|đã\s+dùng|bao\s+nhiêu|thế\s+nào|ra\s+sao)|` +
			`(?i)(container|docker)\s+(nào|đang\s+chạy|đang\s+dừng)|` +
			`(?i)^(?:trạng\s+thái|tình\s+trạng)\s+(?:máy|máy\s+chủ|server)\s*\??$`,
	)

	diskWordPattern = regexp.MustCompile(`ổ\s+cứng|ổ\s+đĩa|dung\s+lượng`)

	privacyOnPattern = regexp.MustCompile(
		`(?i)(bật|kích\s+hoạt|mở)\s+(?:chế\s+độ\s+)?(?:riêng\s+tư|privacy)`,
	)
	privacyOffPattern = regexp.MustCompile(
		`(?i)(tắt|hủy|ngắt)\s+(?:chế\s+độ\s+)?(?:riêng\s+tư|privacy)`,
	)
	cameraSpeakPattern = regexp.MustCompile(
		`(?i)(?:nói|phát|truyền)\s+(?:qua|vào)\s+camera(?:\s*:\s*|\s+)(.+)`,
	)
	cameraViewPattern = regexp.MustCompile(
		`(?i)^(?:xem|mở|bật|kiểm\s+tra)\s+camera(?:\s+(.+))?$`,
	)

	temperaturePattern = regexp.MustCompile(
		`(?i)(?:chỉnh|đặt|set|hạ|tăng|bật)?\s*(?:nhiệt\s+độ\s+)?` +
			`(điều\s+hòa|máy\s+lạnh)(?:\s+phòng\s+[^\d]+|\s+[^\d]+)?\s*` +
			`(?:lên|xuống|về|thành|ở\s+mức)?\s*(\d+(?:[.,]\d+)?)\s*` +
			`(?:độ|°c|c)?$`,
	)
	homeSwitchPattern = regexp.MustCompile(
		`(?i)^(bật|mở|khởi\s+động|tắt|đóng|ngắt)\s+` +
			`((?:đèn|quạt|điều\s+hòa|máy\s+lạnh|bình\s+nóng\s+lạnh|` +
			`nóng\s+lạnh|công\s+tắc|ổ\s+cắm|tivi|loa)(?:\s+.*)?)$`,
	)
	homeCurtainPattern = regexp.MustCompile(
		`(?i)^(mở|đóng|kéo)\s+(rèm(?:\s+.*)?)$`,
	)
	homeDoorPattern = regexp.MustCompile(
		`(?i)^(khóa|mở\s+khóa|mở|đóng)\s+(cửa(?:\s+.*)?)$`,
	)

	mediaVerbPattern = regexp.MustCompile(
		`(?i)^(tìm\s+kiếm|tìm|mở|phát|chiếu|xem|nghe\s+nhạc|nghe|play)\s+(.*)$`,
	)
	mediaKindPattern = regexp.MustCompile(
		`(?i)^(chương\s+trình|tập\s+phim|phim\s+bộ|phim\s+lẻ|phim|` +
			`bài\s+hát|ca\s+khúc|nhạc|video|clip|show|kênh)\s*(.*)$`,
	)
)

func classifySystemStatus(rawText string) *contracts.Intent {
	if !systemStatusPattern.MatchString(rawText) {
		return nil
	}

	lower := strings.ToLower(rawText)
	target := "system"
	switch {
	case strings.Contains(lower, "docker") || strings.Contains(lower, "container"):
		target = "docker"
	case strings.Contains(lower, "nas"):
		target = "nas"
	case diskWordPattern.MatchString(lower):
		target = "disk"
	}

	return &contracts.Intent{
		Name:         IntentSystemStatus,
		Domain:       DomainSystem,
		Confidence:   ConfidenceRuleMatched,
		Slots:        map[string]any{SlotTarget: target},
		MissingSlots: []string{},
	}
}

func classifyCameraControl(rawText string) *contracts.Intent {
	lower := strings.ToLower(rawText)
	if privacyOnPattern.MatchString(rawText) {
		return &contracts.Intent{
			Name:         IntentCameraControl,
			Domain:       DomainCamera,
			Confidence:   ConfidenceRuleMatched,
			Slots:        map[string]any{SlotPrivacy: PrivacyOn},
			MissingSlots: []string{},
		}
	}

	if privacyOffPattern.MatchString(rawText) {
		return &contracts.Intent{
			Name:         IntentCameraControl,
			Domain:       DomainCamera,
			Confidence:   ConfidenceRuleMatched,
			Slots:        map[string]any{SlotPrivacy: PrivacyOff},
			MissingSlots: []string{},
		}
	}

	if match := cameraSpeakPattern.FindStringSubmatch(rawText); len(match) > 1 {
		message := strings.TrimSpace(match[1])
		return &contracts.Intent{
			Name:         IntentCameraControl,
			Domain:       DomainCamera,
			Confidence:   ConfidenceRuleMatched,
			Slots:        map[string]any{SlotSpeak: message},
			MissingSlots: []string{},
		}
	}

	if strings.Contains(lower, "camera") || strings.Contains(lower, "cam") {
		if strings.Contains(lower, "trái") || strings.Contains(lower, "left") {
			return &contracts.Intent{
				Name:         IntentCameraControl,
				Domain:       DomainCamera,
				Confidence:   ConfidenceRuleMatched,
				Slots:        map[string]any{SlotDirection: DirectionLeft},
				MissingSlots: []string{},
			}
		}

		if strings.Contains(lower, "phải") || strings.Contains(lower, "right") {
			return &contracts.Intent{
				Name:         IntentCameraControl,
				Domain:       DomainCamera,
				Confidence:   ConfidenceRuleMatched,
				Slots:        map[string]any{SlotDirection: DirectionRight},
				MissingSlots: []string{},
			}
		}

		if strings.Contains(lower, "lên") || strings.Contains(lower, "up") {
			return &contracts.Intent{
				Name:         IntentCameraControl,
				Domain:       DomainCamera,
				Confidence:   ConfidenceRuleMatched,
				Slots:        map[string]any{SlotDirection: DirectionUp},
				MissingSlots: []string{},
			}
		}

		if strings.Contains(lower, "xuống") || strings.Contains(lower, "down") {
			return &contracts.Intent{
				Name:         IntentCameraControl,
				Domain:       DomainCamera,
				Confidence:   ConfidenceRuleMatched,
				Slots:        map[string]any{SlotDirection: DirectionDown},
				MissingSlots: []string{},
			}
		}

		if match := cameraViewPattern.FindStringSubmatch(rawText); len(match) > 0 {
			slots := map[string]any{SlotAction: ActionView}
			if len(match) > 1 && strings.TrimSpace(match[1]) != "" {
				slots[SlotTargetLabel] = strings.TrimSpace(match[1])
			}
			return &contracts.Intent{
				Name:         IntentCameraControl,
				Domain:       DomainCamera,
				Confidence:   ConfidenceRuleMatched,
				Slots:        slots,
				MissingSlots: []string{},
			}
		}
	}

	return nil
}
