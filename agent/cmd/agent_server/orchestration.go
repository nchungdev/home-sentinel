package main

import (
	"context"
	"fmt"
	"strings"

	camAdapter "github.com/nchungdev/home-sentinel/agent/pkg/adapters/camera"
	haAdapter "github.com/nchungdev/home-sentinel/agent/pkg/adapters/homeassistant"
	"github.com/nchungdev/home-sentinel/agent/pkg/contracts"
	"github.com/nchungdev/home-sentinel/agent/pkg/haclient"
	"github.com/nchungdev/home-sentinel/agent/pkg/llm"
	"github.com/nchungdev/home-sentinel/agent/pkg/mediabridge"
	"github.com/nchungdev/home-sentinel/agent/pkg/resolution"
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
		text, _ := step.Parameters["text"].(string)
		lower := strings.ToLower(text)

		if e.server != nil && (strings.Contains(lower, "câu chào") || strings.Contains(lower, "lời chào")) && (strings.Contains(lower, "tv") || strings.Contains(lower, "tivi")) {
			greeting := "Chúc cả nhà một buổi tối ấm áp và thư giãn bên nhau!"
			if idx := strings.Index(text, ":"); idx > 0 && idx < len(text)-2 {
				greeting = strings.TrimSpace(text[idx+1:])
			} else if e.llmManager != nil {
				var sb strings.Builder
				e.llmManager.StreamChat("Viết 1 câu chào dưới 15 từ gửi đến gia đình hiển thị trên màn hình TV, ấm áp, lịch sự, không để trong ngoặc kép.", func(chunk string) {
					sb.WriteString(chunk)
				})
				if g := strings.Trim(strings.TrimSpace(sb.String()), "\"'“”"); g != "" {
					greeting = g
				}
			}
			sent := e.server.BroadcastDuinchGreeting(greeting, "happy")
			reply := fmt.Sprintf("Đã gửi câu chào lên TV (%d thiết bị): %q", sent, greeting)
			return &contracts.ExecutionResult{
				StepID:   step.StepID,
				Status:   contracts.ActionStatusSucceeded,
				Detail:   &reply,
				CanRetry: false,
			}, nil
		}

		var sb strings.Builder
		e.llmManager.StreamChat(text, func(chunk string) {
			sb.WriteString(chunk)
		})
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

