package llm

import (
	"context"
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
	order := provider.config.PriorityOrder

	for _, engineID := range order {
		if err := ctx.Err(); err != nil {
			return err
		}
		var response strings.Builder
		collect := func(chunk string) { response.WriteString(chunk) }
		var err error
		switch engineID {
		case "local_qwen":
			err = provider.callLocalLLM(ctx, prompt, collect)
		case "agy":
			err = provider.callGenericCLI(ctx, provider.config.AgyPath,
				[]string{"-p", prompt, "--append-system-prompt", SystemPrompt}, collect)
		case "codex":
			err = provider.callGenericCLI(ctx, provider.config.CodexPath, []string{"exec", prompt}, collect)
		case "claude":
			err = provider.callGenericCLI(ctx, provider.config.ClaudePath,
				[]string{"-p", prompt, "--append-system-prompt", SystemPrompt}, collect)
		case "gemini":
			err = provider.callGemini(ctx, prompt, collect)
		case "rule_fallback":
			onChunk("Hiện tại AI chưa khả dụng. Bạn vui lòng thử lại sau.")
			return nil
		default:
			continue
		}

		if err == nil && response.Len() > 0 {
			onChunk(response.String())
			return nil
		}
	}

	if err := ctx.Err(); err != nil {
		return err
	}
	message := "Tất cả các mô hình AI hiện không khả dụng. " +
		"Bạn vui lòng kiểm tra lại cấu hình!"
	onChunk(message)
	return nil
}
