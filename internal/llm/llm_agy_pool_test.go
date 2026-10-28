//go:build unix

package llm

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

const fakeAgyDaemon = `#!/bin/sh
turn=0
total=0
printf '%s\n' '{"event":"init","init":{}}'
while IFS= read -r line; do
  turn=$((turn + 1))
  total=$((total + ${FAKE_TOKENS:-15000}))
  sleep "${FAKE_DELAY:-0}"
  sys=no; case "$line" in *"Home Sentinel"*) sys=yes;; esac
  carry=no; case "$line" in *"cuộc trò chuyện trước đó"*) carry=yes;; esac
  reply="pid=$$ turn=$turn sys=$sys carry=$carry"
  case "$line" in *"Chỉ trả về bản tóm tắt"*) reply="tóm tắt từ $$";; esac
  printf '{"event":"result","result":{"status":"SUCCESS","response":"%s",' "$reply"
  printf '"usage":{"input_tokens":%d}}}\n' "$total"
  if [ "${FAKE_EXIT_AFTER:-0}" = "$turn" ]; then exit 0; fi
done
`

var fakePIDPattern = regexp.MustCompile(`pid=(\d+)`)

func newDaemonManager(t *testing.T) *Manager {
	t.Helper()
	path := filepath.Join(t.TempDir(), "agy")
	if err := os.WriteFile(path, []byte(fakeAgyDaemon), 0o700); err != nil {
		t.Fatal(err)
	}
	manager := &Manager{config: &Config{
		SelectedEngine: "agy", AgyPath: path,
		ConfigPath: filepath.Join(t.TempDir(), "sentinel_llm_config.json"),
	}}
	manager.agy = newAgyPool(manager)
	t.Cleanup(func() {
		manager.agy.reset()
		manager.agy.wait()
	})
	return manager
}

func askAgy(t *testing.T, manager *Manager, conversationID string) string {
	t.Helper()
	ctx := WithConversation(context.Background(), conversationID)
	var response strings.Builder
	err := manager.StreamChatContext(ctx, "xin chào", func(chunk string) {
		response.WriteString(chunk)
	})
	if err != nil {
		t.Fatalf("ask %s: %v", conversationID, err)
	}
	return response.String()
}

func fakePID(t *testing.T, response string) string {
	t.Helper()
	match := fakePIDPattern.FindStringSubmatch(response)
	if match == nil {
		t.Fatalf("no pid in %q", response)
	}
	return match[1]
}

func waitUntil(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("condition not reached")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestDaemonReusesPrimedProcessAcrossTurns(t *testing.T) {
	manager := newDaemonManager(t)
	manager.agy.Prewarm()

	first := askAgy(t, manager, "tv")
	second := askAgy(t, manager, "tv")

	if fakePID(t, first) != fakePID(t, second) {
		t.Fatalf("expected one process, got %q and %q", first, second)
	}
	if !strings.Contains(first, "turn=2 sys=no") || !strings.Contains(second, "turn=3 sys=no") {
		t.Fatalf("primed process should not resend the system prompt: %q %q", first, second)
	}
}

func TestDaemonColdProcessGetsSystemPromptOnce(t *testing.T) {
	manager := newDaemonManager(t)

	first := askAgy(t, manager, "tv")
	second := askAgy(t, manager, "tv")

	if !strings.Contains(first, "turn=1 sys=yes") || !strings.Contains(second, "sys=no") {
		t.Fatalf("unexpected system prompt handling: %q %q", first, second)
	}
}

func TestDaemonSeparatesConversations(t *testing.T) {
	manager := newDaemonManager(t)

	tv := askAgy(t, manager, "tv")
	web := askAgy(t, manager, "web")

	if fakePID(t, tv) == fakePID(t, web) {
		t.Fatalf("tv and web share a process: %q %q", tv, web)
	}
}

func TestDaemonCompactsLargeContextIntoNewProcess(t *testing.T) {
	t.Setenv("FAKE_TOKENS", "45000")
	manager := newDaemonManager(t)

	first := askAgy(t, manager, "tv")
	conversation := manager.agy.conversation("tv")
	waitUntil(t, func() bool {
		conversation.mutex.Lock()
		defer conversation.mutex.Unlock()
		return conversation.carryOver != ""
	})
	second := askAgy(t, manager, "tv")

	if fakePID(t, first) == fakePID(t, second) {
		t.Fatalf("context over the limit was not rotated: %q %q", first, second)
	}
	if !strings.Contains(second, "carry=yes") {
		t.Fatalf("summary was not carried into the new process: %q", second)
	}
}

func TestDaemonReplacesExitedProcess(t *testing.T) {
	t.Setenv("FAKE_EXIT_AFTER", "1")
	manager := newDaemonManager(t)

	first := askAgy(t, manager, "tv")
	conversation := manager.agy.conversation("tv")
	waitUntil(t, func() bool {
		conversation.mutex.Lock()
		defer conversation.mutex.Unlock()
		return !conversation.process.alive()
	})
	second := askAgy(t, manager, "tv")

	if fakePID(t, first) == fakePID(t, second) {
		t.Fatalf("exited process was reused: %q %q", first, second)
	}
}

func TestDaemonCancellationDropsBusyProcess(t *testing.T) {
	t.Setenv("FAKE_DELAY", "2")
	manager := newDaemonManager(t)
	ctx, cancel := context.WithTimeout(
		WithConversation(context.Background(), "tv"), 200*time.Millisecond)
	defer cancel()

	err := manager.StreamChatContext(ctx, "xin chào", func(string) {})

	if err == nil {
		t.Fatal("expected cancellation error")
	}
	conversation := manager.agy.conversation("tv")
	conversation.mutex.Lock()
	defer conversation.mutex.Unlock()
	if conversation.process != nil {
		t.Fatal("cancelled turn left a desynchronised process in the conversation")
	}
}

func TestDaemonRestartsAfterSettingsChange(t *testing.T) {
	manager := newDaemonManager(t)
	first := askAgy(t, manager, "tv")

	manager.agy.reset()
	second := askAgy(t, manager, "tv")

	if fakePID(t, first) == fakePID(t, second) {
		t.Fatalf("reset kept the old process: %q %q", first, second)
	}
}
