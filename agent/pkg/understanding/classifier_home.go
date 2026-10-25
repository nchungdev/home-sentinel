package understanding

import (
	"strconv"
	"strings"

	"github.com/nchungdev/home-sentinel/agent/pkg/contracts"
)

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
