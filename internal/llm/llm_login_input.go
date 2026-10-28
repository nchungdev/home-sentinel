package llm

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var terminalControlPattern = regexp.MustCompile(
	`\x1b\[[0-?]*[ -/]*[@-~]|\x1b\][^\x07\x1b]*(\x07|\x1b\\)`,
)

var loginCodePattern = regexp.MustCompile(`^[A-Za-z0-9._~+/=#:%-]+$`)

var loginURLPattern = regexp.MustCompile(`https://[^\s\x1b<>"']+`)

func AuthorizationURL(output string) string { return loginURL(output) }

func loginURL(output string) string {
	for _, candidate := range loginURLPattern.FindAllString(output, -1) {
		parsed, err := url.Parse(strings.TrimRight(candidate, ").,;"))
		if err != nil || parsed.User != nil {
			continue
		}
		switch parsed.Hostname() {
		case "auth.openai.com", "claude.ai", "claude.com", "console.anthropic.com", "platform.claude.com",
			"accounts.google.com", "antigravity.google", "antigravity.google.com":
			if strings.Contains(parsed.Path, "auth") || strings.Contains(parsed.Path, "/codex/device") {
				return parsed.String()
			}
		}
	}
	return ""
}

func (m *Manager) SubmitLoginCode(engineID, code string) error {
	code = strings.TrimSpace(code)
	if code == "" || len(code) > 4096 || strings.ContainsAny(code, "\r\n\x00") {
		return fmt.Errorf("mã xác thực không hợp lệ")
	}
	m.loginMutex.Lock()
	session := m.logins[engineID]
	m.loginMutex.Unlock()
	if session == nil {
		return fmt.Errorf("chưa có phiên đăng nhập")
	}
	session.mutex.Lock()
	running, input := session.state.Running, session.input
	interactive := session.interactive
	authURL := session.state.LoginURL

	session.mutex.Unlock()
	if !running || input == nil {
		return fmt.Errorf("phiên đăng nhập đã kết thúc; hãy thử lại")
	}
	if authURL == "" {
		return fmt.Errorf("CLI chưa bắt đầu bước xác thực")
	}
	if strings.HasPrefix(code, "http://") {
		return relayLoginCallback(authURL, code)
	}
	if !loginCodePattern.MatchString(code) {
		return fmt.Errorf("mã xác thực không hợp lệ")
	}
	session.mutex.Lock()
	session.secret = code
	session.mutex.Unlock()
	ending := "\n"
	if interactive {
		ending = "\r"
	}
	if _, err := fmt.Fprint(input, code+ending); err != nil {
		return fmt.Errorf("CLI không nhận được mã xác thực")
	}
	return nil
}

func (m *Manager) NavigateLogin(engineID, key string) error {
	keys := map[string]string{"up": "\x1b[A", "down": "\x1b[B", "enter": "\r"}
	sequence, valid := keys[key]
	if !valid {
		return fmt.Errorf("phím không được hỗ trợ trong phiên đăng nhập")
	}
	m.loginMutex.Lock()
	session := m.logins[engineID]
	m.loginMutex.Unlock()
	if session == nil {
		return fmt.Errorf("chưa có phiên đăng nhập")
	}
	session.mutex.Lock()
	running, interactive, input := session.state.Running, session.interactive, session.input
	session.mutex.Unlock()
	if !running || !interactive || input == nil {
		return fmt.Errorf("CLI không chờ lựa chọn trong terminal")
	}
	if _, err := fmt.Fprint(input, sequence); err != nil {
		return fmt.Errorf("không gửi được lựa chọn")
	}
	return nil
}
