package contracts

import "context"

type MediaSearchEvent struct {
	Version    int                     `json:"version"`
	RequestID  string                  `json:"request_id"`
	Sequence   int                     `json:"sequence"`
	Status     string                  `json:"status,omitempty"`
	Sources    []MediaSourceCapability `json:"sources,omitempty"`
	Type       string                  `json:"type"`
	Query      string                  `json:"query,omitempty"`
	SourceID   string                  `json:"source_id,omitempty"`
	Capability *MediaSourceCapability  `json:"capability,omitempty"`
	Result     *MediaSourceResults     `json:"result,omitempty"`
	Snapshot   *MediaSearchResponse    `json:"-"`
}

type mediaSearchEmitterKey struct{}

func WithMediaSearchEmitter(ctx context.Context, emit func(MediaSearchEvent)) context.Context {
	return context.WithValue(ctx, mediaSearchEmitterKey{}, emit)
}

func EmitMediaSearch(ctx context.Context, event MediaSearchEvent) {
	if emit, ok := ctx.Value(mediaSearchEmitterKey{}).(func(MediaSearchEvent)); ok {
		if event.Snapshot != nil {
			snapshot := *event.Snapshot
			snapshot.Capabilities = append([]MediaSourceCapability(nil), snapshot.Capabilities...)
			snapshot.Sources = append([]MediaSourceResults(nil), snapshot.Sources...)
			event.Snapshot = &snapshot
		}
		emit(event)
	}
}
