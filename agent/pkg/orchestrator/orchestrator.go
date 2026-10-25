package orchestrator

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/nchungdev/home-sentinel/agent/pkg/contracts"
	"github.com/nchungdev/home-sentinel/agent/pkg/telemetry"
)

var (
	ErrNilEngine = errors.New("orchestrator engine dependency cannot be nil")
)

type UnderstandingEngine interface {
	Parse(
		ctx context.Context,
		input *contracts.InputEvent,
		reqCtx *contracts.RequestContext,
	) (*contracts.Intent, error)
}

type ResolutionEngine interface {
	Resolve(
		ctx context.Context,
		intent *contracts.Intent,
		reqCtx *contracts.RequestContext,
	) (*contracts.ActionPlan, error)
}

type ExecutionEngine interface {
	Execute(
		ctx context.Context,
		step *contracts.ActionStep,
		reqCtx *contracts.RequestContext,
	) (*contracts.ExecutionResult, error)
}

type PresentationComposer interface {
	Compose(
		ctx context.Context,
		input *contracts.InputEvent,
		intent *contracts.Intent,
		plan *contracts.ActionPlan,
		results []*contracts.ExecutionResult,
		reqCtx *contracts.RequestContext,
	) (*contracts.PresentationResponse, error)
}

type Orchestrator struct {
	understanding UnderstandingEngine
	resolution    ResolutionEngine
	execution     ExecutionEngine
	presentation  PresentationComposer
}

func NewOrchestrator(
	understanding UnderstandingEngine,
	resolution ResolutionEngine,
	execution ExecutionEngine,
	presentation PresentationComposer,
) *Orchestrator {
	return &Orchestrator{
		understanding: understanding,
		resolution:    resolution,
		execution:     execution,
		presentation:  presentation,
	}
}

func (o *Orchestrator) Process(
	ctx context.Context,
	input *contracts.InputEvent,
	reqCtx *contracts.RequestContext,
) (*contracts.PresentationResponse, error) {
	if o.understanding == nil || o.resolution == nil ||
		o.execution == nil || o.presentation == nil {
		return nil, ErrNilEngine
	}

	if input == nil {
		return nil, contracts.ErrInvalidInputEvent
	}

	if err := input.Validate(); err != nil {
		return nil, err
	}

	if reqCtx == nil {
		reqCtx = &contracts.RequestContext{ClientID: "tv-client"}
	}

	trace := telemetry.NewRequestTrace(input.RequestID, input.SessionID)
	defer func() {
		trace.Finish()
		_ = trace.LogStructured(os.Stdout)
	}()

	if err := ctx.Err(); err != nil {
		trace.Record("lifecycle", 0, "cancelled", err.Error())
		return nil, err
	}

	startUnderstanding := time.Now()
	intent, err := o.understanding.Parse(ctx, input, reqCtx)
	if err != nil {
		trace.Record("understanding", time.Since(startUnderstanding), "failed", err.Error())
		return nil, err
	}
	trace.Record("understanding", time.Since(startUnderstanding), "ok", intent.Name)

	if err := ctx.Err(); err != nil {
		trace.Record("lifecycle", 0, "cancelled", err.Error())
		return nil, err
	}

	startResolution := time.Now()
	plan, err := o.resolution.Resolve(ctx, intent, reqCtx)
	if err != nil {
		trace.Record("resolution", time.Since(startResolution), "failed", err.Error())
		return nil, err
	}
	trace.Record("resolution", time.Since(startResolution), "ok", plan.PlanID)

	if err := ctx.Err(); err != nil {
		trace.Record("lifecycle", 0, "cancelled", err.Error())
		return nil, err
	}

	startExecution := time.Now()
	var results []*contracts.ExecutionResult
	if plan != nil && len(plan.Steps) > 0 {
		for stepIndex := range plan.Steps {
			if err := ctx.Err(); err != nil {
				trace.Record("execution", time.Since(startExecution), "cancelled", err.Error())
				return nil, err
			}

			step := &plan.Steps[stepIndex]
			execResult, execErr := o.execution.Execute(ctx, step, reqCtx)
			if execErr != nil {
				if execResult == nil {
					errDetail := execErr.Error()
					execResult = &contracts.ExecutionResult{
						StepID:   step.StepID,
						Status:   contracts.ActionStatusFailed,
						Detail:   &errDetail,
						CanRetry: false,
					}
				}
				results = append(results, execResult)
				break
			}

			if execResult != nil {
				results = append(results, execResult)
				if execResult.Status == contracts.ActionStatusFailed && !execResult.CanRetry {
					break
				}
			}
		}
	}
	trace.Record("execution", time.Since(startExecution), "ok", plan.PlanID)

	if err := ctx.Err(); err != nil {
		trace.Record("lifecycle", 0, "cancelled", err.Error())
		return nil, err
	}

	startPresentation := time.Now()
	pres, err := o.presentation.Compose(ctx, input, intent, plan, results, reqCtx)
	if err != nil {
		trace.Record("presentation", time.Since(startPresentation), "failed", err.Error())
		return nil, err
	}
	trace.Record("presentation", time.Since(startPresentation), "ok", pres.Mood)
	return pres, nil
}
