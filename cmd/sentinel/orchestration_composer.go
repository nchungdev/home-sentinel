package main

import (
	"context"
	"fmt"
	"strings"

	camAdapter "github.com/nchungdev/home-sentinel/internal/adapters/camera"
	haAdapter "github.com/nchungdev/home-sentinel/internal/adapters/homeassistant"
	"github.com/nchungdev/home-sentinel/internal/contracts"
	"github.com/nchungdev/home-sentinel/internal/orchestrator"
	"github.com/nchungdev/home-sentinel/internal/resolution"
	"github.com/nchungdev/home-sentinel/internal/understanding"
)

type serverPresentationComposer struct {
	server *Server
}

func (c *serverPresentationComposer) Compose(
	ctx context.Context,
	input *contracts.InputEvent,
	intent *contracts.Intent,
	plan *contracts.ActionPlan,
	results []*contracts.ExecutionResult,
	reqCtx *contracts.RequestContext,
) (*contracts.PresentationResponse, error) {
	if plan != nil {
		if report, ok := plan.Evidence["media_search"].(*contracts.MediaSearchResponse); ok {
			return contracts.MediaPresentation(report, true), nil
		}
	}
	speech := ""
	var clientActions []map[string]any

	if plan != nil {
		for _, step := range plan.Steps {
			if step.RunsOn == "tv_client" {
				clientActions = append(clientActions, map[string]any{
					"step_id":       step.StepID,
					"action_type":   step.ActionType,
					"target_id":     step.TargetID,
					"parameters":    step.Parameters,
					"runs_on":       step.RunsOn,
					"timeout_ms":    step.TimeoutMs,
					"is_idempotent": step.IsIdempotent,
				})
			}
		}
	}

	clientActions, chatSpeech := routeClientActions(c.server, input, reqCtx, plan, clientActions)

	for _, res := range results {
		if res.Detail != nil && *res.Detail != "" {
			speech = *res.Detail
			break
		}
	}

	if note := libraryErrorNote(plan); speech == "" && note != "" && chatSpeech == "" {
		speech = note
		if len(clientActions) > 0 {
			speech += " Đang mở YouTube trên TV."
		}
	}

	if speech == "" && plan != nil && plan.Evidence != nil {
		if isCat, _ := plan.Evidence["is_catalog"].(bool); isCat {
			if titles, ok := plan.Evidence["titles"].([]string); ok && len(titles) > 0 {
				speech = fmt.Sprintf(
					"Trên NAS hiện có các phim: %s. Bạn muốn mở phim nào?",
					strings.Join(titles, ", "),
				)
			} else {
				speech = "Hiện tại không tìm thấy phim nào trong thư viện Plex trên NAS."
			}
		} else if notFound, _ := plan.Evidence["not_found"].(bool); notFound {
			q, _ := plan.Evidence["query"].(string)
			if q != "" {
				speech = fmt.Sprintf("Không tìm thấy phim %q trên Plex/NAS.", q)
			} else {
				speech = "Không tìm thấy phim yêu cầu trên Plex/NAS."
			}
		}
	}

	if speech == "" {
		if intent != nil && intent.Name == "system.status" {
			speech = "Hệ thống Home Sentinel đang hoạt động bình thường."
		} else if intent != nil && intent.Domain == "home" {
			speech = "Đã thực hiện xong lệnh điều khiển nhà thông minh."
		} else if chatSpeech != "" {
			speech = chatSpeech
		} else if len(clientActions) > 0 {
			speech = "Đang mở trên màn hình TV..."
		} else {
			speech = "Tôi chưa hiểu rõ yêu cầu hoặc chưa xử lý được lệnh này. Bạn vui lòng thử lại nhé."
		}
	}

	if len(clientActions) > 0 && c.server != nil && reqCtx.RunsTVActions() {
		for _, action := range clientActions {
			actionType, _ := action["action_type"].(string)
			stepID, _ := action["step_id"].(string)
			targetID, _ := action["target_id"].(string)
			params, _ := action["parameters"].(map[string]any)
			c.server.BroadcastDuinchAction(stepID, actionType, targetID, speech, params)
		}
	}

	return &contracts.PresentationResponse{
		SpeechText:    speech,
		ClientActions: clientActions,
		Mood:          "idle",
		Handler:       c.handler(intent),
	}, nil
}

func buildServerOrchestrator(server *Server) *orchestrator.Orchestrator {
	understandingEngine := understanding.NewEngine()
	haResolver := &serverHAResolver{haClient: server.HAClient}
	plexSearcher := &serverPlexSearcher{bridge: server.MediaBridge}
	resolver := resolution.NewResolver(plexSearcher, haResolver)

	haExecAdapter := haAdapter.NewAdapter(server.HAClient, haAdapter.Config{})
	camExecAdapter := camAdapter.NewAdapter(nil, camAdapter.Config{})

	executionEngine := &serverExecutionEngine{
		server:        server,
		haAdapter:     haExecAdapter,
		cameraAdapter: camExecAdapter,
		nasAdapter:    server.NAS,
		llmManager:    server.LLMManager,
		haClient:      server.HAClient,
	}

	composer := &serverPresentationComposer{server: server}

	return orchestrator.NewOrchestrator(
		understandingEngine,
		resolver,
		executionEngine,
		composer,
	)
}

func (c *serverPresentationComposer) handler(intent *contracts.Intent) *contracts.Handler {
	if intent == nil {
		return nil
	}
	handler := &contracts.Handler{Route: "rule", Intent: intent.Name}
	if intent.Name == understanding.IntentGeneralChat {
		handler.Route = "ai"
		if c.server != nil && c.server.LLMManager != nil {
			handler.Engine = c.server.LLMManager.Settings().SelectedEngine
		}
	}
	return handler
}
