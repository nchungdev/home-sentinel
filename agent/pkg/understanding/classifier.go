package understanding

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/nchungdev/home-sentinel/agent/pkg/contracts"
)

var (
	systemStatusPattern = regexp.MustCompile(
		`(?i)(trạng\s+thái|thông\s+tin|kiểm\s+tra|dung\s+lượng\s+ổ\s+cứng|` +
			`ổ\s+cứng|status)\s+(nas|docker|hệ\s+thống|system)|` +
			`(?i)(nas|docker)\s+(status|trạng\s+thái)`,
	)

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
	if strings.Contains(lower, "docker") {
		target = "docker"
	} else if strings.Contains(lower, "nas") {
		target = "nas"
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

func classifyHomeControl(rawText string) *contracts.Intent {
	if match := temperaturePattern.FindStringSubmatch(rawText); len(match) > 2 {
		rawNumber := strings.ReplaceAll(match[2], ",", ".")
		if tempValue, err := strconv.ParseFloat(rawNumber, 64); err == nil {
			targetLabel := strings.TrimSpace(match[1])
			return &contracts.Intent{
				Name:       IntentHomeControl,
				Domain:     DomainHome,
				Confidence: ConfidenceRuleMatched,
				Slots: map[string]any{
					SlotAction:      ActionSetTemperature,
					SlotTargetLabel: targetLabel,
					SlotTemperature: tempValue,
				},
				MissingSlots: []string{},
			}
		}
	}

	if match := homeCurtainPattern.FindStringSubmatch(rawText); len(match) > 2 {
		action := ActionOpen
		if strings.EqualFold(match[1], "đóng") || strings.Contains(rawText, "xuống") {
			action = ActionClose
		}
		targetLabel := strings.TrimSpace(match[2])
		return &contracts.Intent{
			Name:       IntentHomeControl,
			Domain:     DomainHome,
			Confidence: ConfidenceRuleMatched,
			Slots: map[string]any{
				SlotAction:      action,
				SlotTargetLabel: targetLabel,
			},
			MissingSlots: []string{},
		}
	}

	if match := homeDoorPattern.FindStringSubmatch(rawText); len(match) > 2 {
		action := ActionOpen
		switch strings.ToLower(match[1]) {
		case "khóa":
			action = ActionLock
		case "mở khóa":
			action = ActionUnlock
		case "đóng":
			action = ActionClose
		}
		targetLabel := strings.TrimSpace(match[2])
		return &contracts.Intent{
			Name:       IntentHomeControl,
			Domain:     DomainHome,
			Confidence: ConfidenceRuleMatched,
			Slots: map[string]any{
				SlotAction:      action,
				SlotTargetLabel: targetLabel,
			},
			MissingSlots: []string{},
		}
	}

	if match := homeSwitchPattern.FindStringSubmatch(rawText); len(match) > 2 {
		action := ActionTurnOn
		lowerAction := strings.ToLower(match[1])
		if lowerAction == "tắt" || lowerAction == "đóng" || lowerAction == "ngắt" {
			action = ActionTurnOff
		}
		targetLabel := strings.TrimSpace(match[2])
		return &contracts.Intent{
			Name:       IntentHomeControl,
			Domain:     DomainHome,
			Confidence: ConfidenceRuleMatched,
			Slots: map[string]any{
				SlotAction:      action,
				SlotTargetLabel: targetLabel,
			},
			MissingSlots: []string{},
		}
	}

	return nil
}

func classifyMediaSearch(rawText string, explicitSource string) *contracts.Intent {
	var kind string
	var query string
	matched := false

	if match := mediaVerbPattern.FindStringSubmatch(rawText); len(match) > 2 {
		tail := strings.TrimSpace(match[2])
		if kindMatch := mediaKindPattern.FindStringSubmatch(tail); len(kindMatch) > 2 {
			kind = strings.ToLower(strings.TrimSpace(kindMatch[1]))
			query = strings.TrimSpace(kindMatch[2])
		} else {
			query = tail
		}
		matched = true
	} else if kindMatch := mediaKindPattern.FindStringSubmatch(rawText); len(kindMatch) > 2 {
		kind = strings.ToLower(strings.TrimSpace(kindMatch[1]))
		query = strings.TrimSpace(kindMatch[2])
		matched = true
	} else if explicitSource != "" {
		query = strings.TrimSpace(rawText)
		matched = true
	}

	if !matched {
		return nil
	}

	slots := make(map[string]any)
	missingSlots := make([]string, 0)
	confidence := ConfidenceRuleMatched

	if query != "" {
		slots[SlotQuery] = query
	} else {
		missingSlots = append(missingSlots, SlotQuery)
		confidence = ConfidenceFallback
	}

	if kind != "" {
		slots[SlotKind] = kind
	}

	return &contracts.Intent{
		Name:           IntentMediaSearch,
		Domain:         DomainMedia,
		Confidence:     confidence,
		Slots:          slots,
		ExplicitSource: explicitSource,
		MissingSlots:   missingSlots,
	}
}

func classifyGeneralChat(rawText string) *contracts.Intent {
	return &contracts.Intent{
		Name:       IntentGeneralChat,
		Domain:     DomainGeneral,
		Confidence: ConfidenceFallback,
		Slots: map[string]any{
			SlotText:  rawText,
			SlotQuery: rawText,
		},
		MissingSlots: []string{},
	}
}
