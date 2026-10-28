package nas

import (
	"context"
	"strings"

	"github.com/nchungdev/home-sentinel/internal/contracts"
)

const (
	TargetDisk   = "disk"
	TargetDocker = "docker"

	errorCodeProbeFailed = "NAS_PROBE_FAILED"
)

type Adapter struct {
	probe Probe
}

func NewAdapter(probe Probe) *Adapter {
	return &Adapter{probe: probe}
}

func (a *Adapter) Snapshot(ctx context.Context) Snapshot {
	return Collect(ctx, a.probe)
}

func (a *Adapter) Execute(
	ctx context.Context,
	step *contracts.ActionStep,
) (*contracts.ExecutionResult, error) {
	if step == nil {
		return nil, contracts.ErrInvalidActionStep
	}

	snapshot := a.Snapshot(ctx)
	sections := reportSections(step.TargetID)
	report, failed := composeReport(snapshot, sections)

	result := &contracts.ExecutionResult{
		StepID:   step.StepID,
		Status:   contracts.ActionStatusSucceeded,
		Detail:   &report,
		CanRetry: true,
	}
	if failed {
		code := errorCodeProbeFailed
		result.Status = contracts.ActionStatusFailed
		result.ErrorCode = &code
	}
	return result, nil
}

type sectionRenderer func(Snapshot) (string, bool)

func reportSections(target string) []sectionRenderer {
	switch strings.ToLower(strings.TrimSpace(target)) {
	case TargetDisk:
		return []sectionRenderer{renderDisks}
	case TargetDocker:
		return []sectionRenderer{renderContainers}
	default:
		return []sectionRenderer{renderHost, renderDisks, renderContainers}
	}
}

func composeReport(snapshot Snapshot, sections []sectionRenderer) (string, bool) {
	lines := make([]string, 0, len(sections))
	failures := 0
	for _, render := range sections {
		line, ok := render(snapshot)
		if !ok {
			failures++
		}
		if line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n"), failures == len(sections)
}
