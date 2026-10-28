package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func (m *Manager) callKeyAPI(
	ctx context.Context, engineID, prompt string, onChunk func(string),
) error {
	endpoint := "https://api.openai.com/v1/chat/completions"
	key, model := m.config.OpenAIKey, m.config.OpenAIModel
	body := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": SystemPrompt},
			{"role": "user", "content": prompt},
		},
	}
	if engineID == "anthropic_api" {
		endpoint = "https://api.anthropic.com/v1/messages"
		key, model = m.config.AnthropicKey, m.config.AnthropicModel
		body = map[string]any{
			"model": model, "max_tokens": 512, "system": SystemPrompt,
			"messages": []map[string]string{{"role": "user", "content": prompt}},
		}
	}
	if key == "" || model == "" {
		return fmt.Errorf("cần API key và model")
	}
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(
		ctx, http.MethodPost, endpoint, bytes.NewReader(data),
	)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	if engineID == "anthropic_api" {
		request.Header.Set("x-api-key", key)
		request.Header.Set("anthropic-version", "2023-06-01")
	} else {
		request.Header.Set("Authorization", "Bearer "+key)
	}
	response, err := m.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("không kết nối được API")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("API HTTP %d", response.StatusCode)
	}
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		}
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(&result); err != nil {
		return fmt.Errorf("phản hồi API không hợp lệ")
	}
	for _, choice := range result.Choices {
		onChunk(choice.Message.Content)
	}
	for _, content := range result.Content {
		if content.Type == "text" {
			onChunk(content.Text)
		}
	}
	return nil
}

func (m *Manager) TestSettings(ctx context.Context, settings Settings) (bool, string) {
	provider, err := m.configuredProvider(settings)
	if err != nil {
		return false, err.Error()
	}
	if settings.SelectedEngine == "rule_fallback" {
		return true, "AI đang tắt"
	}
	err = provider.StreamChatContext(ctx, "Chỉ trả lời OK.", func(string) {})
	if err != nil {
		return false, err.Error()
	}
	return true, "Engine đã trả lời thành công"
}
