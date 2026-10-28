package llm

import (
	"context"
	"fmt"
	"io"
	"net/mail"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type LoginState struct {
	EngineID  string `json:"engine_id"`
	Method    string `json:"method"`
	LoginURL  string `json:"login_url"`
	Output    string `json:"output"`
	Running   bool   `json:"running"`
	Succeeded bool   `json:"succeeded"`
	Message   string `json:"message"`
}

type loginSession struct {
	input         io.WriteCloser
	mutex         sync.Mutex
	state         LoginState
	cancel        context.CancelFunc
	complete      sync.Once
	interactive   bool
	secret        string
	browserOutput string
}

func (s *loginSession) Write(data []byte) (int, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	const limit = 32768
	s.state.Output += string(data)
	if s.secret != "" {
		s.state.Output = strings.ReplaceAll(s.state.Output, s.secret, "[redacted]")
	}
	if authURL := loginURL(s.state.Output); authURL != "" {
		s.state.LoginURL = authURL
	}
	if len(s.state.Output) > limit {
		s.state.Output = s.state.Output[len(s.state.Output)-limit:]
	}
	return len(data), nil
}

func (s *loginSession) snapshot() LoginState {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	state := s.state
	state.Output = terminalControlPattern.ReplaceAllString(state.Output, "")
	return state
}

func (s *loginSession) finish(authenticated bool, message string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if !s.state.Running && s.state.Message != "" {
		return
	}
	s.state.Running, s.state.Succeeded, s.state.Message = false, authenticated, message
}

func (m *Manager) StartLogin(engineID string) (LoginState, error) {
	return m.BeginLogin(context.Background(), LoginRequest{EngineID: engineID})
}

func (m *Manager) BeginLogin(ctx context.Context, request LoginRequest) (LoginState, error) {
	method, err := findAuthMethod(request.EngineID, request.Method)
	if err != nil {
		return LoginState{}, err
	}
	request.Method = method.ID
	provider := m.providerSnapshot()
	status := provider.authStatus(ctx, request.EngineID, request.Method)
	if status.Authenticated && request.Secret == "" {
		return LoginState{EngineID: request.EngineID, Method: method.ID,
			Succeeded: true, Message: status.Message}, nil
	}
	if method.ID == "sso" {
		address, err := mail.ParseAddress(request.Email)
		if err != nil || address.Address != request.Email {
			return LoginState{}, fmt.Errorf("vui lòng nhập email tổ chức để đăng nhập SSO")
		}
	}
	if method.Kind == "environment" {
		return LoginState{}, fmt.Errorf("máy chủ chưa có cấu hình xác thực cloud hợp lệ")
	}
	m.loginMutex.Lock()
	defer m.loginMutex.Unlock()
	if m.logins == nil {
		m.logins = make(map[string]*loginSession)
	}
	if previous := m.logins[request.EngineID]; previous != nil && previous.snapshot().Running {
		if previous.snapshot().Method != method.ID {
			return LoginState{}, fmt.Errorf("hãy hủy phiên hiện tại trước khi đổi phương thức")
		}
		return previous.snapshot(), nil
	}
	configured, err := m.prepareAuth(request, method)
	if err != nil {
		return LoginState{}, err
	}
	if configured {
		return LoginState{EngineID: request.EngineID, Method: method.ID, Succeeded: true,
			Message: "Đã lưu API key/token trên máy chủ; kiểm tra kết nối để xác minh"}, nil
	}
	provider = m.providerSnapshot()
	path, args, interactive := provider.loginCommand(request)
	loginContext, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	workspace, err := os.MkdirTemp("", "sentinel-login-*")
	if err != nil {
		cancel()
		return LoginState{}, err
	}
	command := exec.CommandContext(loginContext, path, args...)
	command.Dir = workspace
	command.Env = provider.commandEnvironment(request.EngineID, request.Method)
	if request.EngineID == "agy" && request.Method == "google" {
		command.Env = append(command.Env, "SSH_CONNECTION=127.0.0.1 0 127.0.0.1 0")
	}
	command.WaitDelay = time.Second
	session := &loginSession{cancel: cancel, interactive: interactive, secret: request.Secret,
		state: LoginState{EngineID: request.EngineID, Method: method.ID, Running: true,
			Message: "Đang khởi chạy luồng đăng nhập"}}
	if request.EngineID == "claude" {
		executable, executableError := os.Executable()
		if executableError != nil {
			cancel()
			os.RemoveAll(workspace)
			return LoginState{}, executableError
		}
		session.browserOutput = filepath.Join(workspace, "browser-url")
		command.Env = append(command.Env, "BROWSER="+executable,
			"SENTINEL_LOGIN_BROWSER_FILE="+session.browserOutput)
	}
	wait, err := startLoginCommand(loginContext, command, session, interactive)
	if err != nil {
		cancel()
		os.RemoveAll(workspace)
		return LoginState{}, fmt.Errorf("không mở được CLI trên máy chủ; kiểm tra cài đặt")
	}
	m.logins[request.EngineID] = session
	if request.Secret != "" {
		if _, err := fmt.Fprintln(session.input, request.Secret); err != nil {
			cancel()
		}
		session.input.Close()
	}
	go m.watchLogin(loginContext, provider, session, request)
	go func() {
		defer cancel()
		defer session.input.Close()
		defer os.RemoveAll(workspace)
		err := wait()
		status := provider.authStatus(context.Background(), request.EngineID, request.Method)
		if status.Authenticated {
			m.completeLogin(session, request, status)
			return
		}

		message := "CLI chưa xác thực được tài khoản; xem hướng dẫn bên dưới"
		if loginContext.Err() != nil {
			message = "Phiên đăng nhập đã hết hạn; hãy thử lại"
		}
		if err == nil {
			message = "CLI đã thoát nhưng chưa có phiên đăng nhập hợp lệ"
		}
		session.finish(false, message)
	}()
	return session.snapshot(), nil
}

func (m *Manager) LoginStatus(engineID string) LoginState {
	m.loginMutex.Lock()
	defer m.loginMutex.Unlock()
	if session := m.logins[engineID]; session != nil {
		return session.snapshot()
	}
	return LoginState{EngineID: engineID}
}

func (m *Manager) CancelLogin(engineID string) {
	m.loginMutex.Lock()
	session := m.logins[engineID]
	m.loginMutex.Unlock()
	if session == nil {
		return
	}
	session.finish(false, "Đã hủy phiên đăng nhập")
	if session.cancel != nil {
		session.cancel()
	}
}

func subscriptionEnvironment() []string {
	environment := make([]string, 0)
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if !secretEnvironmentName(name) {
			environment = append(environment, entry)
		}
	}
	return environment
}

func (m *Manager) completeLogin(session *loginSession, request LoginRequest, status AuthStatus) {
	session.complete.Do(func() {
		if !session.snapshot().Running {
			return
		}
		if err := m.saveCLIAuth(request.EngineID, CLIAuthConfig{Method: request.Method}); err != nil {
			session.finish(false, "Đã xác thực nhưng không lưu được phương thức")
			return
		}
		session.finish(true, status.Message)
	})
}
