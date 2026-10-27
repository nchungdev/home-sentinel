package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

func (m *Manager) callLocalLLM(ctx context.Context, prompt string, onChunk func(string)) error {
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

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, m.config.LocalLLMURL, bytes.NewReader(data),
	)
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
	scanner.Buffer(make([]byte, 4096), 1024*1024)
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

	if err := scanner.Err(); err != nil {
		return err
	}
	if !hasData {
		return fmt.Errorf("empty local llm stream")
	}
	return nil
}

func (m *Manager) callGenericCLI(ctx context.Context, binPath string, args []string, onChunk func(string)) error {
	if binPath == "" {
		return fmt.Errorf("binary path empty")
	}
	cmd := exec.CommandContext(ctx, binPath, args...)
	cmd.WaitDelay = time.Second
	cmd.Dir = os.Getenv("HOME")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	hasData := false
	for scanner.Scan() {
		text := scanner.Text()
		if text != "" {
			hasData = true
			onChunk(text + "\n")
		}
	}

	if scanner.Err() != nil {
		_ = cmd.Process.Kill()
	}
	if err := cmd.Wait(); err != nil {
		return err
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if !hasData {
		return fmt.Errorf("empty cli response")
	}
	return nil
}

func (m *Manager) callGemini(ctx context.Context, prompt string, onChunk func(string)) error {
	if m.config.GeminiKey == "" {
		return fmt.Errorf("no gemini key")
	}

	endpoint := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/"+
			"gemini-2.5-flash:streamGenerateContent?alt=sse&key=%s",
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

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
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
	scanner.Buffer(make([]byte, 4096), 1024*1024)
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

	if err := scanner.Err(); err != nil {
		return err
	}
	if !hasData {
		return fmt.Errorf("empty gemini stream")
	}
	return nil
}
