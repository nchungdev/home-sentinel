package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSettingsMigrateAndPreserveKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	legacy := `{"priority_order":["gemini","codex"],"gemini_key":"secret"}`
	if err := os.WriteFile(path, []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(path)
	if manager.Settings().SelectedEngine != "gemini" {
		t.Fatal("legacy selection not migrated")
	}
	if err := manager.SaveSettings(Settings{SelectedEngine: "codex"}); err != nil {
		t.Fatal(err)
	}
	settings := NewManager(path).Settings()
	if settings.SelectedEngine != "codex" || !settings.GeminiKeySet {
		t.Fatalf("settings not persisted: %+v", settings)
	}
	encoded, _ := json.Marshal(settings)
	if strings.Contains(string(encoded), "secret") {
		t.Fatal("settings exposed key")
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "priority_order") {
		t.Fatal("saved legacy priority list")
	}
}

func TestFailedSaveDoesNotChangeRuntime(t *testing.T) {
	manager := NewManager(filepath.Join(t.TempDir(), "missing", "config.json"))
	before := manager.Settings()
	settings := Settings{SelectedEngine: "gemini", GeminiKey: "new"}
	if err := manager.SaveSettings(settings); err == nil {
		t.Fatal("save unexpectedly succeeded")
	}
	if manager.Settings() != before {
		t.Fatal("failed save changed runtime settings")
	}
	if err := manager.SaveSettings(Settings{SelectedEngine: "unknown"}); err == nil {
		t.Fatal("invalid engine accepted")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestAPIConnectionUsesDraftWithoutSaving(t *testing.T) {
	for _, engineID := range []string{"openai_api", "anthropic_api"} {
		t.Run(engineID, func(t *testing.T) {
			manager := NewManager(filepath.Join(t.TempDir(), "config.json"))
			before := manager.Settings()
			calls := 0
			transport := func(request *http.Request) (*http.Response, error) {
				calls++
				if request.Method != http.MethodPost {
					t.Fatal("test did not send a model request")
				}
				var body map[string]any
				if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if body["model"] != "draft-model" {
					t.Fatal("draft model ignored")
				}
				response := `{"choices":[{"message":{"content":"OK"}}]}`
				if engineID == "anthropic_api" {
					if request.Header.Get("x-api-key") != "draft-key" {
						t.Fatal("missing Anthropic key")
					}
					if request.URL.Host != "api.anthropic.com" {
						t.Fatal("wrong provider")
					}
					response = `{"content":[{"type":"text","text":"OK"}]}`
				} else {
					if request.Header.Get("Authorization") != "Bearer draft-key" {
						t.Fatal("missing key")
					}
					if request.URL.Host != "api.openai.com" {
						t.Fatal("wrong provider")
					}
				}
				return &http.Response{
					StatusCode: 200, Body: io.NopCloser(strings.NewReader(response)),
				}, nil
			}
			manager.httpClient.Transport = roundTripFunc(transport)
			ok, message := manager.TestSettings(context.Background(), Settings{
				SelectedEngine: engineID, OpenAIKey: "draft-key", AnthropicKey: "draft-key",
				OpenAIModel: "draft-model", AnthropicModel: "draft-model",
			})
			if !ok || calls != 1 {
				t.Fatalf("test failed: %s, calls=%d", message, calls)
			}
			if manager.Settings() != before {
				t.Fatal("test changed saved settings")
			}
			if _, used := manager.LastUsage(); used {
				t.Fatal("test changed last chat usage")
			}
		})
	}
}

func TestUnknownSelectionNeverFallsBack(t *testing.T) {
	manager := &Manager{config: &Config{
		SelectedEngine: "unknown", PriorityOrder: []string{"rule_fallback"},
	}}
	err := manager.StreamChatContext(context.Background(), "hi", func(string) {
		t.Fatal("unknown selection fell back")
	})
	if err == nil {
		t.Fatal("unknown engine accepted")
	}
}

func TestLoginRejectsUnsupportedOrMissingCLI(t *testing.T) {
	manager := &Manager{config: &Config{CodexPath: filepath.Join(t.TempDir(), "missing")}}
	for _, engineID := range []string{"gemini", "codex"} {
		if _, err := manager.StartLogin(engineID); err == nil {
			t.Fatal("invalid login accepted")
		}
	}
	if manager.LoginStatus("codex").Running {
		t.Fatal("missing CLI reports running login")
	}
}

func TestSubscriptionEnvironmentExcludesAPIKeys(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "openai-secret")
	t.Setenv("ANTHROPIC_API_KEY", "anthropic-secret")
	for _, entry := range subscriptionEnvironment() {
		if strings.Contains(entry, "openai-secret") || strings.Contains(entry, "anthropic-secret") {
			t.Fatal("API key leaked into subscription command environment")
		}
	}
}
