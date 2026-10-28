package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nchungdev/home-sentinel/internal/contracts"
	"github.com/nchungdev/home-sentinel/internal/tools"
)

const maxToolResults = 8

func newAgentTools(server *Server) *tools.Registry {
	registry := tools.NewRegistry()
	registerMediaTools(registry, server)
	registerTVTools(registry, server)
	registerHomeTools(registry, server)
	registerCameraTools(registry, server)
	return registry
}

func schema(properties map[string]any, required ...string) map[string]any {
	return map[string]any{"type": "object", "properties": properties, "required": required}
}

func stringField(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func stringArg(call tools.Call, name string) string {
	value, _ := call.Args[name].(string)
	return value
}

func jsonText(value any) (string, error) {
	data, err := json.Marshal(value)
	return string(data), err
}

func registerMediaTools(registry *tools.Registry, server *Server) {
	registry.Register(tools.Tool{
		Name:        "nas_status",
		Description: "Trạng thái NAS: dung lượng ổ đĩa, container Docker, tải hệ thống.",
		InputSchema: schema(map[string]any{"target": map[string]any{
			"type": "string", "enum": []string{"system", "disk", "docker"},
		}}),
		Run: func(ctx context.Context, call tools.Call) (string, error) {
			target := stringArg(call, "target")
			if target == "" {
				target = "system"
			}
			result, err := server.NAS.Execute(ctx, &contracts.ActionStep{
				StepID: "tool", ActionType: "system.status", TargetID: target,
			})
			if err != nil || result.Detail == nil {
				return "", fmt.Errorf("không đọc được trạng thái NAS: %v", err)
			}
			return *result.Detail, nil
		},
	})
	registry.Register(tools.Tool{
		Name: "plex_search",
		Description: "Tìm phim, series trong thư viện Plex trên NAS. " +
			"Trả về rating_key và deep_link để mở trên TV.",
		InputSchema: schema(map[string]any{"query": stringField("Tên phim hoặc từ khóa")}, "query"),
		Run: func(ctx context.Context, call tools.Call) (string, error) {
			results, err := server.MediaBridge.PlexSearch(stringArg(call, "query"), "")
			if err != nil {
				return "", fmt.Errorf("không tìm được trên Plex: %w", err)
			}
			if len(results) > maxToolResults {
				results = results[:maxToolResults]
			}
			return jsonText(results)
		},
	})
	registry.Register(tools.Tool{
		Name:        "plex_recent",
		Description: "Phim, tập mới được thêm vào thư viện Plex gần đây.",
		InputSchema: schema(map[string]any{"limit": map[string]any{"type": "integer"}}),
		Run: func(ctx context.Context, call tools.Call) (string, error) {
			limit := maxToolResults
			if value, ok := call.Args["limit"].(float64); ok && value > 0 && value < 30 {
				limit = int(value)
			}
			results, err := server.MediaBridge.PlexRecent("", limit)
			if err != nil {
				return "", fmt.Errorf("không đọc được Plex: %w", err)
			}
			return jsonText(results)
		},
	})
}

func registerTVTools(registry *tools.Registry, server *Server) {
	registry.Register(tools.Tool{
		Name:        "tv_open_plex",
		Description: "Mở một phim Plex trên TV. Lấy rating_key và deep_link từ plex_search.",
		InputSchema: schema(map[string]any{
			"rating_key": stringField("rating_key từ plex_search"),
			"deep_link":  stringField("deep_link từ plex_search"),
			"title":      stringField("Tên phim để thông báo"),
		}, "rating_key", "deep_link"),
		Run: func(ctx context.Context, call tools.Call) (string, error) {
			title := stringArg(call, "title")
			sent := server.BroadcastDuinchAction(toolStepID(), "plex.open",
				stringArg(call, "rating_key"), "Đang mở "+title,
				map[string]any{"deep_link": stringArg(call, "deep_link"), "title": title})
			return tvDelivery(sent, "Đã gửi lệnh mở "+title+" lên TV")
		},
	})
	registry.Register(tools.Tool{
		Name:        "tv_open_youtube",
		Description: "Mở YouTube trên TV và tìm theo từ khóa.",
		InputSchema: schema(map[string]any{"query": stringField("Từ khóa tìm trên YouTube")}, "query"),
		Run: func(ctx context.Context, call tools.Call) (string, error) {
			query := stringArg(call, "query")
			sent := server.BroadcastDuinchAction(toolStepID(), "youtube.launch", "youtube",
				"Đang mở YouTube", map[string]any{"query": query})
			return tvDelivery(sent, "Đã gửi lệnh mở YouTube với từ khóa "+query)
		},
	})
	registry.Register(tools.Tool{
		Name:        "tv_notify",
		Description: "Hiển thị một thông báo ngắn trên màn hình TV.",
		InputSchema: schema(map[string]any{"message": stringField("Nội dung thông báo")}, "message"),
		Run: func(ctx context.Context, call tools.Call) (string, error) {
			sent := server.BroadcastDuinchNotification(&DuinchNotificationPayload{
				Source: "agent", Title: "Tin nhắn", Message: stringArg(call, "message"),
				Severity: "info",
			})
			return tvDelivery(sent, "Đã hiển thị thông báo trên TV")
		},
	})
}

func toolStepID() string {
	return fmt.Sprintf("tool-%d", time.Now().UnixNano())
}

func tvDelivery(sent int, success string) (string, error) {
	if sent == 0 {
		return "", fmt.Errorf("không có TV nào đang kết nối với Sentinel")
	}
	return fmt.Sprintf("%s (%d TV).", success, sent), nil
}
