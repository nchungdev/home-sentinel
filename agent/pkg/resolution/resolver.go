package resolution

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/nchungdev/home-sentinel/agent/pkg/contracts"
	"github.com/nchungdev/home-sentinel/agent/pkg/understanding"
)

type Resolver struct {
	plexSearcher PlexSearcher
	haResolver   HAEntityResolver
	idCounter    uint64
}

func NewResolver(plexSearcher PlexSearcher, haResolver HAEntityResolver) *Resolver {
	return &Resolver{
		plexSearcher: plexSearcher,
		haResolver:   haResolver,
	}
}

func (r *Resolver) nextPlanID(prefix string) string {
	sequence := atomic.AddUint64(&r.idCounter, 1)
	return fmt.Sprintf("plan-%s-%d-%d", prefix, time.Now().UnixNano(), sequence)
}

func (r *Resolver) Resolve(
	ctx context.Context,
	intent *contracts.Intent,
	reqCtx *contracts.RequestContext,
) (*contracts.ActionPlan, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if intent == nil {
		return nil, contracts.ErrInvalidIntent
	}

	if err := intent.Validate(); err != nil {
		return nil, err
	}

	if reqCtx == nil {
		reqCtx = &contracts.RequestContext{ClientID: "tv-client"}
	}

	switch intent.Name {
	case understanding.IntentMediaSearch:
		return r.resolveMedia(ctx, intent, reqCtx)
	case understanding.IntentHomeControl:
		return r.resolveHome(ctx, intent, reqCtx)
	case understanding.IntentCameraControl:
		return r.resolveCamera(ctx, intent, reqCtx)
	case understanding.IntentSystemStatus:
		return r.resolveSystem(ctx, intent, reqCtx)
	case understanding.IntentGeneralChat:
		return r.resolveGeneral(ctx, intent, reqCtx)
	default:
		return r.resolveUnknown(ctx, intent, reqCtx)
	}
}
