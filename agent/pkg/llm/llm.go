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
	"sync"
	"time"
)

type EngineInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`        // "local_llm", "gemini", "claude", "rule"
	Endpoint    string `json:"endpoint"`
	ModelName   string `json:"model_name"`
	Available   bool   `json:"available"`
	Priority    int    `json:"priority"`    // 1 là cao nhất
	StatusDesc  string `json:"status_desc"`
}

type Config struct {
	PriorityOrder []string `json:"priority_order"` // Danh sách Engine ID theo thứ tự ưu tiên
	GeminiKey     string   `json:"gemini_key"`
	LocalLLMURL   string   `json:"local_llm_url"`
	ClaudePath    string   `json:"claude_path"`
	ConfigPath    string   `json:"-"`
	mu            sync.RWMutex
}

type Manager struct {
	config     *Config
	httpClient *http.Client
}

func NewManager(configPath string) *Manager {
	if configPath == "" {
		configPath = "sentinel_config.json"
	}

	cfg := &Config{
		ConfigPath:    configPath,
		PriorityOrder: []string{"local_qwen", "gemini", "claude", "rule_fallback"},
		LocalLLMURL:   "http://127.0.0.1:9931/v1/chat/completions",
		GeminiKey:     os.Getenv("GEMINI_API_KEY"),
		ClaudePath:    "/home/linuxbrew/.linuxbrew/bin/claude",
	}

	// Đọc config đã lưu nếu có
	if data, err := os.ReadFile(configPath); err == nil {
		_ = json.Unmarshal(data, cfg)
	}

	return &Manager{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 45 * time.Second,
		},
	}
}

func (m *Manager) SaveConfig(newOrder []string, geminiKey string, localURL string) error {
	m.config.mu.Lock()
	defer m.config.mu.Unlock()

	if len(newOrder) > 0 {
		m.config.PriorityOrder = newOrder
	}
	if geminiKey != "" {
		m.config.GeminiKey = geminiKey
	}
	if localURL != "" {
		m.config.LocalLLMURL = localURL
	}

	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.config.ConfigPath, data, 0644)
}

func (m *Manager) ListEngines() []EngineInfo {
	m.config.mu.RLock()
	defer m.config.mu.RUnlock()

	var engines []EngineInfo

	// 1. Local LLM (NAS port 9931)
	localAvail, localDesc := m.probeLocalLLM()
	engines = append(engines, EngineInfo{
		ID:         "local_qwen",
		Name:       "Local LLM (Qwen2.5-1.5B NAS :9931)",
		Type:       "local_llm",
		Endpoint:   m.config.LocalLLMURL,
		ModelName:  "qwen2.5-1.5b-instruct-q4_k_m.gguf",
		Available:  localAvail,
		StatusDesc: localDesc,
	})

	// 2. Google Gemini API
	geminiAvail := m.config.GeminiKey != ""
	geminiDesc := "Chưa cấu hình API Key"
	if geminiAvail {
		geminiDesc = "Sẵn sàng (Gemini 2.5 Flash)"
	}
	engines = append(engines, EngineInfo{
		ID:         "gemini",
		Name:       "Google Gemini API (2.5 Flash)",
		Type:       "gemini",
		Endpoint:   "https://generativelanguage.googleapis.com",
		ModelName:  "gemini-2.5-flash",
		Available:  geminiAvail,
		StatusDesc: geminiDesc,
	})

	// 3. Claude CLI
	claudeAvail := false
	claudeDesc := "Không tìm thấy binary claude CLI"
	if _, err := os.Stat(m.config.ClaudePath); err == nil {
		claudeAvail = true
		claudeDesc = "Sẵn sàng (Claude 3.5 Sonnet CLI)"
	} else if p, err := exec.LookPath("claude"); err == nil {
		claudeAvail = true
		claudeDesc = fmt.Sprintf("Sẵn sàng (%s)", p)
	}
	engines = append(engines, EngineInfo{
		ID:         "claude",
		Name:       "Claude CLI (Anthropic Claude 3.5)",
		Type:       "claude",
		Endpoint:   m.config.ClaudePath,
		ModelName:  "claude-3-5-sonnet",
		Available:  claudeAvail,
		StatusDesc: claudeDesc,
	})

	// 4. Built-in Fallback Engine
	engines = append(engines, EngineInfo{
		ID:         "rule_fallback",
		Name:       "Built-in Rule Engine (Dự phòng cơ bản)",
		Type:       "rule",
		Endpoint:   "embedded",
		ModelName:  "direct-rule",
		Available:  true,
		StatusDesc: "Luôn hoạt động offline",
	})

	// Gán priority theo thứ tự m.config.PriorityOrder
	orderMap := make(map[string]int)
	for idx, id := range m.config.PriorityOrder {
		orderMap[id] = idx + 1
	}

	for i := range engines {
		if p, ok := orderMap[engines[i].ID]; ok {
			engines[i].Priority = p
		} else {
			engines[i].Priority = 99
		}
	}

	return engines
}

func (m *Manager) probeLocalLLM() (bool, string) {
	client := http.Client{Timeout: 1500 * time.Millisecond}
	resp, err := client.Get("http://127.0.0.1:9931/v1/models")
	if err != nil {
		return false, "Không kết nối được port 9931"
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return true, "Đang chạy mượt trên NAS (Llama.cpp)"
	}
	return false, fmt.Sprintf("HTTP status %d", resp.StatusCode)
}

const SystemPrompt = `Bạn là trợ lý thông minh Home Sentinel trên TV và nhà thông minh.
Trả lời bằng tiếng Việt, súc tích, định dạng ngắn gọn, thân thiện và dễ đọc.
Không khẳng định đã thao tác thiết bị nếu không có kết quả từ công cụ.`

// StreamChat tự động chạy theo thứ tự ưu tiên (Priority Order), nếu 1 engine lỗi sẽ fallback sang cái kế tiếp
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
		case "gemini":
			err = m.callGemini(prompt, onChunk)
		case "claude":
			err = m.callClaude(prompt, onChunk)
		case "rule_fallback":
			onChunk(fmt.Sprintf("Chào bạn! Yêu cầu \"%s\" đã được ghi nhận qua Rule Engine dự phòng.", prompt))
			return
		}

		if err == nil {
			return // Thành công, kết thúc
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

func (m *Manager) callClaude(prompt string, onChunk func(string)) error {
	cmd := exec.Command(m.config.ClaudePath, "-p", prompt, "--append-system-prompt", SystemPrompt)
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
		return fmt.Errorf("empty claude response")
	}
	return nil
}
