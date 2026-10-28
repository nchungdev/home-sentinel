package llm

import (
	"fmt"
	"os"
	"strings"
)

type CLIAuthConfig struct {
	Method string `json:"method"`
	Secret string `json:"secret,omitempty"`
}

type AuthMethod struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Kind  string `json:"kind"`
}

func authMethods(engineID string) []AuthMethod {
	switch engineID {
	case "codex":
		return []AuthMethod{
			{"chatgpt", "Tài khoản ChatGPT", "browser"},
			{"api_key", "OpenAI API key", "secret"},
			{"access_token", "Access token tổ chức", "secret"},
		}
	case "claude":
		return []AuthMethod{
			{"subscription", "Tài khoản Claude subscription", "browser"},
			{"console", "Anthropic Console", "browser"},
			{"sso", "SSO tổ chức", "browser"},
			{"api_key", "Anthropic API key", "secret"},
			{"oauth_token", "Claude Code OAuth token", "secret"},
			{"environment", "Cấu hình cloud trên máy chủ", "environment"},
		}
	case "agy":
		return []AuthMethod{
			{"google", "Tài khoản Google / Gemini Enterprise", "browser"},
			{"api_key", "Gemini API key", "secret"},
			{"adc", "Google Cloud Application Default Credentials", "browser"},
		}
	}
	return nil
}

func findAuthMethod(engineID, methodID string) (AuthMethod, error) {
	methods := authMethods(engineID)
	if methodID == "" && len(methods) > 0 {
		return methods[0], nil
	}
	for _, method := range methods {
		if method.ID == methodID {
			return method, nil
		}
	}
	return AuthMethod{}, fmt.Errorf("phương thức xác thực không được engine hỗ trợ")
}

func cloneCLIAuth(source map[string]CLIAuthConfig) map[string]CLIAuthConfig {
	result := make(map[string]CLIAuthConfig, len(source))
	for engineID, auth := range source {
		result[engineID] = auth
	}
	return result
}

func authSecretName(engineID, method string) string {
	switch engineID + "/" + method {
	case "codex/api_key":
		return "OPENAI_API_KEY"
	case "codex/access_token":
		return "CODEX_ACCESS_TOKEN"
	case "claude/api_key":
		return "ANTHROPIC_API_KEY"
	case "claude/oauth_token":
		return "CLAUDE_CODE_OAUTH_TOKEN"
	case "agy/api_key":
		return "GEMINI_API_KEY"
	}
	return ""
}

func (m *Manager) commandEnvironment(engineID, method string) []string {
	if method == "" || method == "environment" {
		return os.Environ()
	}
	environment := subscriptionEnvironment()
	secretName := authSecretName(engineID, method)
	secret := os.Getenv(secretName)
	if m.config != nil {
		if auth := m.config.CLIAuth[engineID]; auth.Method == method && auth.Secret != "" {
			secret = auth.Secret
		}
	}
	if secretName != "" && secret != "" {
		environment = append(environment, secretName+"="+secret)
	}
	if engineID == "agy" && method == "adc" {
		environment = append(environment, "AGY_ADC_AUTH=true")
		if _, path, found := cachedADCProfile(); found {
			environment = append(environment, "GOOGLE_APPLICATION_CREDENTIALS="+path)
		}
	}
	return environment
}

func engineForPath(config *Config, path string) string {
	if config == nil {
		return ""
	}
	for engineID, enginePath := range map[string]string{
		"codex": config.CodexPath, "claude": config.ClaudePath, "agy": config.AgyPath,
	} {
		if enginePath == path {
			return engineID
		}
	}
	return ""
}

func secretEnvironmentName(name string) bool {
	switch strings.ToUpper(name) {
	case "CLAUDE_CODE_USE_BEDROCK", "CLAUDE_CODE_USE_VERTEX", "CLAUDE_CODE_USE_FOUNDRY",
		"ANTHROPIC_BASE_URL",
		"OPENAI_API_KEY", "ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "CODEX_API_KEY",
		"CODEX_ACCESS_TOKEN", "CLAUDE_CODE_OAUTH_TOKEN", "GEMINI_API_KEY", "GOOGLE_API_KEY",
		"AGY_ADC_AUTH":
		return true
	}
	return false
}
