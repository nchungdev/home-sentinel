package llm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
)

func cachedADC() (string, bool) {
	account, _, found := cachedADCProfile()
	return account, found
}

func cachedADCProfile() (string, string, bool) {
	paths := []string{os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")}
	directory := os.Getenv("CLOUDSDK_CONFIG")
	if directory == "" {
		if runtime.GOOS == "windows" {
			directory = filepath.Join(os.Getenv("APPDATA"), "gcloud")
		} else {
			homeDir, _ := os.UserHomeDir()
			directory = filepath.Join(homeDir, ".config", "gcloud")
		}
	}
	if paths[0] == "" {
		paths = []string{filepath.Join(directory, "application_default_credentials.json")}
	}
	for _, path := range paths {
		if path == "" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var credential struct {
			Type              string         `json:"type"`
			RefreshToken      string         `json:"refresh_token"`
			ClientID          string         `json:"client_id"`
			ClientSecret      string         `json:"client_secret"`
			ClientEmail       string         `json:"client_email"`
			PrivateKey        string         `json:"private_key"`
			Audience          string         `json:"audience"`
			CredentialSource  map[string]any `json:"credential_source"`
			SourceCredentials map[string]any `json:"source_credentials"`
			ImpersonationURL  string         `json:"service_account_impersonation_url"`
		}
		if json.Unmarshal(data, &credential) != nil {
			continue
		}
		switch credential.Type {
		case "authorized_user", "external_account_authorized_user":
			if credential.RefreshToken != "" && credential.ClientID != "" && credential.ClientSecret != "" {
				return "", path, true
			}
		case "service_account":
			if credential.ClientEmail != "" && credential.PrivateKey != "" {
				return credential.ClientEmail, path, true
			}
		case "external_account":
			if credential.Audience != "" && len(credential.CredentialSource) > 0 {
				return "", path, true
			}
		case "impersonated_service_account":
			if credential.ImpersonationURL != "" && len(credential.SourceCredentials) > 0 {
				return "", path, true
			}
		}
	}
	return "", "", false
}

func nativeADCEnabled(homeDir string) bool {
	if os.Getenv("AGY_ADC_AUTH") == "true" {
		return true
	}
	var config struct {
		UserSettings struct {
			EnableADC bool `json:"enableAdc"`
		} `json:"userSettings"`
	}
	data, err := os.ReadFile(filepath.Join(homeDir, ".gemini", "config", "config.json"))
	return err == nil && json.Unmarshal(data, &config) == nil && config.UserSettings.EnableADC
}
