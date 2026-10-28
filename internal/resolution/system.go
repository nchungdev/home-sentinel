package resolution

import (
	"context"

	"github.com/nchungdev/home-sentinel/internal/contracts"
	"github.com/nchungdev/home-sentinel/internal/understanding"
)

func (r *Resolver) resolveSystem(
	ctx context.Context,
	intent *contracts.Intent,
	reqCtx *contracts.RequestContext,
) (*contracts.ActionPlan, error) {
	target := "system"
	if targetValue, exists := intent.Slots[understanding.SlotTarget].(string); exists &&
		targetValue != "" {
		target = targetValue
	}

	return &contracts.ActionPlan{
		PlanID:     r.nextPlanID("system-status"),
		IntentName: intent.Name,
		Steps: []contracts.ActionStep{
			{
				StepID:     "step-1",
				ActionType: ActionTypeSystemStatus,
				TargetID:   target,
				Parameters: map[string]any{
					"target": target,
				},
				RunsOn:       RuntimeBackend,
				TimeoutMs:    DefaultTimeoutMs,
				IsIdempotent: true,
			},
		},
		ConfirmationRequired: false,
		Evidence: map[string]any{
			"target": target,
		},
	}, nil
}

func (r *Resolver) resolveGeneral(
	ctx context.Context,
	intent *contracts.Intent,
	reqCtx *contracts.RequestContext,
) (*contracts.ActionPlan, error) {
	text := ""
	if textValue, exists := intent.Slots[understanding.SlotText].(string); exists {
		text = textValue
	}

	return &contracts.ActionPlan{
		PlanID:     r.nextPlanID("general-chat"),
		IntentName: intent.Name,
		Steps: []contracts.ActionStep{
			{
				StepID:     "step-1",
				ActionType: ActionTypeLLMChat,
				TargetID:   "llm",
				Parameters: map[string]any{
					"text": text,
				},
				RunsOn:       RuntimeBackend,
				TimeoutMs:    10000,
				IsIdempotent: true,
			},
		},
		ConfirmationRequired: false,
		Evidence: map[string]any{
			"text": text,
		},
	}, nil
}

func (r *Resolver) resolveUnknown(
	ctx context.Context,
	intent *contracts.Intent,
	reqCtx *contracts.RequestContext,
) (*contracts.ActionPlan, error) {
	return &contracts.ActionPlan{
		PlanID:               r.nextPlanID("unknown"),
		IntentName:           intent.Name,
		Steps:                []contracts.ActionStep{},
		ConfirmationRequired: false,
		Evidence: map[string]any{
			"unknown_intent": true,
		},
	}, nil
}
