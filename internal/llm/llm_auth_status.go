package llm

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type AuthStatus struct {
	Authenticated bool   `json:"authenticated"`
	Method        string `json:"method"`
	Account       string `json:"account,omitempty"`
	Message       string `json:"message"`
}

type AuthReport struct {
	EngineID       string       `json:"engine_id"`
	Methods        []AuthMethod `json:"methods"`
	SelectedMethod string       `json:"selected_method"`
	Status         AuthStatus   `json:"status"`
	Login          LoginState   `json:"login"`
}

func (m *Manager) AuthReport(ctx context.Context, engineID, method string) (AuthReport, error) {
	provider := m.providerSnapshot()
	login := m.LoginStatus(engineID)
	if method == "" && login.Running {
		method = login.Method
	}
	if method == "" {
		method = provider.config.CLIAuth[engineID].Method
	}
	if _, err := findAuthMethod(engineID, method); err != nil {
		return AuthReport{}, err
	}
	status := provider.authStatus(ctx, engineID, method)
	if method == "" {
		method = status.Method
		if method == "" {
			method = authMethods(engineID)[0].ID
		}
	}
	return AuthReport{EngineID: engineID, Methods: authMethods(engineID), SelectedMethod: method,
		Status: status, Login: login}, nil
}

func authProbe(ctx context.Context, path string, args, environment []string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, path, args...)
	command.Env = environment
	command.WaitDelay = time.Second
	return command.CombinedOutput()
}

func (m *Manager) authStatus(ctx context.Context, engineID, method string) AuthStatus {
	status := AuthStatus{Method: method, Message: "Chưa đăng nhập trên máy chạy Sentinel"}
	environment := m.commandEnvironment(engineID, method)
	switch engineID {
	case "codex":
		output, err := authProbe(ctx, m.config.CodexPath, []string{"login", "status"}, environment)
		if err != nil {
			return status
		}
		actual := ""
		switch {
		case strings.Contains(string(output), "ChatGPT"):
			actual = "chatgpt"
		case strings.Contains(strings.ToLower(string(output)), "api key"):
			actual = "api_key"
		case strings.Contains(strings.ToLower(string(output)), "access token"):
			actual = "access_token"
		}
		if actual == "" || (method != "" && method != actual) {
			return status
		}
		status.Method, status.Authenticated = actual, true
	case "claude":
		args := []string{"auth", "status", "--json"}
		output, err := authProbe(ctx, m.config.ClaudePath, args, environment)
		if err != nil {
			return status
		}
		var native struct {
			LoggedIn   bool   `json:"loggedIn"`
			AuthMethod string `json:"authMethod"`
			Email      string `json:"email"`
		}
		if json.Unmarshal(output, &native) != nil || !native.LoggedIn {
			return status
		}
		actual := map[string]string{
			"claude.ai": "subscription", "oauth_token": "oauth_token", "api_key": "api_key",
			"api_key_helper": "environment", "third_party": "environment",
		}[native.AuthMethod]
		matches := method == "" || method == actual ||
			(method == "sso" && actual == "subscription") ||
			(method == "console" && actual == "api_key")
		if actual == "" || !matches {
			return status
		}
		if method != "" {
			actual = method
		}
		status.Method, status.Authenticated, status.Account = actual, true, native.Email
	case "agy":
		return m.agyAuthStatus(ctx, method)
	}
	if status.Authenticated {
		status.Message = "Đã đăng nhập trên máy chạy Sentinel"
	}
	return status
}

func (m *Manager) agyAuthStatus(ctx context.Context, method string) AuthStatus {
	status := AuthStatus{Method: method, Message: "Chưa có phiên Google trên máy chạy Sentinel"}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return status
	}
	var settings struct {
		ModelProvider string `json:"modelProvider"`
	}
	data, _ := os.ReadFile(filepath.Join(homeDir, ".gemini", "antigravity-cli", "settings.json"))
	_ = json.Unmarshal(data, &settings)
	if method == "api_key" || (method == "" && settings.ModelProvider == "gemini") {
		status.Method = "api_key"
		credential := m.config.CLIAuth["agy"].Secret
		status.Authenticated = settings.ModelProvider == "gemini" &&
			(credential != "" || os.Getenv("GEMINI_API_KEY") != "")
		status.Message = "Đã cấu hình Gemini API key; kiểm tra kết nối để xác minh"
		if !status.Authenticated {
			status.Message = "Chưa cấu hình Gemini API key"
		}
		return status
	}
	if settings.ModelProvider == "gemini" {
		status.Message = "CLI đang dùng Gemini API key; chọn Đăng nhập để đổi phương thức"
		return status
	}
	if method == "adc" || (method == "" && nativeADCEnabled(homeDir)) {
		status.Method = "adc"
		if account, configured := cachedADC(); configured {
			status.Authenticated, status.Account = true, account
			status.Message = "Đã có cấu hình ADC trên máy chủ; không cần đăng nhập lại"
			return status
		}
		args := []string{"auth", "application-default", "print-access-token"}
		if _, err := exec.LookPath("gcloud"); err != nil {
			status.Message = "Cần cài Google Cloud CLI (gcloud) trên máy chạy Sentinel"
			return status
		}
		_, err := authProbe(ctx, "gcloud", args,
			m.commandEnvironment("agy", "adc"))
		status.Authenticated = err == nil
		if status.Authenticated {
			status.Message = "Đã có Google Cloud ADC trên máy chủ"
		}
		return status
	}
	status.Method = "google"
	var profile struct {
		Email string `json:"id_token_email"`
		Token struct {
			AccessToken  string    `json:"access_token"`
			RefreshToken string    `json:"refresh_token"`
			Expiry       time.Time `json:"expiry"`
		} `json:"token"`
	}
	profilePath := filepath.Join(homeDir, ".gemini", "antigravity-cli", "antigravity-oauth-token")
	data, err = os.ReadFile(profilePath)
	if err != nil || json.Unmarshal(data, &profile) != nil {
		return status
	}
	status.Authenticated = profile.Token.AccessToken != "" &&
		(profile.Token.Expiry.After(time.Now()) || profile.Token.RefreshToken != "")
	if status.Authenticated {
		status.Account = profile.Email
		status.Message = "Đã có phiên Google trên máy chủ; không cần đăng nhập lại"
	}
	return status
}
