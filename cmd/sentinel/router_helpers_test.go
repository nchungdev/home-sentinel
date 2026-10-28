package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/nchungdev/home-sentinel/internal/adapters/nas"
	"github.com/nchungdev/home-sentinel/internal/adapters/nas/nastest"
)

const testTVToken = "tv-secret-token"

type testSentinel struct {
	server  *Server
	baseURL string
	dir     string
}

func startSentinel(t *testing.T, token string, fake *nastest.FakeNAS) *testSentinel {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("AGENT_TOKEN", "")
	t.Setenv("AGENT_ALLOW", "")

	server := NewServer()
	server.Auth = newAuthGate(token, nil, filepath.Join(dir, authStoreFile))
	server.NAS = nas.NewAdapter(fake.Start(t))

	httpServer := httptest.NewServer(newRouter(server))
	t.Cleanup(httpServer.Close)
	return &testSentinel{server: server, baseURL: httpServer.URL, dir: dir}
}

func (s *testSentinel) request(
	t *testing.T,
	client *http.Client,
	method, path, bearer string,
	body any,
) (*http.Response, []byte) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("encode body: %v", err)
		}
		reader = bytes.NewReader(encoded)
	}

	request, err := http.NewRequest(method, s.baseURL+path, reader)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		request.Header.Set("Authorization", "Bearer "+bearer)
	}
	if client == nil {
		client = http.DefaultClient
	}

	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer response.Body.Close()
	payload, _ := io.ReadAll(response.Body)
	return response, payload
}

func (s *testSentinel) status(t *testing.T, method, path, bearer string) int {
	t.Helper()
	response, _ := s.request(t, nil, method, path, bearer, nil)
	return response.StatusCode
}

func (s *testSentinel) ask(t *testing.T, bearer, text string) string {
	t.Helper()
	body := map[string]string{
		"request_id": "req-test",
		"session_id": "sess-test",
		"text":       text,
		"locale":     "vi-VN",
		"input_type": "text",
	}
	response, payload := s.request(t, nil, http.MethodPost, "/v2/ask", bearer, body)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("ask %q: status %d %s", text, response.StatusCode, payload)
	}
	return speechFromNDJSON(t, payload)
}

func speechFromNDJSON(t *testing.T, payload []byte) string {
	t.Helper()
	scanner := bufio.NewScanner(bytes.NewReader(payload))
	for scanner.Scan() {
		var event struct {
			Type    string `json:"type"`
			Payload struct {
				SpeechText string `json:"speech_text"`
			} `json:"payload"`
		}
		if json.Unmarshal(scanner.Bytes(), &event) == nil && event.Type == "presentation" {
			return event.Payload.SpeechText
		}
	}
	t.Fatalf("no presentation event in stream:\n%s", payload)
	return ""
}

func newBrowser(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	return &http.Client{Jar: jar}
}
