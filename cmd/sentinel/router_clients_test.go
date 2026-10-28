package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nchungdev/home-sentinel/internal/adapters/nas/nastest"
)

const webContext = `{"client_id":"web-test","client_capabilities":["chat"]}`

type fakeTV struct {
	conn   *websocket.Conn
	frames chan map[string]any
}

func connectFakeTV(t *testing.T, sentinel *testSentinel) *fakeTV {
	t.Helper()
	url := "ws" + strings.TrimPrefix(sentinel.baseURL, "http") + "/ws/duinch"
	conn, _, err := websocket.DefaultDialer.Dial(url,
		http.Header{"Authorization": {"Bearer " + testTVToken}})
	if err != nil {
		t.Fatalf("connect fake TV: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	tv := &fakeTV{conn: conn, frames: make(chan map[string]any, 16)}
	go func() {
		for {
			var frame map[string]any
			if conn.ReadJSON(&frame) != nil {
				return
			}
			tv.frames <- frame
		}
	}()
	time.Sleep(100 * time.Millisecond)
	return tv
}

func (tv *fakeTV) actions(wait time.Duration) []string {
	var received []string
	deadline := time.After(wait)
	for {
		select {
		case frame := <-tv.frames:
			if frame["type"] == "action" {
				received = append(received, frame["action_type"].(string))
			}
		case <-deadline:
			return received
		}
	}
}

func askAs(t *testing.T, sentinel *testSentinel, requestContext, text string) (string, []string) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"request_id": "r", "session_id": "s",
		"text": text, "locale": "vi-VN", "input_type": "text"})
	request, _ := http.NewRequest(http.MethodPost, sentinel.baseURL+"/v2/ask",
		bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+testTVToken)
	if requestContext != "" {
		request.Header.Set("X-Request-Context", requestContext)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	payload, _ := io.ReadAll(response.Body)

	var clientActions []string
	for _, line := range bytes.Split(payload, []byte("\n")) {
		var event struct {
			Type    string         `json:"type"`
			Payload map[string]any `json:"payload"`
		}
		if json.Unmarshal(line, &event) == nil && event.Type == "client_action" {
			clientActions = append(clientActions, event.Payload["action_type"].(string))
		}
	}
	return speechFromNDJSON(t, payload), clientActions
}

func TestWebSearchDoesNotDriveTheTV(t *testing.T) {
	sentinel := startSentinel(t, testTVToken, nastest.DefaultNAS())
	tv := connectFakeTV(t, sentinel)

	speech, clientActions := askAs(t, sentinel, webContext, "tìm phim Naruto")

	if len(clientActions) != 0 {
		t.Fatalf("web received TV actions %v", clientActions)
	}
	if !strings.Contains(speech, "Chưa tra được thư viện Plex") ||
		strings.Contains(speech, "Không thấy") || strings.Contains(speech, "Đang mở") {
		t.Fatalf("speech = %q", speech)
	}
	if sent := tv.actions(300 * time.Millisecond); len(sent) != 0 {
		t.Fatalf("TV was driven by a web search: %v", sent)
	}
}

func TestWebCanExplicitlyTargetTheTV(t *testing.T) {
	sentinel := startSentinel(t, testTVToken, nastest.DefaultNAS())
	tv := connectFakeTV(t, sentinel)

	speech, clientActions := askAs(t, sentinel, webContext, "mở phim Naruto trên TV")

	if len(clientActions) != 0 || !strings.Contains(speech, "Đã gửi lệnh mở lên TV (1 TV)") {
		t.Fatalf("speech=%q actions=%v", speech, clientActions)
	}
	if sent := tv.actions(time.Second); len(sent) != 1 || sent[0] != "youtube.launch" {
		t.Fatalf("TV actions = %v", sent)
	}
}

func TestUndeclaredClientKeepsTVBehaviour(t *testing.T) {
	sentinel := startSentinel(t, testTVToken, nastest.DefaultNAS())

	_, clientActions := askAs(t, sentinel, "", "tìm phim Naruto")

	if len(clientActions) != 1 || clientActions[0] != "youtube.launch" {
		t.Fatalf("legacy TV lost its client action: %v", clientActions)
	}
}
