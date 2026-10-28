package llm

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func (m *Manager) StreamChat(prompt string, onChunk func(string)) {
	_ = m.StreamChatContext(context.Background(), prompt, onChunk)
}

func (m *Manager) StreamChatContext(
	ctx context.Context, prompt string, onChunk func(string),
) error {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	provider := m.providerSnapshot()
	engineID := provider.config.SelectedEngine
	var response strings.Builder
	collect := func(chunk string) { response.WriteString(chunk) }
	var err error
	switch engineID {
	case "local_qwen":
		err = provider.callLocalLLM(ctx, prompt, collect)
	case "agy":
		if m.agy != nil {
			err = m.agy.ask(ctx, conversationFrom(ctx), prompt, collect)
		} else {
			err = provider.callAgy(ctx, SystemPrompt+"\n\n"+prompt, collect)
		}
	case "codex":
		err = provider.callGenericCLI(ctx, provider.config.CodexPath,
			[]string{"exec", "--skip-git-repo-check", prompt}, collect)
	case "claude":
		err = provider.callGenericCLI(ctx, provider.config.ClaudePath,
			[]string{"-p", prompt, "--append-system-prompt", SystemPrompt}, collect)
	case "gemini":
		err = provider.callGemini(ctx, prompt, collect)
	case "openai_api", "anthropic_api":
		err = provider.callKeyAPI(ctx, engineID, prompt, collect)
	case "rule_fallback":
		onChunk("AI đang tắt. Hãy chọn engine trong Cài đặt để trò chuyện.")
		return nil
	default:
		return fmt.Errorf("engine chưa được chọn hoặc không hợp lệ")
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil || response.Len() == 0 {
		message := "engine %s không trả lời được; " +
			"kiểm tra kết nối, đăng nhập hoặc quota"
		return fmt.Errorf(message, engineID)
	}
	m.usage.record(engineID)
	onChunk(response.String())

	return nil
}
