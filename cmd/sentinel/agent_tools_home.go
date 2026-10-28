package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	haAdapter "github.com/nchungdev/home-sentinel/internal/adapters/homeassistant"
	"github.com/nchungdev/home-sentinel/internal/contracts"
	"github.com/nchungdev/home-sentinel/internal/integrations/haclient"
	"github.com/nchungdev/home-sentinel/internal/tools"
)

var controllableDomains = map[string]bool{
	"light": true, "switch": true, "fan": true, "climate": true, "cover": true,
}

type entitySummary struct {
	EntityID string `json:"entity_id"`
	Name     string `json:"name"`
	State    string `json:"state"`
}

func registerHomeTools(registry *tools.Registry, server *Server) {
	adapter := haAdapter.NewAdapter(server.HAClient, haAdapter.Config{})
	registry.Register(tools.Tool{
		Name:        "ha_list_entities",
		Description: "Liệt kê thiết bị Home Assistant điều khiển được (đèn, công tắc, quạt, điều hòa, rèm).",
		InputSchema: schema(map[string]any{
			"search": stringField("Lọc theo tên hoặc entity_id, ví dụ 'phòng khách'"),
		}),
		Run: func(ctx context.Context, call tools.Call) (string, error) {
			states, err := server.HAClient.GetStates()
			if err != nil {
				return "", fmt.Errorf("không kết nối được Home Assistant: %w", err)
			}
			return jsonText(filterEntities(states, stringArg(call, "search")))
		},
	})
	registry.Register(tools.Tool{
		Name: "ha_control",
		Description: "Điều khiển một thiết bị Home Assistant. service: turn_on, turn_off, toggle; " +
			"điều hòa có thể kèm temperature.",
		InputSchema: schema(map[string]any{
			"entity_id":   stringField("entity_id từ ha_list_entities"),
			"service":     stringField("turn_on, turn_off hoặc toggle"),
			"temperature": map[string]any{"type": "number"},
		}, "entity_id", "service"),
		Run: func(ctx context.Context, call tools.Call) (string, error) {
			parameters := map[string]any{"service": stringArg(call, "service")}
			if temperature, ok := call.Args["temperature"].(float64); ok {
				parameters["temperature"] = temperature
			}
			result, err := adapter.Execute(ctx, &contracts.ActionStep{
				StepID: toolStepID(), ActionType: "ha.service_call",
				TargetID: stringArg(call, "entity_id"), Parameters: parameters, IsIdempotent: true,
			})
			return executionText(result, err)
		},
	})
}

func filterEntities(states []haclient.EntityState, search string) []entitySummary {
	search = strings.ToLower(strings.TrimSpace(search))
	var matches []entitySummary
	for _, state := range states {
		domain, _, _ := strings.Cut(state.EntityID, ".")
		if !controllableDomains[domain] {
			continue
		}
		name, _ := state.Attributes["friendly_name"].(string)
		haystack := strings.ToLower(state.EntityID + " " + name)
		if search != "" && !strings.Contains(haystack, search) {
			continue
		}
		matches = append(matches, entitySummary{state.EntityID, name, state.State})
	}
	return matches
}

func registerCameraTools(registry *tools.Registry, server *Server) {
	registry.Register(tools.Tool{
		Name:                 "camera_ptz",
		Description:          "Xoay camera theo hướng (left, right, up, down) hoặc tới preset.",
		RequiresConfirmation: true,
		InputSchema: schema(map[string]any{
			"direction": stringField("left, right, up hoặc down"),
			"preset":    stringField("Tên hoặc số preset, thay cho direction"),
		}),
		Summarize: func(args map[string]any) string {
			return fmt.Sprintf("xoay camera %v%v", args["direction"], args["preset"])
		},
		Run: func(ctx context.Context, call tools.Call) (string, error) {
			return invokeHandler(server.handleCameraPTZ, call.Args)
		},
	})
	registry.Register(tools.Tool{
		Name:                 "camera_speak",
		Description:          "Phát một câu nói qua loa của camera.",
		RequiresConfirmation: true,
		InputSchema:          schema(map[string]any{"text": stringField("Câu cần phát")}, "text"),
		Summarize: func(args map[string]any) string {
			return fmt.Sprintf("phát qua loa camera: %q", args["text"])
		},
		Run: func(ctx context.Context, call tools.Call) (string, error) {
			return invokeHandler(server.handleCameraSpeak, call.Args)
		},
	})
}

func invokeHandler(handler http.HandlerFunc, body map[string]any) (string, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(payload))
	handler(recorder, request)

	var reply struct {
		OK      bool   `json:"ok"`
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &reply); err != nil {
		return "", fmt.Errorf("camera trả về dữ liệu không hợp lệ")
	}
	if !reply.OK {
		return "", fmt.Errorf("camera báo lỗi: %s", reply.Error)
	}
	return "Camera đã thực hiện: " + reply.Message, nil
}

func executionText(result *contracts.ExecutionResult, err error) (string, error) {
	if err != nil {
		return "", err
	}
	detail := ""
	if result.Detail != nil {
		detail = *result.Detail
	}
	if result.Status != contracts.ActionStatusSucceeded {
		return "", fmt.Errorf("%s", detail)
	}
	return detail, nil
}
