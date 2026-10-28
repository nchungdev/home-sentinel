//go:build unix

package llm

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fakeCodex(t *testing.T, script string) *Manager {
	t.Helper()
	path := filepath.Join(t.TempDir(), "codex")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0700); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(filepath.Join(t.TempDir(), "config.json"))
	manager.config.CodexPath = path
	return manager
}

func TestExistingCodexSessionSkipsLoginCommand(t *testing.T) {
	manager := fakeCodex(t, `if [ "$2" = status ]; then
 echo 'Logged in using ChatGPT' >&2
 exit 0
fi
exit 42
`)
	report, err := manager.AuthReport(context.Background(), "codex", "")
	if err != nil || !report.Status.Authenticated || report.SelectedMethod != "chatgpt" {
		t.Fatalf("existing auth not detected: %+v %v", report, err)
	}
	request := LoginRequest{EngineID: "codex", Method: "chatgpt"}
	state, err := manager.BeginLogin(context.Background(), request)
	if err != nil || state.Running || !state.Succeeded {
		t.Fatalf("login not skipped: %+v %v", state, err)
	}
}

func TestExitZeroDoesNotMeanAuthenticated(t *testing.T) {
	manager := fakeCodex(t, "if [ \"$2\" = status ]; then exit 1; fi\nexit 0\n")
	if _, err := manager.BeginLogin(context.Background(), LoginRequest{
		EngineID: "codex", Method: "chatgpt",
	}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		state := manager.LoginStatus("codex")
		if !state.Running {
			if state.Succeeded || !strings.Contains(state.Message, "chưa có phiên") {
				t.Fatalf("false success: %+v", state)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("login did not complete")
}

func TestCancelLoginKeepsCancelledState(t *testing.T) {
	manager := fakeCodex(t, "if [ \"$2\" = status ]; then exit 1; fi\nexec sleep 30\n")
	if _, err := manager.BeginLogin(context.Background(), LoginRequest{
		EngineID: "codex", Method: "chatgpt",
	}); err != nil {
		t.Fatal(err)
	}
	manager.CancelLogin("codex")
	time.Sleep(100 * time.Millisecond)
	state := manager.LoginStatus("codex")
	if state.Running || state.Succeeded || state.Message != "Đã hủy phiên đăng nhập" {
		t.Fatalf("cancellation was overwritten: %+v", state)
	}
}
