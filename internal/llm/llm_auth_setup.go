package llm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type LoginRequest struct {
	EngineID string `json:"engine_id"`
	Method   string `json:"method"`
	Secret   string `json:"secret"`
	Email    string `json:"email"`
	Code     string `json:"code"`
	Action   string `json:"action"`
	Key      string `json:"key"`
}

func (m *Manager) saveCLIAuth(engineID string, auth CLIAuthConfig) error {
	return m.SaveSettings(Settings{authEngine: engineID, authUpdate: &auth})
}

func configureAGYProvider(provider string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	path := filepath.Join(homeDir, ".gemini", "antigravity-cli", "settings.json")
	settings := map[string]any{}
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &settings); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if provider == "gemini" {
		settings["modelProvider"] = provider
	} else {
		delete(settings, "modelProvider")
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func (m *Manager) prepareAuth(request LoginRequest, method AuthMethod) (bool, error) {
	if method.Kind == "secret" {
		if request.Secret == "" {
			return false, fmt.Errorf("vui lòng nhập API key hoặc token")
		}
		if request.EngineID == "codex" {
			return false, nil
		}
		if request.EngineID == "agy" {
			if err := configureAGYProvider("gemini"); err != nil {
				return false, err
			}
		}
		auth := CLIAuthConfig{Method: method.ID, Secret: request.Secret}
		err := m.saveCLIAuth(request.EngineID, auth)
		return true, err
	}
	if request.EngineID == "agy" {
		if err := configureAGYProvider(""); err != nil {
			return false, err
		}
	}
	return false, nil
}

func (m *Manager) loginCommand(request LoginRequest) (string, []string, bool) {
	switch request.EngineID {
	case "codex":
		option := map[string]string{
			"chatgpt": "--device-auth", "api_key": "--with-api-key",
			"access_token": "--with-access-token",
		}[request.Method]
		return m.config.CodexPath, []string{"login", option}, false
	case "claude":
		option := map[string]string{
			"subscription": "--claudeai", "console": "--console", "sso": "--sso",
		}[request.Method]
		args := []string{"auth", "login", option}
		if request.Email != "" {
			args = append(args, "--email", request.Email)
		}
		return m.config.ClaudePath, args, false
	case "agy":
		if request.Method == "adc" {
			args := []string{"auth", "application-default", "login", "--no-launch-browser"}
			return "gcloud", args, false
		}
		return m.config.AgyPath, nil, true
	}
	return "", nil, false
}
