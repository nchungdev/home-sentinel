package resolution

import (
	"context"
	"strings"

	"github.com/nchungdev/home-sentinel/internal/contracts"
	"github.com/nchungdev/home-sentinel/internal/understanding"
)

func (r *Resolver) resolveHome(
	ctx context.Context,
	intent *contracts.Intent,
	reqCtx *contracts.RequestContext,
) (*contracts.ActionPlan, error) {
	var targetLabel string
	if slotValue, exists := intent.Slots[understanding.SlotTargetLabel].(string); exists {
		targetLabel = strings.TrimSpace(slotValue)
	}

	var action string
	if slotValue, exists := intent.Slots[understanding.SlotAction].(string); exists {
		action = strings.TrimSpace(slotValue)
	}

	if r.haResolver == nil {
		return &contracts.ActionPlan{
			PlanID:     r.nextPlanID("ha-unresolved"),
			IntentName: intent.Name,
			Steps:      []contracts.ActionStep{},
			Evidence: map[string]any{
				"matched":      false,
				"target_label": targetLabel,
				"action":       action,
			},
		}, nil
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	match, resolveErr := r.haResolver.ResolveEntity(ctx, targetLabel, action)
	if resolveErr != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return &contracts.ActionPlan{
			PlanID:     r.nextPlanID("ha-unresolved"),
			IntentName: intent.Name,
			Steps:      []contracts.ActionStep{},
			Evidence: map[string]any{
				"matched":      false,
				"target_label": targetLabel,
				"action":       action,
				"error":        resolveErr.Error(),
			},
		}, nil
	}

	if match == nil {
		return &contracts.ActionPlan{
			PlanID:     r.nextPlanID("ha-unresolved"),
			IntentName: intent.Name,
			Steps:      []contracts.ActionStep{},
			Evidence: map[string]any{
				"matched":      false,
				"target_label": targetLabel,
				"action":       action,
			},
		}, nil
	}

	parameters := map[string]any{
		"entity_id": match.EntityID,
		"domain":    match.Domain,
		"service":   match.Service,
	}
	for key, value := range match.ServiceData {
		parameters[key] = value
	}
	if tempValue, exists := intent.Slots[understanding.SlotTemperature]; exists {
		parameters["temperature"] = tempValue
	}

	return &contracts.ActionPlan{
		PlanID:     r.nextPlanID("ha"),
		IntentName: intent.Name,
		Steps: []contracts.ActionStep{
			{
				StepID:       "step-1",
				ActionType:   ActionTypeHAServiceCall,
				TargetID:     match.EntityID,
				Parameters:   parameters,
				RunsOn:       RuntimeBackend,
				TimeoutMs:    DefaultTimeoutMs,
				IsIdempotent: true,
			},
		},
		ConfirmationRequired: false,
		Evidence: map[string]any{
			"matched":   true,
			"entity_id": match.EntityID,
			"domain":    match.Domain,
			"service":   match.Service,
		},
	}, nil
}
