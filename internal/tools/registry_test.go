package tools

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"time"
)

var confirmationIDPattern = regexp.MustCompile(`confirmation_id="([0-9a-f]+)"`)

type recordedRuns struct{ calls []map[string]any }

func newTestRegistry() (*Registry, *recordedRuns) {
	runs := &recordedRuns{}
	registry := NewRegistry()
	run := func(ctx context.Context, call Call) (string, error) {
		runs.calls = append(runs.calls, call.Args)
		return "done", nil
	}
	registry.Register(Tool{Name: "read", Run: run})
	registry.Register(Tool{
		Name: "add_movie", RequiresConfirmation: true, Run: run,
		InputSchema: map[string]any{"properties": map[string]any{"title": map[string]any{}}},
		Summarize:   func(args map[string]any) string { return "thêm " + args["title"].(string) },
	})
	return registry, runs
}

func callTool(t *testing.T, registry *Registry, conversation, name, args string) (string, error) {
	t.Helper()
	return registry.Call(context.Background(), conversation, name, json.RawMessage(args))
}

func requestConfirmation(t *testing.T, registry *Registry, conversation string) string {
	t.Helper()
	registry.BeginTurn(conversation, "thêm phim Dune")
	reply, err := callTool(t, registry, conversation, "add_movie", `{"title":"Dune"}`)
	if err != nil {
		t.Fatal(err)
	}
	match := confirmationIDPattern.FindStringSubmatch(reply)
	if !strings.Contains(reply, "CẦN XÁC NHẬN: thêm Dune") || match == nil {
		t.Fatalf("expected a confirmation request, got %q", reply)
	}
	return match[1]
}

func withID(id string) string {
	return `{"title":"Dune","confirmation_id":"` + id + `"}`
}

func TestReadToolRunsDirectly(t *testing.T) {
	registry, runs := newTestRegistry()

	reply, err := callTool(t, registry, "tv", "read", `{"q":"x"}`)

	if err != nil || reply != "done" || len(runs.calls) != 1 {
		t.Fatalf("reply=%q err=%v runs=%v", reply, err, runs.calls)
	}
}

func TestUnknownToolIsError(t *testing.T) {
	registry, _ := newTestRegistry()
	if _, err := callTool(t, registry, "tv", "rm_rf", `{}`); err == nil {
		t.Fatal("expected error for unknown tool")
	}
}

func TestConfirmationRunsOnlyAfterUserAgreesNextTurn(t *testing.T) {
	registry, runs := newTestRegistry()
	id := requestConfirmation(t, registry, "tv")
	if len(runs.calls) != 0 {
		t.Fatal("tool ran before confirmation")
	}

	registry.BeginTurn("tv", "ok thêm đi")
	reply, err := callTool(t, registry, "tv", "add_movie", withID(id))

	if err != nil || reply != "done" || len(runs.calls) != 1 {
		t.Fatalf("reply=%q err=%v runs=%v", reply, err, runs.calls)
	}
	if _, present := runs.calls[0]["confirmation_id"]; present {
		t.Fatal("confirmation id leaked into tool arguments")
	}
}

func TestConfirmationCannotBeBypassed(t *testing.T) {
	cases := []struct {
		name  string
		setup func(registry *Registry, id string) (conversation, args string)
	}{
		{"same turn self confirmation", func(registry *Registry, id string) (string, string) {
			return "tv", withID(id)
		}},
		{"user said no", func(registry *Registry, id string) (string, string) {
			registry.BeginTurn("tv", "không, thôi")
			return "tv", withID(id)
		}},
		{"unrelated next message", func(registry *Registry, id string) (string, string) {
			registry.BeginTurn("tv", "thời tiết hôm nay")
			return "tv", withID(id)
		}},
		{"two turns later", func(registry *Registry, id string) (string, string) {
			registry.BeginTurn("tv", "ok")
			registry.BeginTurn("tv", "ok")
			return "tv", withID(id)
		}},
		{"changed arguments", func(registry *Registry, id string) (string, string) {
			registry.BeginTurn("tv", "ok")
			return "tv", `{"title":"Dune 2","confirmation_id":"` + id + `"}`
		}},
		{"other conversation", func(registry *Registry, id string) (string, string) {
			registry.BeginTurn("web", "ok")
			return "web", withID(id)
		}},
		{"made up id", func(registry *Registry, id string) (string, string) {
			registry.BeginTurn("tv", "ok")
			return "tv", withID("deadbeefdeadbeef")
		}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			registry, runs := newTestRegistry()
			id := requestConfirmation(t, registry, "tv")
			conversation, args := testCase.setup(registry, id)

			if _, err := callTool(t, registry, conversation, "add_movie", args); err == nil {
				t.Fatal("confirmation was bypassed")
			}
			if len(runs.calls) != 0 {
				t.Fatalf("tool ran: %v", runs.calls)
			}
		})
	}
}

func TestConfirmationIsSingleUseAndExpires(t *testing.T) {
	registry, runs := newTestRegistry()
	id := requestConfirmation(t, registry, "tv")
	registry.BeginTurn("tv", "đồng ý")
	if _, err := callTool(t, registry, "tv", "add_movie", withID(id)); err != nil {
		t.Fatal(err)
	}
	if _, err := callTool(t, registry, "tv", "add_movie", withID(id)); err == nil {
		t.Fatal("confirmation id was reused")
	}

	expired := requestConfirmation(t, registry, "tv")
	registry.confirmations.now = func() time.Time { return time.Now().Add(time.Hour) }
	registry.BeginTurn("tv", "ok")
	if _, err := callTool(t, registry, "tv", "add_movie", withID(expired)); err == nil {
		t.Fatal("expired confirmation was accepted")
	}
	if len(runs.calls) != 1 {
		t.Fatalf("expected one run, got %v", runs.calls)
	}
}

func TestAffirmativeReplies(t *testing.T) {
	accepted := []string{"ok", "Ok thêm đi", "Có", "dạ được", "đồng ý", "ừ", "yes"}
	rejected := []string{"không", "có lẽ không", "ok nhưng thôi", "hủy đi", "để sau", "coi thử"}

	for _, text := range accepted {
		if !affirmative(text) {
			t.Errorf("%q should count as agreement", text)
		}
	}
	for _, text := range rejected {
		if affirmative(text) {
			t.Errorf("%q should not count as agreement", text)
		}
	}
}

func TestConfirmationSchemaAddsField(t *testing.T) {
	registry, _ := newTestRegistry()
	for _, tool := range registry.List() {
		properties, _ := tool.InputSchema["properties"].(map[string]any)
		_, hasField := properties[confirmationField]
		if hasField != tool.RequiresConfirmation {
			t.Errorf("%s: confirmation field present=%v", tool.Name, hasField)
		}
	}
}
