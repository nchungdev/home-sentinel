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
	"time"
)

type Engine struct {
	LocalLLMURL string
	GeminiKey   string
	ClaudePath  string
	HTTPClient  *http.Client
}

func NewEngine() *Engine {
	localURL := os.Getenv("AGENT_LOCAL_LLM_URL")
	if localURL == "" {
		localURL = "http://127.0.0.1:9931/v1/chat/completions"
	}

	geminiKey := os.Getenv("GEMINI_API_KEY")

	claudePath := "/home/linuxbrew/.linuxbrew/bin/claude"
	if p, err := exec.LookPath("claude"); err == nil {
		claudePath = p
	}

	return &Engine{
		LocalLLMURL: localURL,
		GeminiKey:   geminiKey,
		ClaudePath:  claudePath,
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

const SystemPrompt = `Bạn là trợ lý thông minh Home Sentinel trên TV và nhà thông minh.
Trả lời bằng tiếng Việt, súc tích, định dạng ngắn gọn, thân thiện và dễ đọc.
Không khẳng định đã thao tác thiết bị nếu không có kết quả từ công cụ.`

func (e *Engine) StreamChat(prompt string, onChunk func(string)) error {
	// Ưu tiên 1: Thử Local LLM trên NAS (llama.cpp port 9931)
	err := e.streamLocalLLM(prompt, onChunk)
	if err == nil {
		return nil
	}

	// Ưu tiên 2: Thử Claude CLI
	if _, statErr := os.Stat(e.ClaudePath); statErr == nil {
		errClaude := e.streamClaudeCLI(prompt, onChunk)
		if errClaude == nil {
			return nil
		}
	}

	// Fallback nếu không có model nào chạy
	onChunk(fmt.Sprintf("Chào bạn! Tôi đã nhận được: \"%s\". Hiện tại Local LLM (port 9931) tạm thời không phản hồi.", prompt))
	return nil
}

func (e *Engine) streamLocalLLM(prompt string, onChunk func(string)) error {
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

	req, err := http.NewRequest(http.MethodPost, e.LocalLLMURL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("local llm returned status %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	hasOutput := false
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
				hasOutput = true
				onChunk(chunk.Choices[0].Delta.Content)
			}
		}
	}

	if !hasOutput {
		return fmt.Errorf("no output from local llm")
	}
	return nil
}

func (e *Engine) streamClaudeCLI(prompt string, onChunk func(string)) error {
	cmd := exec.Command(e.ClaudePath, "-p", prompt, "--append-system-prompt", SystemPrompt)
	cmd.Dir = os.Getenv("HOME")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		text := scanner.Text()
		if text != "" {
			onChunk(text + "\n")
		}
	}

	return cmd.Wait()
}
