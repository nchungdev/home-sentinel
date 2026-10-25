package llm

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

func (m *Manager) StreamChat(prompt string, onChunk func(string)) {
	m.config.mu.RLock()
	order := make([]string, len(m.config.PriorityOrder))
	copy(order, m.config.PriorityOrder)
	m.config.mu.RUnlock()

	for _, engineID := range order {
		var err error
		switch engineID {
		case "local_qwen":
			err = m.callLocalLLM(prompt, onChunk)
		case "agy":
			err = m.callGenericCLI(m.config.AgyPath, []string{"-p", prompt, "--append-system-prompt", SystemPrompt}, onChunk)
		case "codex":
			err = m.callGenericCLI(m.config.CodexPath, []string{"exec", prompt}, onChunk)
		case "claude":
			err = m.callGenericCLI(m.config.ClaudePath, []string{"-p", prompt, "--append-system-prompt", SystemPrompt}, onChunk)
		case "gemini":
			err = m.callGemini(prompt, onChunk)
		case "rule_fallback":
			onChunk(fmt.Sprintf("Chào bạn! Yêu cầu \"%s\" đã được ghi nhận qua Rule Engine dự phòng.", prompt))
			return
		}

		if err == nil {
			return
		}
	}

	onChunk("Tất cả các mô hình AI hiện không khả dụng. Bạn vui lòng kiểm tra lại cấu hình!")
}

func (m *Manager) callLocalLLM(prompt string, onChunk func(string)) error {
	reqBody := map[string]interface{}{
		"messages": []map[string]string{
			{"role": "system", "content": SystemPrompt},
			{"role": "user", "content": prompt},
		},
		"stream":      true,
		"max_tokens":  512,
		"temperature": 0.7,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, m.config.LocalLLMURL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("local llm error: %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	hasData := false
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		jsonStr := strings.TrimPrefix(line, "data: ")
		if strings.TrimSpace(jsonStr) == "[DONE]" {
			break
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}

		if err := json.Unmarshal([]byte(jsonStr), &chunk); err == nil {
			if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
				hasData = true
				onChunk(chunk.Choices[0].Delta.Content)
			}
		}
	}

	if !hasData {
		return fmt.Errorf("empty local llm stream")
	}
	return nil
}

func (m *Manager) callGenericCLI(binPath string, args []string, onChunk func(string)) error {
	if binPath == "" {
		return fmt.Errorf("binary path empty")
	}
	cmd := exec.Command(binPath, args...)
	cmd.Dir = os.Getenv("HOME")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	scanner := bufio.NewScanner(stdout)
	hasData := false
	for scanner.Scan() {
		text := scanner.Text()
		if text != "" {
			hasData = true
			onChunk(text + "\n")
		}
	}

	if err := cmd.Wait(); err != nil {
		return err
	}
	if !hasData {
		return fmt.Errorf("empty cli response")
	}
	return nil
}

func (m *Manager) callGemini(prompt string, onChunk func(string)) error {
	if m.config.GeminiKey == "" {
		return fmt.Errorf("no gemini key")
	}

	endpoint := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:streamGenerateContent?alt=sse&key=%s",
		m.config.GeminiKey,
	)

	body := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{
					{"text": SystemPrompt + "\n\nNgười dùng: " + prompt},
				},
			},
		},
	}

	data, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("gemini api error: %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	hasData := false
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		jsonStr := strings.TrimPrefix(line, "data: ")

		var chunk struct {
			Candidates []struct {
				Content struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"content"`
			} `json:"candidates"`
		}

		if err := json.Unmarshal([]byte(jsonStr), &chunk); err == nil {
			for _, cand := range chunk.Candidates {
				for _, part := range cand.Content.Parts {
					if part.Text != "" {
						hasData = true
						onChunk(part.Text)
					}
				}
			}
		}
	}

	if !hasData {
		return fmt.Errorf("empty gemini stream")
	}
	return nil
}
