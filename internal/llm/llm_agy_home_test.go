//go:build unix

package llm

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fakeAgyEnvironment = `#!/bin/sh
printf '{"event":"result","result":{"status":"SUCCESS","response":"home=%s pwd=%s"}}\n' \
  "$HOME" "$(pwd -P)"
`

func TestAgyRunsInIsolatedHome(t *testing.T) {
	configDir := t.TempDir()
	agyPath := filepath.Join(t.TempDir(), "agy")
	if err := os.WriteFile(agyPath, []byte(fakeAgyEnvironment), 0o700); err != nil {
		t.Fatal(err)
	}
	manager := &Manager{config: &Config{SelectedEngine: "agy", AgyPath: agyPath,
		ConfigPath: filepath.Join(configDir, "sentinel_llm_config.json")}}

	var response strings.Builder
	err := manager.StreamChatContext(context.Background(), "hi", func(chunk string) {
		response.WriteString(chunk)
	})
	if err != nil {
		t.Fatal(err)
	}

	home, _ := filepath.EvalSymlinks(filepath.Join(configDir, agyHomeDirName))
	if !strings.Contains(response.String(), "home="+filepath.Join(configDir, agyHomeDirName)) ||
		!strings.Contains(response.String(), "pwd="+filepath.Join(home, "workspace")) {
		t.Fatalf("agy was not isolated: %s", response.String())
	}

	data, err := os.ReadFile(filepath.Join(home, agySettingsDir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var settings struct {
		AllowNonWorkspaceAccess bool `json:"allowNonWorkspaceAccess"`
		Permissions             struct {
			Allow []string `json:"allow"`
		} `json:"permissions"`
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatal(err)
	}
	if settings.AllowNonWorkspaceAccess || len(settings.Permissions.Allow) != 0 {
		t.Fatalf("sandbox settings grant access: %s", data)
	}
	if _, err := os.Readlink(filepath.Join(home, agySettingsDir, agyTokenFileName)); err != nil {
		t.Fatalf("login token is not linked: %v", err)
	}
}

func TestAgyRefusesToRunWithoutSandbox(t *testing.T) {
	manager := &Manager{config: &Config{SelectedEngine: "agy", AgyPath: "/bin/echo"}}

	err := manager.StreamChatContext(context.Background(), "hi", func(string) {})

	if err == nil {
		t.Fatal("agy ran without an isolated home")
	}
}
