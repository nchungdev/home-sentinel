package llm

import (
	"context"
	"os"
	"strings"
	"time"
)

func (m *Manager) watchLogin(
	ctx context.Context, provider *Manager, session *loginSession, request LoginRequest,
) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	started := time.Now()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			state := session.snapshot()
			if state.LoginURL == "" && session.browserOutput != "" {
				if data, err := os.ReadFile(session.browserOutput); err == nil {
					_, _ = session.Write(data)
				}
			}
			if !state.Running {
				return
			}
			status := provider.authStatus(ctx, request.EngineID, request.Method)
			if status.Authenticated {
				m.completeLogin(session, request, status)
				session.cancel()
				return
			}

			if time.Since(started) > 20*time.Second && strings.TrimSpace(state.Output) == "" {
				session.finish(false, "CLI không trả hướng dẫn đăng nhập; kiểm tra CLI trên máy chủ")
				session.cancel()
				return
			}
		}
	}
}
