package llm

import (
	"bytes"
	"io"
	"testing"
)

func TestLoginURLExtractsOnlyProviderAuthorization(t *testing.T) {
	cases := []struct{ output, want string }{
		{"Open https://auth.openai.com/codex/device now", "https://auth.openai.com/codex/device"},
		{"https://claude.ai/oauth/authorize?state=test", "https://claude.ai/oauth/authorize?state=test"},
		{"https://claude.com/cai/oauth/authorize?state=test",
			"https://claude.com/cai/oauth/authorize?state=test"},
		{"https://accounts.google.com/o/oauth2/auth?state=test",
			"https://accounts.google.com/o/oauth2/auth?state=test"},
		{"https://example.com/oauth/authorize", ""},
		{"https://auth.openai.com.evil.test/oauth", ""},
		{"https://user:secret@auth.openai.com/oauth", ""},
		{"https://antigravity.google/docs/install", ""},
	}
	for _, test := range cases {
		if got := loginURL(test.output); got != test.want {
			t.Errorf("URL = %q, want %q", got, test.want)
		}
	}
}

type loginInputBuffer struct{ bytes.Buffer }

func (b *loginInputBuffer) Close() error { return nil }

func TestLoginCodeOnlyGoesToRunningSession(t *testing.T) {
	input := &loginInputBuffer{}
	manager := &Manager{logins: map[string]*loginSession{
		"agy": {input: input, state: LoginState{Running: true, LoginURL: "https://accounts.google.com/o/oauth2/auth"}},
	}}
	if err := manager.SubmitLoginCode("agy", "test-code"); err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(input)
	if string(data) != "test-code\n" {
		t.Fatal("code was not delivered")
	}
	if err := manager.SubmitLoginCode("agy", "code\nother"); err == nil {
		t.Fatal("multiline code accepted")
	}
	if err := manager.SubmitLoginCode("claude", "code"); err == nil {
		t.Fatal("code accepted without session")
	}
	manager.logins["agy"].state.Running = false
	if err := manager.SubmitLoginCode("agy", "code"); err == nil {
		t.Fatal("code accepted for ended session")
	}
}
