package llm

import (
	"context"
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
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"` // "local_llm", "gemini", "claude", "agy", "codex", "rule"
	Endpoint  string `json:"endpoint"`
	ModelName string `json:"model_name"`
	Available bool   `json:"available"`

	StatusDesc string `json:"status_desc"`
}

type Config struct {
	CLIAuth        map[string]CLIAuthConfig `json:"cli_auth,omitempty"`
	PriorityOrder  []string                 `json:"priority_order,omitempty"`
	SelectedEngine string                   `json:"selected_engine"`
	OpenAIModel    string                   `json:"openai_model"`
	AnthropicModel string                   `json:"anthropic_model"`
	OpenAIKey      string                   `json:"openai_key"`
	AnthropicKey   string                   `json:"anthropic_key"`
	GeminiKey      string                   `json:"gemini_key"`
	LocalLLMURL    string                   `json:"local_llm_url"`
	ClaudePath     string                   `json:"claude_path"`
	AgyPath        string                   `json:"agy_path"`
	CodexPath      string                   `json:"codex_path"`
	ConfigPath     string                   `json:"-"`
	mu             sync.RWMutex
}

type Manager struct {
	config     *Config
	httpClient *http.Client
	usage      usageTracker
	agy        *agyPool
	loginMutex sync.Mutex
	logins     map[string]*loginSession
}

func NewManager(configPath string) *Manager {
	if configPath == "" {
		configPath = "sentinel_llm_config.json"
	}

	cfg := &Config{
		ConfigPath: configPath,

		LocalLLMURL:  "http://127.0.0.1:9931/v1/chat/completions",
		GeminiKey:    os.Getenv("GEMINI_API_KEY"),
		OpenAIKey:    os.Getenv("OPENAI_API_KEY"),
		AnthropicKey: os.Getenv("ANTHROPIC_API_KEY"),
		ClaudePath:   findBinary("claude", []string{"/home/linuxbrew/.linuxbrew/bin/claude", "/usr/local/bin/claude"}),
		AgyPath:      findBinary("agy", []string{"/usr/local/bin/agy"}),
		CodexPath:    findBinary("codex", []string{"/home/linuxbrew/.linuxbrew/bin/codex", "/usr/bin/codex"}),
	}

	if data, err := os.ReadFile(configPath); err == nil {
		_ = json.Unmarshal(data, cfg)
	}

	cfg.SelectedEngine = selectedEngine(cfg)
	cfg.PriorityOrder = nil

	manager := &Manager{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 45 * time.Second,
		},
	}
	manager.agy = newAgyPool(manager)
	return manager
}

func findBinary(name string, fallbacks []string) string {
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	for _, f := range fallbacks {
		if _, err := os.Stat(f); err == nil {
			return f
		}
	}
	return name
}

