package resolution

import (
	"context"

	"github.com/nchungdev/home-sentinel/internal/contracts"
	"github.com/nchungdev/home-sentinel/internal/understanding"
)

func (r *Resolver) resolveCamera(
	ctx context.Context,
	intent *contracts.Intent,
	reqCtx *contracts.RequestContext,
) (*contracts.ActionPlan, error) {
	if privacyValue, exists := intent.Slots[understanding.SlotPrivacy].(string); exists &&
		privacyValue != "" {
		return &contracts.ActionPlan{
			PlanID:     r.nextPlanID("camera-privacy"),
			IntentName: intent.Name,
			Steps: []contracts.ActionStep{
				{
					StepID:     "step-1",
					ActionType: ActionTypeCameraPrivacy,
					TargetID:   "camera",
					Parameters: map[string]any{
						"enabled": privacyValue == understanding.PrivacyOn,
					},
					RunsOn:       RuntimeBackend,
					TimeoutMs:    DefaultTimeoutMs,
					IsIdempotent: true,
				},
			},
			ConfirmationRequired: false,
			Evidence: map[string]any{
				"privacy": privacyValue,
			},
		}, nil
	}

	if directionValue, exists := intent.Slots[understanding.SlotDirection].(string); exists &&
		directionValue != "" {
		return &contracts.ActionPlan{
			PlanID:     r.nextPlanID("camera-ptz"),
			IntentName: intent.Name,
			Steps: []contracts.ActionStep{
				{
					StepID:     "step-1",
					ActionType: ActionTypeCameraPTZ,
					TargetID:   "camera",
					Parameters: map[string]any{
						"direction": directionValue,
						"step":      10,
					},
					RunsOn:       RuntimeBackend,
					TimeoutMs:    DefaultTimeoutMs,
					IsIdempotent: false,
				},
			},
			ConfirmationRequired: false,
			Evidence: map[string]any{
				"direction": directionValue,
			},
		}, nil
	}

	if speakValue, exists := intent.Slots[understanding.SlotSpeak].(string); exists &&
		speakValue != "" {
		return &contracts.ActionPlan{
			PlanID:     r.nextPlanID("camera-speak"),
			IntentName: intent.Name,
			Steps: []contracts.ActionStep{
				{
					StepID:     "step-1",
					ActionType: ActionTypeCameraSpeak,
					TargetID:   "camera",
					Parameters: map[string]any{
						"text": speakValue,
					},
					RunsOn:       RuntimeBackend,
					TimeoutMs:    DefaultTimeoutMs,
					IsIdempotent: false,
				},
			},
			ConfirmationRequired: false,
			Evidence: map[string]any{
				"text": speakValue,
			},
		}, nil
	}

	if actionValue, exists := intent.Slots[understanding.SlotAction].(string); exists &&
		actionValue == understanding.ActionView {
		targetLabel := "camera"
		if labelValue, ok := intent.Slots[understanding.SlotTargetLabel].(string); ok &&
			labelValue != "" {
			targetLabel = labelValue
		}
		return &contracts.ActionPlan{
			PlanID:     r.nextPlanID("camera-view"),
			IntentName: intent.Name,
			Steps: []contracts.ActionStep{
				{
					StepID:     "step-1",
					ActionType: ActionTypeCameraView,
					TargetID:   targetLabel,
					Parameters: map[string]any{
						"target": targetLabel,
					},
					RunsOn:       RuntimeTVClient,
					TimeoutMs:    DefaultTimeoutMs,
					IsIdempotent: true,
				},
			},
			ConfirmationRequired: false,
			Evidence: map[string]any{
				"target": targetLabel,
			},
		}, nil
	}

	return &contracts.ActionPlan{
		PlanID:     r.nextPlanID("camera-status"),
		IntentName: intent.Name,
		Steps: []contracts.ActionStep{
			{
				StepID:       "step-1",
				ActionType:   ActionTypeCameraStatus,
				TargetID:     "camera",
				Parameters:   map[string]any{},
				RunsOn:       RuntimeBackend,
				TimeoutMs:    DefaultTimeoutMs,
				IsIdempotent: true,
			},
		},
		ConfirmationRequired: false,
		Evidence:             map[string]any{},
	}, nil
}
