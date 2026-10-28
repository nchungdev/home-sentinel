package llm

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestExistingADCNeedsNoGcloudLogin(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "credential.json")
	credential := `{"type":"service_account","client_email":"test@example.test","private_key":"fake"}`
	if err := os.WriteFile(path, []byte(credential), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", path)
	t.Setenv("HOME", directory)
	t.Setenv("USERPROFILE", directory)
	manager := NewManager(filepath.Join(directory, "config.json"))
	status := manager.agyAuthStatus(context.Background(), "adc")
	if !status.Authenticated || status.Method != "adc" {
		t.Fatalf("ADC not reused: %+v", status)
	}
	request := LoginRequest{EngineID: "agy", Method: "adc"}
	state, err := manager.BeginLogin(context.Background(), request)
	if err != nil || state.Running || !state.Succeeded {
		t.Fatalf("ADC login not skipped: %+v %v", state, err)
	}
}

func TestInvalidExplicitADCCannotFallBackToAnotherCredential(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("CLOUDSDK_CONFIG", directory)
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", filepath.Join(directory, "missing.json"))
	data := `{"type":"service_account","client_email":"test@example.test","private_key":"fake"}`
	if err := os.WriteFile(filepath.Join(directory, "application_default_credentials.json"),
		[]byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	if _, configured := cachedADC(); configured {
		t.Fatal("invalid explicit credential was ignored")
	}
}
