package requestflow

import (
	"context"
	"crypto/rand"
	"time"
)

type Runner struct {
	Classifier Classifier
	Skills     Registry
	FilterPlan func(Plan) Plan
}

func PlaceholderRunner() Runner {
	return Runner{Classifier: RuleClassifier{}, Skills: PlaceholderRegistry()}
}

func (r Runner) Run(ctx context.Context, input Input, emit func(Event)) error {
	requestID, sequence := rand.Text(), 0
	send := func(event Event) {
		sequence++
		event.Version, event.RequestID, event.Sequence = 1, requestID, sequence
		emit(event)
	}
	understanding, err := r.Classifier.Understand(ctx, input)
	if err != nil {
		return err
	}
	plan := BuildPlan(understanding)
	if r.FilterPlan != nil {
		plan = r.FilterPlan(plan)
	}
	send(Event{Type: "plan_ready", Plan: &plan})
	if understanding.NeedsInput {
		send(Event{Type: "response_completed", Status: "needs_input"})
		return nil
	}

	updates := make(chan Event, len(plan.Tasks)*2)
	workers := make(chan struct{}, 4)
	for _, task := range plan.Tasks {
		go func() {
			select {
			case workers <- struct{}{}:
				defer func() { <-workers }()
			case <-ctx.Done():
				return
			}
			updates <- Event{Type: "task_started", Task: &task}
			taskContext, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			skill, found := r.Skills[task.Skill]
			if !found {
				result := Result{TaskID: task.ID, Source: task.Source, Status: "skipped",
					ErrorCode: "skill_unavailable", Cache: "bypassed", Items: []Item{}}
				updates <- Event{Type: "task_result", Result: &result}
				return
			}
			result := skill.Execute(taskContext, task)
			if taskContext.Err() == context.DeadlineExceeded {
				result.Status, result.ErrorCode = "error", "timeout"
			}
			result.TaskID, result.Source = task.ID, task.Source
			updates <- Event{Type: "task_result", Result: &result}
		}()
	}

	succeeded := 0
	for completed := 0; completed < len(plan.Tasks); {
		select {
		case event := <-updates:
			if event.Result != nil {
				completed++
				if event.Result.Status == "ok" || event.Result.Status == "empty" {
					succeeded++
				}
			}
			send(event)
		case <-ctx.Done():
			send(Event{Type: "response_completed", Status: "cancelled"})
			return ctx.Err()
		}
	}
	status := "unavailable"
	if succeeded > 0 {
		status = "partial"
	}
	if succeeded == len(plan.Tasks) && succeeded > 0 {
		status = "completed"
	}
	if ctx.Err() != nil {
		status = "cancelled"
	}
	send(Event{Type: "response_completed", Status: status})

	return nil
}
