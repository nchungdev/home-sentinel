package main

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	camAdapter "github.com/nchungdev/home-sentinel/internal/adapters/camera"
	haAdapter "github.com/nchungdev/home-sentinel/internal/adapters/homeassistant"
	"github.com/nchungdev/home-sentinel/internal/adapters/nas"
	"github.com/nchungdev/home-sentinel/internal/contracts"
	"github.com/nchungdev/home-sentinel/internal/integrations/haclient"
	"github.com/nchungdev/home-sentinel/internal/llm"
	"github.com/nchungdev/home-sentinel/internal/integrations/mediabridge"
	"github.com/nchungdev/home-sentinel/internal/resolution"
)

type serverPlexSearcher struct {
	bridge *mediabridge.Bridge
}

func (s *serverPlexSearcher) SearchFirst(
	ctx context.Context,
	query string,
) (*resolution.PlexMediaItem, error) {
	if s.bridge == nil {
		return nil, nil
	}
	results, err := s.bridge.PlexSearch(query, "")
	if err != nil || len(results) == 0 {
		return nil, err
	}
	first := results[0]
	return &resolution.PlexMediaItem{
		Title:     first.Title,
		Year:      first.Year,
		RatingKey: first.RatingKey,
		Type:      first.Type,
		Summary:   first.Summary,
		Thumb:     first.Thumb,
		DeepLink:  first.DeepLink,
	}, nil
}

func (s *serverPlexSearcher) GetRecent(
	ctx context.Context,
	limit int,
) ([]*resolution.PlexMediaItem, error) {
	if s.bridge == nil {
		return nil, nil
	}
	results, err := s.bridge.PlexRecent("", limit)
	if err != nil {
		return nil, err
	}
	var items []*resolution.PlexMediaItem
	for _, res := range results {
		items = append(items, &resolution.PlexMediaItem{
			Title:     res.Title,
			Year:      res.Year,
			RatingKey: res.RatingKey,
			Type:      res.Type,
			Summary:   res.Summary,
			Thumb:     res.Thumb,
			DeepLink:  res.DeepLink,
		})
	}
	return items, nil
}

type serverHAResolver struct {
	haClient *haclient.Client
}

func (s *serverHAResolver) ResolveEntity(
	ctx context.Context,
	targetLabel string,
	action string,
) (*resolution.HAEntityMatch, error) {
	if s.haClient == nil {
		return nil, nil
	}
	states, err := s.haClient.GetStates()
	if err != nil {
		return nil, err
	}
	targetNormalized := strings.ToLower(strings.TrimSpace(targetLabel))
	words := strings.Fields(targetNormalized)
	var bestMatch *resolution.HAEntityMatch
	var maxScore int

	for _, st := range states {
		fn, _ := st.Attributes["friendly_name"].(string)
		fnLower := strings.ToLower(fn)
		eidLower := strings.ToLower(st.EntityID)

		if strings.Contains(fnLower, targetNormalized) || strings.Contains(eidLower, targetNormalized) {
			return makeHAMatch(st.EntityID, fn, action), nil
		}

		score := 0
		for _, w := range words {
			if len(w) > 1 && strings.Contains(fnLower, w) {
				score++
			}
		}
		if score > maxScore && score >= 2 {
			maxScore = score
			bestMatch = makeHAMatch(st.EntityID, fn, action)
		}
	}
	if bestMatch != nil {
		return bestMatch, nil
	}
	return nil, nil
}

func makeHAMatch(entityID, fn, action string) *resolution.HAEntityMatch {
	parts := strings.Split(entityID, ".")
	domain := parts[0]
	service := "turn_on"
	if action == "off" || action == "tắt" {
		service = "turn_off"
	}
	return &resolution.HAEntityMatch{
		EntityID: entityID,
		Domain:   domain,
		Service:  service,
		Name:     fn,
	}
}

type serverExecutionEngine struct {
	server        *Server
	haAdapter     *haAdapter.Adapter
	cameraAdapter *camAdapter.Adapter
	nasAdapter    *nas.Adapter
	llmManager    *llm.Manager
	haClient      *haclient.Client
}

