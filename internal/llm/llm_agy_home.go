package llm

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

const (
	agyHomeDirName   = "agy-home"
	agySettingsDir   = ".gemini/antigravity-cli"
	agyTokenFileName = "antigravity-oauth-token"
)

var (
	agyCarriedSettings = []string{"model", "modelProvider"}
	agySandboxMutex    sync.Mutex
)

type agySandbox struct {
	home      string
	workspace string
}

func (m *Manager) agySandbox() (agySandbox, error) {
	if m.config.ConfigPath == "" {
		return agySandbox{}, errors.New("agy sandbox needs a config path")
	}
	agySandboxMutex.Lock()
	defer agySandboxMutex.Unlock()
	configPath, err := filepath.Abs(m.config.ConfigPath)
	if err != nil {
		return agySandbox{}, err
	}
	home := filepath.Join(filepath.Dir(configPath), agyHomeDirName)
	sandbox := agySandbox{home: home, workspace: filepath.Join(home, "workspace")}

	settingsDir := filepath.Join(home, agySettingsDir)
	for _, dir := range []string{settingsDir, sandbox.workspace} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return agySandbox{}, err
		}
	}
	userHome, err := os.UserHomeDir()
	if err != nil {
		return agySandbox{}, err
	}
	if err := linkAgyToken(userHome, settingsDir); err != nil {
		return agySandbox{}, err
	}
	return sandbox, writeAgySettings(userHome, settingsDir)
}

func linkAgyToken(userHome, settingsDir string) error {
	target := filepath.Join(userHome, agySettingsDir, agyTokenFileName)
	link := filepath.Join(settingsDir, agyTokenFileName)
	if current, err := os.Readlink(link); err == nil && current == target {
		return nil
	}
	if err := os.Remove(link); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return os.Symlink(target, link)
}

func writeAgySettings(userHome, settingsDir string) error {
	userSettings := map[string]any{}
	data, err := os.ReadFile(filepath.Join(userHome, agySettingsDir, "settings.json"))
	if err == nil {
		_ = json.Unmarshal(data, &userSettings)
	}

	settings := map[string]any{
		"allowNonWorkspaceAccess": false,
		"permissions":             map[string]any{"allow": []string{}},
	}
	for _, key := range agyCarriedSettings {
		if value, ok := userSettings[key]; ok {
			settings[key] = value
		}
	}
	data, err = json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(settingsDir, "settings.json"), data, 0o600)
}

func (m *Manager) isolateAgy(cmd *exec.Cmd) error {
	sandbox, err := m.agySandbox()
	if err != nil {
		return err
	}
	cmd.Dir = sandbox.workspace
	cmd.Env = append(cmd.Env, "HOME="+sandbox.home)
	return nil
}
