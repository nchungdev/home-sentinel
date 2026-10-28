//go:build unix

package llm

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const agyInit = `printf '%s\n' '{"event":"init","init":{}}'`

func fakeAgy(t *testing.T, body string) *Manager {
	t.Helper()
	path := filepath.Join(t.TempDir(), "agy")
	script := "#!/bin/sh\n" + agyInit + "\n" + body + "\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return &Manager{config: &Config{
		SelectedEngine: "agy", AgyPath: path,
		ConfigPath: filepath.Join(t.TempDir(), "sentinel_llm_config.json"),
	}}
}

func chatAgy(ctx context.Context, manager *Manager) (string, time.Duration, error) {
	started := time.Now()
	var response strings.Builder
	err := manager.StreamChatContext(ctx, "hello", func(chunk string) {
		response.WriteString(chunk)
	})
	return response.String(), time.Since(started), err
}

func TestAgyAnswersWithoutWaitingForExit(t *testing.T) {
	manager := fakeAgy(t, `printf '%s\n' '{"event":"result","result":`+
		`{"status":"SUCCESS","response":"Xin chào\n"}}'`+"\nsleep 3")

	response, elapsed, err := chatAgy(context.Background(), manager)

	if err != nil {
		t.Fatal(err)
	}
	if response != "Xin chào\n" {
		t.Fatalf("unexpected response %q", response)
	}
	if elapsed > time.Second {
		t.Fatalf("waited %v for agy to exit after its result", elapsed)
	}
	if usage, used := manager.LastUsage(); !used || usage.EngineID != "agy" {
		t.Fatalf("expected agy usage, got %+v", usage)
	}
}

func TestAgyFailedResultIsError(t *testing.T) {
	manager := fakeAgy(t, `printf '%s\n' '{"event":"result","result":{"status":"ERROR"}}'`)

	if _, _, err := chatAgy(context.Background(), manager); err == nil {
		t.Fatal("expected error for failed agy result")
	}
}

func TestAgyWithoutResultIsError(t *testing.T) {
	manager := fakeAgy(t, "exit 1")

	if _, _, err := chatAgy(context.Background(), manager); err == nil {
		t.Fatal("expected error when agy exits without a result")
	}
}

func TestAgyCancellationStopsProcess(t *testing.T) {
	manager := fakeAgy(t, "sleep 10")
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_, elapsed, err := chatAgy(ctx, manager)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("cancellation took %v", elapsed)
	}
}