func (e *serverExecutionEngine) Execute(
	ctx context.Context,
	step *contracts.ActionStep,
	reqCtx *contracts.RequestContext,
) (*contracts.ExecutionResult, error) {
	if step == nil {
		return nil, contracts.ErrInvalidActionStep
	}

	if step.RunsOn == "tv_client" {
		return &contracts.ExecutionResult{
			StepID:   step.StepID,
			Status:   contracts.ActionStatusSucceeded,
			CanRetry: false,
		}, nil
	}

	if step.ActionType == "llm.chat" || step.TargetID == "llm" {
		if reqCtx != nil {
			ctx = llm.WithConversation(ctx, reqCtx.ClientID)
		}
		text, _ := step.Parameters["text"].(string)
		lower := strings.ToLower(text)

		tvPattern := regexp.MustCompile(
			`(?i)(?:nói\s+gì\s+đó|gửi\s+(?:tin\s+nhắn|lời\s+chào|câu\s+chào|thông\s+báo)|chào|nhắn|bảo)\s+.*(?:với|lên|qua|cho|tới)\s+(?:tv|tivi|màn\s+hình)|` +
				`(?i)(?:gửi\s+tin\s+nhắn\s+qua\s+socket|nhắn\s+lên\s+tv|chào\s+tv|nói\s+với\s+tv|lời\s+chào.*tv|câu\s+chào.*tv)`,
		)

		if e.server != nil && tvPattern.MatchString(lower) {
			msg, isExplicit := extractTVMessage(text)
			if !isExplicit && e.llmManager != nil {
				prompt := fmt.Sprintf(
					"Bạn là trợ lý AI Duinch trên TV phòng khách. Người dùng yêu cầu hiển thị một lời chào hoặc thông điệp lên TV: %q.\n"+
						"Hãy soạn 1 câu ngắn gọn, tự nhiên, tươi vui gửi đến người xem TV (tối đa 15 từ). "+
						"Chỉ trả lời bằng đúng nội dung câu, không kèm giải thích hay dấu ngoặc kép.",
					text,
				)
				var sb strings.Builder
				err := e.llmManager.StreamChatContext(
					ctx, prompt, func(chunk string) { sb.WriteString(chunk) },
				)
				if err != nil {
					return nil, err
				}
				if g := strings.Trim(strings.TrimSpace(sb.String()), "\"'“”"); g != "" {
					msg = g
				}
			}
			if msg == "" {
				msg = "Xin chào gia đình, chúc cả nhà một ngày thật vui vẻ!"
			}

			sent := e.server.BroadcastDuinchGreeting(msg, "happy")
			e.server.BroadcastDuinchNotification(&DuinchNotificationPayload{
				Source:   "user",
				Title:    "Tin nhắn",
				Message:  msg,
				Severity: "info",
			})

			actionLabel := "tin nhắn"
			if !isExplicit {
				actionLabel = "câu chào"
			}
			reply := fmt.Sprintf("Đã gửi %s lên TV (%d thiết bị): %q", actionLabel, sent, msg)
			return &contracts.ExecutionResult{
				StepID:   step.StepID,
				Status:   contracts.ActionStatusSucceeded,
				Detail:   &reply,
				CanRetry: false,
			}, nil
		}

		prompt := text
		if !reqCtx.RunsTVActions() {
			prompt = chatClientNote + text
		}
		var sb strings.Builder
		err := e.llmManager.StreamChatContext(ctx, prompt, func(chunk string) {
			sb.WriteString(chunk)
		})
		if err != nil {
			return nil, err
		}
		reply := sb.String()
		if reply == "" {
			reply = "Hiện tại trợ lý AI chưa phản hồi được. Bạn vui lòng thử lại sau giây lát nhé."
		}
		return &contracts.ExecutionResult{
			StepID:   step.StepID,
			Status:   contracts.ActionStatusSucceeded,
			Detail:   &reply,
			CanRetry: false,
		}, nil
	}

	if step.ActionType == resolution.ActionTypeSystemStatus && e.nasAdapter != nil {
		return e.nasAdapter.Execute(ctx, step)
	}

	if strings.HasPrefix(step.ActionType, "homeassistant.") ||
		strings.HasPrefix(step.ActionType, "ha.") {
		if e.haAdapter != nil {
			return e.haAdapter.Execute(ctx, step)
		}
	}

	if strings.HasPrefix(step.ActionType, "camera.") ||
		strings.HasPrefix(step.ActionType, "tapo.") {
		if e.cameraAdapter != nil {
			return e.cameraAdapter.Execute(ctx, step)
		}
	}

	return &contracts.ExecutionResult{
		StepID:   step.StepID,
		Status:   contracts.ActionStatusSucceeded,
		CanRetry: false,
	}, nil
}
