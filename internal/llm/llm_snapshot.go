package llm

func (m *Manager) providerSnapshot() *Manager {
	m.config.mu.RLock()
	defer m.config.mu.RUnlock()
	return &Manager{
		httpClient: m.httpClient,
		config: &Config{
			ConfigPath:     m.config.ConfigPath,
			CLIAuth:        cloneCLIAuth(m.config.CLIAuth),
			SelectedEngine: selectedEngine(m.config),
			OpenAIKey:      m.config.OpenAIKey,
			OpenAIModel:    m.config.OpenAIModel,
			AnthropicModel: m.config.AnthropicModel,
			AnthropicKey:   m.config.AnthropicKey,
			GeminiKey:      m.config.GeminiKey,
			LocalLLMURL:    m.config.LocalLLMURL,
			ClaudePath:     m.config.ClaudePath,
			AgyPath:        m.config.AgyPath,
			CodexPath:      m.config.CodexPath,
		},
	}
}
