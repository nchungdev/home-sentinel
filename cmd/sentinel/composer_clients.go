package main

import (
	"fmt"
	"regexp"

	"github.com/nchungdev/home-sentinel/internal/contracts"
)

var mentionsTVPattern = regexp.MustCompile(
	`(?i)(?:^|[^\p{L}\p{N}])(?:tv|tivi|ti\s+vi|màn\s+hình)(?:$|[^\p{L}\p{N}])`,
)

const chatClientNote = "[Người dùng đang nhắn từ trang web hoặc điện thoại, không phải từ TV.]\n"

func routeClientActions(
	server *Server,
	input *contracts.InputEvent,
	reqCtx *contracts.RequestContext,
	plan *contracts.ActionPlan,
	actions []map[string]any,
) ([]map[string]any, string) {
	if len(actions) == 0 || reqCtx.RunsTVActions() {
		return actions, ""
	}
	if input != nil && mentionsTVPattern.MatchString(input.Text) && server != nil {
		sent := 0
		for _, action := range actions {
			sent += broadcastAction(server, action)
		}
		if sent == 0 {
			return nil, "Không có TV nào đang kết nối nên chưa mở được trên TV."
		}
		return nil, fmt.Sprintf("Đã gửi lệnh mở lên TV (%d TV).", sent)
	}
	return nil, describeForChat(actions[0], libraryErrorNote(plan))
}

func libraryErrorNote(plan *contracts.ActionPlan) string {
	if plan == nil {
		return ""
	}
	for _, key := range []string{"library_error", "error"} {
		if message, _ := plan.Evidence[key].(string); message != "" {
			return fmt.Sprintf("Chưa tra được thư viện Plex (%s).", message)
		}
	}
	return ""
}

func broadcastAction(server *Server, action map[string]any) int {
	actionType, _ := action["action_type"].(string)
	stepID, _ := action["step_id"].(string)
	targetID, _ := action["target_id"].(string)
	params, _ := action["parameters"].(map[string]any)
	return server.BroadcastDuinchAction(stepID, actionType, targetID, "", params)
}

func describeForChat(action map[string]any, libraryNote string) string {
	params, _ := action["parameters"].(map[string]any)
	title, _ := params["title"].(string)
	query, _ := params["query"].(string)
	switch action["action_type"] {
	case "plex.open":
		return fmt.Sprintf("Thư viện Plex có “%s”. Nói “mở %s trên TV” để xem trên TV.",
			title, title)
	case "youtube.launch":
		lookup := fmt.Sprintf("Không thấy “%s” trong thư viện Plex.", query)
		if libraryNote != "" {
			lookup = libraryNote
		}
		return lookup + fmt.Sprintf(" Nói “mở %s trên TV” nếu muốn tìm trên YouTube ở TV.", query)
	}
	return "Yêu cầu này cần mở trên TV. Hãy nói thêm “trên TV” để gửi lệnh lên TV."
}
