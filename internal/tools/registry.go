package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
)

type Tool struct {
	Name                 string
	Description          string
	InputSchema          map[string]any
	RequiresConfirmation bool
	Summarize            func(args map[string]any) string
	Run                  func(ctx context.Context, call Call) (string, error)
}

type Call struct {
	ConversationID string
	Args           map[string]any
}

type Registry struct {
	mutex         sync.RWMutex
	tools         map[string]Tool
	confirmations *confirmationStore
}

func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool), confirmations: newConfirmationStore()}
}

func (r *Registry) Register(tool Tool) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if tool.RequiresConfirmation {
		tool.InputSchema = withConfirmationField(tool.InputSchema)
	}
	r.tools[tool.Name] = tool
}

func (r *Registry) List() []Tool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	list := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		list = append(list, tool)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return list
}

func (r *Registry) BeginTurn(conversationID, userText string) {
	r.confirmations.beginTurn(conversationID, userText)
}

func (r *Registry) Call(
	ctx context.Context, conversationID, name string, rawArgs json.RawMessage,
) (string, error) {
	r.mutex.RLock()
	tool, found := r.tools[name]
	r.mutex.RUnlock()
	if !found {
		return "", fmt.Errorf("không có công cụ %q", name)
	}

	args := map[string]any{}
	if len(rawArgs) > 0 {
		if err := json.Unmarshal(rawArgs, &args); err != nil {
			return "", fmt.Errorf("tham số không hợp lệ: %w", err)
		}
	}

	if tool.RequiresConfirmation {
		confirmationID, _ := args[confirmationField].(string)
		delete(args, confirmationField)
		if confirmationID == "" {
			return r.confirmations.request(conversationID, tool, args), nil
		}
		if err := r.confirmations.consume(conversationID, confirmationID, tool.Name, args); err != nil {
			return "", err
		}
	}
	return tool.Run(ctx, Call{ConversationID: conversationID, Args: args})
}

func withConfirmationField(schema map[string]any) map[string]any {
	copied := map[string]any{"type": "object"}
	for key, value := range schema {
		copied[key] = value
	}
	properties := map[string]any{}
	if existing, ok := copied["properties"].(map[string]any); ok {
		for key, value := range existing {
			properties[key] = value
		}
	}
	properties[confirmationField] = map[string]any{
		"type": "string",
		"description": "Mã xác nhận nhận được ở lần gọi trước. " +
			"Chỉ truyền sau khi người dùng đã đồng ý ở lượt nói tiếp theo.",
	}
	copied["properties"] = properties
	return copied
}
