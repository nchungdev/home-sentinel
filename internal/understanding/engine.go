package understanding

import (
	"context"
	"strings"

	"github.com/nchungdev/home-sentinel/internal/contracts"
	"github.com/nchungdev/home-sentinel/internal/orchestrator"
)

var _ orchestrator.UnderstandingEngine = (*Engine)(nil)

type Engine struct{}

func NewEngine() *Engine {
	return &Engine{}
}

func (e *Engine) Parse(
	ctx context.Context,
	input *contracts.InputEvent,
	reqCtx *contracts.RequestContext,
) (*contracts.Intent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if input == nil {
		return nil, contracts.ErrInvalidInputEvent
	}

	if err := input.Validate(); err != nil {
		return nil, err
	}

	normalizedInput := strings.Join(strings.Fields(input.Text), " ")
	if systemIntent := classifySystemStatus(normalizedInput); systemIntent != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return systemIntent, nil
	}

	if cameraIntent := classifyCameraControl(normalizedInput); cameraIntent != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return cameraIntent, nil
	}

	explicitSource, sourceStripped := ExtractExplicitSource(normalizedInput)
	phoneticNormalized := NormalizePhonetics(sourceStripped)

	if homeIntent := classifyHomeControl(phoneticNormalized); homeIntent != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return homeIntent, nil
	}

	if mediaIntent := classifyMediaSearch(phoneticNormalized, explicitSource); mediaIntent != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return mediaIntent, nil
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return classifyGeneralChat(phoneticNormalized), nil
}
