package llm

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAGYExistingCredentialRequiresNoLogin(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)
	path := filepath.Join(homeDir, ".gemini", "antigravity-cli")
	if err := os.MkdirAll(path, 0700); err != nil {
		t.Fatal(err)
	}
	profile := map[string]any{"id_token_email": "test@example.test", "token": map[string]any{
		"access_token": "private-access", "expiry": time.Now().Add(time.Hour),
	}}
	data, _ := json.Marshal(profile)
	if err := os.WriteFile(filepath.Join(path, "antigravity-oauth-token"), data, 0600); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(filepath.Join(t.TempDir(), "config.json"))
	manager.config.AgyPath = filepath.Join(t.TempDir(), "does-not-exist")
	report, err := manager.AuthReport(context.Background(), "agy", "")
	if err != nil || !report.Status.Authenticated || report.SelectedMethod != "google" {
		t.Fatalf("existing credential not recognized: %+v, %v", report, err)
	}
	request := LoginRequest{EngineID: "agy", Method: "google"}
	state, err := manager.BeginLogin(context.Background(), request)
	if err != nil || !state.Succeeded || state.Running {
		t.Fatalf("login not skipped: %+v %v", state, err)
	}
	encoded, _ := json.Marshal(report)
	if strings.Contains(string(encoded), "private-access") {
		t.Fatal("auth report exposed token")
	}
}

func TestExpiredAGYCredentialWithoutRefreshRequiresLogin(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)
	path := filepath.Join(homeDir, ".gemini", "antigravity-cli")
	os.MkdirAll(path, 0700)
	profile := map[string]any{"token": map[string]any{
		"access_token": "old", "expiry": time.Now().Add(-time.Hour),
	}}
	data, _ := json.Marshal(profile)
	os.WriteFile(filepath.Join(path, "antigravity-oauth-token"), data, 0600)
	manager := NewManager(filepath.Join(t.TempDir(), "config.json"))
	if manager.agyAuthStatus(context.Background(), "google").Authenticated {
		t.Fatal("expired credential was reported authenticated")
	}
}

func TestAllDeclaredAuthMethodsHaveCorrectFlows(t *testing.T) {
	manager := &Manager{config: &Config{CodexPath: "codex", ClaudePath: "claude", AgyPath: "agy"}}
	for _, engineID := range []string{"codex", "claude", "agy"} {
		for _, method := range authMethods(engineID) {
			if _, err := findAuthMethod(engineID, method.ID); err != nil {
				t.Fatal(err)
			}
			if method.Kind != "browser" && engineID != "codex" {
				continue
			}
			request := LoginRequest{EngineID: engineID, Method: method.ID}
			path, args, interactive := manager.loginCommand(request)
			if path == "" || (len(args) == 0 && !interactive) {
				t.Fatalf("missing flow: %s/%s", engineID, method.ID)
			}
		}
	}
	if _, err := findAuthMethod("agy", "chatgpt"); err == nil {
		t.Fatal("cross-provider method accepted")
	}
}

func TestCLIAuthSecretIsPrivateAndScopedToMethod(t *testing.T) {
	manager := NewManager(filepath.Join(t.TempDir(), "config.json"))
	auth := CLIAuthConfig{Method: "api_key", Secret: "private-key"}
	if err := manager.saveCLIAuth("claude", auth); err != nil {
		t.Fatal(err)
	}
	provider := manager.providerSnapshot()
	apiEnvironment := strings.Join(provider.commandEnvironment("claude", "api_key"), "\n")
	if !strings.Contains(apiEnvironment, "private-key") {
		t.Fatal("credential was not provided to selected CLI method")
	}
	subscriptionEnvironment := strings.Join(
		provider.commandEnvironment("claude", "subscription"), "\n",
	)
	if strings.Contains(subscriptionEnvironment, "private-key") {
		t.Fatal("API credential overrode subscription")
	}
	settings, _ := json.Marshal(manager.Settings())
	if strings.Contains(string(settings), "private-key") {
		t.Fatal("settings exposed credential")
	}
	update := Settings{SelectedEngine: "claude", AuthMethod: "subscription"}
	if err := manager.SaveSettings(update); err != nil {
		t.Fatal(err)
	}
	if manager.providerSnapshot().config.CLIAuth["claude"].Secret != "" {
		t.Fatal("old credential was reused for a different method")
	}
}