func (m *Manager) ListEngines() []EngineInfo {
	m.config.mu.RLock()
	defer m.config.mu.RUnlock()

	var engines []EngineInfo

	// 1. Local LLM (Llama.cpp port 9931)
	localAvail, localDesc := m.probeLocalLLM()
	engines = append(engines, EngineInfo{
		ID:         "local_qwen",
		Name:       "Local LLM",
		Type:       "local_llm",
		Endpoint:   m.config.LocalLLMURL,
		ModelName:  "qwen2.5-1.5b-instruct-q4_k_m.gguf",
		Available:  localAvail,
		StatusDesc: localDesc,
	})

	// 2. AGY CLI (Google Antigravity)
	agyAvail, agyDesc := m.probeCLI(m.config.AgyPath, "--version")
	engines = append(engines, EngineInfo{
		ID:         "agy",
		Name:       "AGY CLI (Google Antigravity Engine)",
		Type:       "agy",
		Endpoint:   m.config.AgyPath,
		ModelName:  "antigravity-native",
		Available:  agyAvail,
		StatusDesc: agyDesc,
	})

	// 3. Codex CLI (OpenAI Codex CLI)
	codexAvail, codexDesc := m.probeCLI(m.config.CodexPath, "--version")
	engines = append(engines, EngineInfo{
		ID:         "codex",
		Name:       "Codex CLI (OpenAI Codex Engine)",
		Type:       "codex",
		Endpoint:   m.config.CodexPath,
		ModelName:  "codex-app-server",
		Available:  codexAvail,
		StatusDesc: codexDesc,
	})

	// 4. Claude CLI
	claudeAvail, claudeDesc := m.probeCLI(m.config.ClaudePath, "--version")
	engines = append(engines, EngineInfo{
		ID:         "claude",
		Name:       "Claude Code",
		Type:       "claude",
		Endpoint:   m.config.ClaudePath,
		ModelName:  "claude-3-5-sonnet",
		Available:  claudeAvail,
		StatusDesc: claudeDesc,
	})

	// 5. Google Gemini API
	geminiAvail := m.config.GeminiKey != ""
	geminiDesc := "Chưa cấu hình API Key"
	if geminiAvail {
		geminiDesc = "Đã lưu API key; cần kiểm tra kết nối"
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

	// 6. Built-in Rule Engine
	engines = append(engines, EngineInfo{
		ID:         "rule_fallback",
		Name:       "Tắt AI",
		Type:       "rule",
		Endpoint:   "embedded",
		ModelName:  "direct-rule",
		Available:  true,
		StatusDesc: "Không gọi model AI",
	})

	engines = append(engines,
		EngineInfo{ID: "openai_api", Name: "OpenAI API", Type: "api",
			Available: m.config.OpenAIKey != "", StatusDesc: keyStatus(m.config.OpenAIKey)},
		EngineInfo{ID: "anthropic_api", Name: "Anthropic API", Type: "api",
			Available: m.config.AnthropicKey != "", StatusDesc: keyStatus(m.config.AnthropicKey)},
	)

	return engines
}

func (m *Manager) probeLocalLLM() (bool, string) {
	testURL := m.config.LocalLLMURL
	if testURL == "" {
		testURL = "http://127.0.0.1:9931/v1/chat/completions"
	}
	modelsURL := strings.Replace(testURL, "/chat/completions", "/models", 1)

	client := http.Client{Timeout: 2000 * time.Millisecond}
	resp, err := client.Get(modelsURL)
	if err != nil {
		return false, fmt.Sprintf("Không kết nối được: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return true, "Đang chạy mượt (OpenAI / Llama.cpp)"
	}
	return false, fmt.Sprintf("HTTP status %d", resp.StatusCode)
}

func (m *Manager) probeCLI(path string, arg string) (bool, string) {
	if path == "" {
		return false, "Chưa cài đặt CLI"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, arg)
	out, err := cmd.Output()
	if err == nil {
		ver := strings.TrimSpace(string(out))
		if len(ver) > 40 {
			ver = ver[:40]
		}
		return true, fmt.Sprintf("Đã cài CLI (%s)", ver)
	}
	return false, "Không tìm thấy hoặc chưa xác thực"
}

const SystemPrompt = `Bạn là trợ lý thông minh Home Sentinel cho gia đình và TV Launcher.
QUY TẮC BẮT BUỘC:
1. Luôn luôn trả lời hoàn toàn bằng Tiếng Việt chuẩn. Tuyệt đối KHÔNG sử dụng chữ Hán/tiếng Trung trong mọi trường hợp (ví dụ: không dùng 智能家居, hãy dùng từ "nhà thông minh").
2. Câu trả lời súc tích, ngắn gọn, thân thiện và tự nhiên.
3. Nếu không có kết quả từ thiết bị phần cứng, không khẳng định là đã bật/tắt thiết bị.
4. Không dùng công cụ, không chạy lệnh, không đọc file. Chỉ trả lời trực tiếp bằng kiến thức của bạn.`
