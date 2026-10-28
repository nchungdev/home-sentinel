package requestflow

import (
	"context"
	"testing"
)

func TestPlansRouteByPurpose(t *testing.T) {
	tests := []struct {
		text   string
		intent string
		skills map[string]bool
	}{
		{"Ai đạo diễn Interstellar?", "lookup",
			map[string]bool{"movie.metadata": true, "web.search": true}},
		{"Tìm Interstellar để xem", "find_to_watch",
			map[string]bool{"library.search": true, "catalog.search": true,
				"video.search": true, "web.search": true}},
		{"Tìm cách sửa lỗi Windows", "lookup",
			map[string]bool{"documentation.search": true, "web.search": true}},
		{"Tóm tắt https://example.com/article", "summarize_url",
			map[string]bool{"page.extract": true}},
		{"Thêm phim Interstellar vào thư viện", "request_library",
			map[string]bool{"library.check": true, "request.prepare": true}},
		{"Interstellar", "clarify", map[string]bool{}},
		{"xin chào", "chat", map[string]bool{"answer.compose": true}},
	}
	for _, test := range tests {
		t.Run(test.text, func(t *testing.T) {
			understanding, err := (RuleClassifier{}).Understand(
				context.Background(), Input{Text: test.text},
			)
			if err != nil || understanding.Intent != test.intent {
				t.Fatalf("unexpected classification: %+v, %v", understanding, err)
			}
			plan := BuildPlan(understanding)
			found := map[string]bool{}
			for _, task := range plan.Tasks {
				found[task.Skill] = true
				if !test.skills[task.Skill] {
					t.Fatalf("unrelated skill scheduled: %+v", task)
				}
			}
			for skill := range test.skills {
				if !found[skill] {
					t.Fatalf("missing skill: %s", skill)
				}
			}
		})
	}
}

func TestPlaceholderNeverClaimsLiveResults(t *testing.T) {
	var events []Event
	err := PlaceholderRunner().Run(context.Background(), Input{Text: "tìm phim"},
		func(event Event) { events = append(events, event) })
	if err != nil {
		t.Fatal(err)
	}
	if len(events[0].Plan.Tasks) != 12 {
		t.Fatal("missing media source placeholders")
	}
	started, finished := map[string]bool{}, map[string]bool{}
	for index, event := range events {
		if event.Sequence != index+1 || event.RequestID != events[0].RequestID {
			t.Fatalf("broken event ordering: %+v", event)
		}
		if event.Task != nil {
			started[event.Task.ID] = true
		}
		if event.Result != nil {
			result := event.Result
			if !started[result.TaskID] || finished[result.TaskID] {
				t.Fatal("result must follow one task start")
			}
			finished[result.TaskID] = true
			if result.Status != "skipped" || result.ErrorCode != "not_implemented" ||
				result.Cache != "bypassed" || len(result.Items) != 0 {
				t.Fatalf("placeholder fabricated data: %+v", result)
			}
		}
	}
	if len(finished) != 12 || events[len(events)-1].Status != "unavailable" {
		t.Fatal("placeholder claimed successful completion")
	}
}

type skillFunc func(context.Context, Task) Result

func (f skillFunc) Execute(ctx context.Context, task Task) Result { return f(ctx, task) }

func TestStreamsResultsBeforeAllTasksFinish(t *testing.T) {
	release := make(chan struct{})
	runner := PlaceholderRunner()
	runner.Skills["movie.metadata"] = skillFunc(func(context.Context, Task) Result {
		return Result{Status: "ok", Items: []Item{{ID: "movie"}}}
	})
	runner.Skills["web.search"] = skillFunc(func(ctx context.Context, _ Task) Result {
		select {
		case <-release:
		case <-ctx.Done():
			t.Error("fast result was not delivered before slow task timed out")
		}
		return Result{Status: "error", ErrorCode: "provider_unavailable"}
	})
	lastStatus := ""
	err := runner.Run(context.Background(), Input{
		Text: "ai đạo diễn phim này", Context: InputContext{Title: "Interstellar"},
	}, func(event Event) {
		if event.Result != nil && event.Result.Status == "ok" {
			close(release)
		}
		if event.Type == "response_completed" {
			lastStatus = event.Status
		}
	})
	if err != nil || lastStatus != "partial" {
		t.Fatalf("failed source erased successful result: %s, %v", lastStatus, err)
	}
}

func TestAmbiguousInputDoesNotExecuteSkills(t *testing.T) {
	runner := PlaceholderRunner()
	runner.Skills = nil
	var events []Event
	_ = runner.Run(context.Background(), Input{Text: "Interstellar"}, func(event Event) {
		events = append(events, event)
	})
	if len(events) != 2 || events[1].Status != "needs_input" {
		t.Fatalf("ambiguous input triggered retrieval: %+v", events)
	}
}
