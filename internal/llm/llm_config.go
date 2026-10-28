package llm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Settings struct {
	AuthMethod      string `json:"auth_method,omitempty"`
	authEngine      string
	authUpdate      *CLIAuthConfig
	SelectedEngine  string `json:"selected_engine"`
	GeminiKey       string `json:"gemini_key,omitempty"`
	OpenAIKey       string `json:"openai_key,omitempty"`
	AnthropicKey    string `json:"anthropic_key,omitempty"`
	OpenAIModel     string `json:"openai_model"`
	AnthropicModel  string `json:"anthropic_model"`
	LocalLLMURL     string `json:"local_llm_url"`
	GeminiKeySet    bool   `json:"gemini_key_set"`
	OpenAIKeySet    bool   `json:"openai_key_set"`
	AnthropicKeySet bool   `json:"anthropic_key_set"`
}

func validEngine(engineID string) bool {
	switch engineID {
	case "local_qwen", "gemini", "claude", "codex", "agy", "rule_fallback",
		"openai_api", "anthropic_api":
		return true
	}
	return false
}

func selectedEngine(config *Config) string {
	if config.SelectedEngine != "" {
		return config.SelectedEngine
	}
	for _, engineID := range config.PriorityOrder {
		if validEngine(engineID) {
			return engineID
		}
	}
	return "rule_fallback"
}

func keyStatus(key string) string {
	if key == "" {
		return "Chưa nhập API key"
	}
	return "Đã lưu API key; cần kiểm tra kết nối"
}

func (m *Manager) Settings() Settings {
	m.config.mu.RLock()
	defer m.config.mu.RUnlock()
	return Settings{
		AuthMethod:     m.config.CLIAuth[selectedEngine(m.config)].Method,
		SelectedEngine: selectedEngine(m.config), LocalLLMURL: m.config.LocalLLMURL,
		GeminiKeySet: m.config.GeminiKey != "", OpenAIKeySet: m.config.OpenAIKey != "",
		AnthropicKeySet: m.config.AnthropicKey != "",
		OpenAIModel:     m.config.OpenAIModel, AnthropicModel: m.config.AnthropicModel,
	}
}

func (m *Manager) configuredProvider(settings Settings) (*Manager, error) {
	provider := m.providerSnapshot()
	config := provider.config
	if !validEngine(settings.SelectedEngine) {
		return nil, fmt.Errorf("engine không hợp lệ")
	}
	config.SelectedEngine = settings.SelectedEngine
	if settings.GeminiKey != "" {
		config.GeminiKey = settings.GeminiKey
	}
	if settings.OpenAIKey != "" {
		config.OpenAIKey = settings.OpenAIKey
	}
	if settings.AnthropicKey != "" {
		config.AnthropicKey = settings.AnthropicKey
	}
	if settings.LocalLLMURL != "" {
		config.LocalLLMURL = settings.LocalLLMURL
	}
	if settings.OpenAIModel != "" {
		config.OpenAIModel = settings.OpenAIModel
	}
	if settings.AnthropicModel != "" {
		config.AnthropicModel = settings.AnthropicModel
	}
	if settings.AuthMethod != "" {
		if _, err := findAuthMethod(settings.SelectedEngine, settings.AuthMethod); err != nil {
			return nil, err
		}
		auth := config.CLIAuth[settings.SelectedEngine]
		if auth.Method != settings.AuthMethod {
			auth.Secret = ""
		}
		auth.Method = settings.AuthMethod
		config.CLIAuth[settings.SelectedEngine] = auth
	}
	return provider, nil
}

func (m *Manager) SaveSettings(settings Settings) error {
	m.config.mu.Lock()
	defer m.config.mu.Unlock()
	config := m.config
	candidate := Config{
		CLIAuth:    cloneCLIAuth(config.CLIAuth),
		ConfigPath: config.ConfigPath, SelectedEngine: config.SelectedEngine,
		GeminiKey: config.GeminiKey, OpenAIKey: config.OpenAIKey,
		AnthropicKey: config.AnthropicKey, LocalLLMURL: config.LocalLLMURL,
		OpenAIModel: config.OpenAIModel, AnthropicModel: config.AnthropicModel,
		ClaudePath: config.ClaudePath, CodexPath: config.CodexPath, AgyPath: config.AgyPath,
	}
	if settings.authUpdate != nil {
		settings.SelectedEngine = selectedEngine(config)
		candidate.CLIAuth[settings.authEngine] = *settings.authUpdate
	}
	if !validEngine(settings.SelectedEngine) {
		return fmt.Errorf("engine không hợp lệ")
	}
	if settings.AuthMethod != "" {
		if _, err := findAuthMethod(settings.SelectedEngine, settings.AuthMethod); err != nil {
			return err
		}
		auth := candidate.CLIAuth[settings.SelectedEngine]
		if auth.Method != settings.AuthMethod {
			auth.Secret = ""
		}
		auth.Method = settings.AuthMethod
		candidate.CLIAuth[settings.SelectedEngine] = auth
	}
	candidate.SelectedEngine = settings.SelectedEngine
	candidate.PriorityOrder = nil
	if settings.GeminiKey != "" {
		candidate.GeminiKey = settings.GeminiKey
	}
	if settings.OpenAIKey != "" {
		candidate.OpenAIKey = settings.OpenAIKey
	}
	if settings.AnthropicKey != "" {
		candidate.AnthropicKey = settings.AnthropicKey
	}
	if settings.LocalLLMURL != "" {
		candidate.LocalLLMURL = settings.LocalLLMURL
	}
	if settings.OpenAIModel != "" {
		candidate.OpenAIModel = settings.OpenAIModel
	}
	if settings.AnthropicModel != "" {
		candidate.AnthropicModel = settings.AnthropicModel
	}
	data, err := json.MarshalIndent(&candidate, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(config.ConfigPath), ".llm-config-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	if err = os.Rename(file.Name(), config.ConfigPath); err != nil {
		return err
	}
	config.CLIAuth = candidate.CLIAuth
	config.SelectedEngine = candidate.SelectedEngine
	config.PriorityOrder = nil
	config.GeminiKey = candidate.GeminiKey
	config.OpenAIKey = candidate.OpenAIKey
	config.AnthropicKey = candidate.AnthropicKey
	config.LocalLLMURL = candidate.LocalLLMURL
	config.OpenAIModel = candidate.OpenAIModel
	config.AnthropicModel = candidate.AnthropicModel
	m.restartAgent()
	return nil
}
