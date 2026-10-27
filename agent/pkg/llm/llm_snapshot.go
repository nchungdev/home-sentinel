package llm

func (m *Manager) providerSnapshot() *Manager {
	m.config.mu.RLock()
	defer m.config.mu.RUnlock()
	return &Manager{
		httpClient: m.httpClient,
		config: &Config{
			PriorityOrder: append([]string(nil), m.config.PriorityOrder...),
			GeminiKey:     m.config.GeminiKey,
			LocalLLMURL:   m.config.LocalLLMURL,
			ClaudePath:    m.config.ClaudePath,
			AgyPath:       m.config.AgyPath,
			CodexPath:     m.config.CodexPath,
		},
	}
}
