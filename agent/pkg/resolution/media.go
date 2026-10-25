package resolution

import (
	"context"
	"regexp"
	"strings"

	"github.com/nchungdev/home-sentinel/agent/pkg/contracts"
	"github.com/nchungdev/home-sentinel/agent/pkg/understanding"
)

var catalogPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^(có\s+)?phim\s+gì`),
	regexp.MustCompile(`(?i)^(có\s+)?phim\s+nào`),
	regexp.MustCompile(`(?i)^gợi\s+ý\s+phim`),
	regexp.MustCompile(`(?i)^danh\s+sách\s+phim`),
	regexp.MustCompile(`(?i)^xem\s+phim\s+gì`),
	regexp.MustCompile(`(?i)^có\s+(gì|phim\s+gì)\s+xem`),
	regexp.MustCompile(`(?i)^phim\s+mới`),
	regexp.MustCompile(`(?i)^(có\s+thể\s+xem\s+(gì|k|không))`),
}

func isCatalogQuery(query string) bool {
	q := strings.TrimSpace(strings.ToLower(query))
	if q == "" {
		return true
	}
	for _, p := range catalogPatterns {
		if p.MatchString(q) {
			return true
		}
	}
	return false
}

func (r *Resolver) createPlexCatalogPlan(
	intent *contracts.Intent,
	items []*PlexMediaItem,
) *contracts.ActionPlan {
	titles := make([]string, 0, len(items))
	for _, it := range items {
		titles = append(titles, it.Title)
	}

	evidence := map[string]any{
		"found_in_plex":   true,
		"is_catalog":      true,
		"titles":          titles,
		"explicit_source": understanding.SourcePlex,
	}

	return &contracts.ActionPlan{
		PlanID:               r.nextPlanID("media-plex-catalog"),
		IntentName:           intent.Name,
		Steps:                []contracts.ActionStep{},
		ConfirmationRequired: false,
		Evidence:             evidence,
	}
}

func isYouTubeKind(kind string) bool {
	switch kind {
	case "chương trình", "show", "nhạc", "bài hát", "ca khúc", "video", "clip":
		return true
	default:
		return false
	}
}

func (r *Resolver) createPlexPlan(
	intent *contracts.Intent,
	item *PlexMediaItem,
	isExplicit bool,
) *contracts.ActionPlan {
	evidence := map[string]any{
		"found_in_plex": true,
		"deep_link":     item.DeepLink,
		"rating_key":    item.RatingKey,
		"title":         item.Title,
		"year":          item.Year,
	}
	if isExplicit {
		evidence["explicit_source"] = understanding.SourcePlex
	}

	return &contracts.ActionPlan{
		PlanID:     r.nextPlanID("media-plex"),
		IntentName: intent.Name,
		Steps: []contracts.ActionStep{
			{
				StepID:     "step-1",
				ActionType: ActionTypePlexOpen,
				TargetID:   item.RatingKey,
				Parameters: map[string]any{
					"deep_link":  item.DeepLink,
					"rating_key": item.RatingKey,
					"title":      item.Title,
				},
				RunsOn:       RuntimeTVClient,
				TimeoutMs:    DefaultTimeoutMs,
				IsIdempotent: true,
			},
		},
		ConfirmationRequired: false,
		Evidence:             evidence,
	}
}

func (r *Resolver) createYouTubePlan(
	intent *contracts.Intent,
	query string,
	kind string,
	evidence map[string]any,
) *contracts.ActionPlan {
	parameters := map[string]any{
		"query": query,
	}
	if kind != "" {
		parameters["kind"] = kind
	}

	return &contracts.ActionPlan{
		PlanID:     r.nextPlanID("media-youtube"),
		IntentName: intent.Name,
		Steps: []contracts.ActionStep{
			{
				StepID:       "step-1",
				ActionType:   ActionTypeYouTubeLaunch,
				TargetID:     "youtube",
				Parameters:   parameters,
				RunsOn:       RuntimeTVClient,
				TimeoutMs:    DefaultTimeoutMs,
				IsIdempotent: true,
			},
		},
		ConfirmationRequired: false,
		Evidence:             evidence,
	}
}

func (r *Resolver) createEmptyMediaPlan(
	intent *contracts.Intent,
	query string,
	evidence map[string]any,
) *contracts.ActionPlan {
	evidence["query"] = query
	return &contracts.ActionPlan{
		PlanID:               r.nextPlanID("media-empty"),
		IntentName:           intent.Name,
		Steps:                []contracts.ActionStep{},
		ConfirmationRequired: false,
		Evidence:             evidence,
	}
}

func (r *Resolver) resolveMedia(
	ctx context.Context,
	intent *contracts.Intent,
	reqCtx *contracts.RequestContext,
) (*contracts.ActionPlan, error) {
	var query string
	if slotValue, exists := intent.Slots[understanding.SlotQuery].(string); exists {
		query = strings.TrimSpace(slotValue)
	}

	var kind string
	if slotValue, exists := intent.Slots[understanding.SlotKind].(string); exists {
		kind = strings.ToLower(strings.TrimSpace(slotValue))
	}

	explicitSource := strings.ToLower(strings.TrimSpace(intent.ExplicitSource))

	if explicitSource == understanding.SourceYouTube {
		return r.createYouTubePlan(intent, query, kind, map[string]any{
			"source":   understanding.SourceYouTube,
			"explicit": true,
		}), nil
	}

	if explicitSource == understanding.SourcePlex {
		if r.plexSearcher == nil {
			return r.createEmptyMediaPlan(intent, query, map[string]any{
				"found_in_plex":   false,
				"explicit_source": understanding.SourcePlex,
			}), nil
		}

		if err := ctx.Err(); err != nil {
			return nil, err
		}

		if isCatalogQuery(query) {
			items, recErr := r.plexSearcher.GetRecent(ctx, 5)
			if recErr != nil {
				if ctxErr := ctx.Err(); ctxErr != nil {
					return nil, ctxErr
				}
				return r.createEmptyMediaPlan(intent, query, map[string]any{
					"found_in_plex":   false,
					"explicit_source": understanding.SourcePlex,
					"is_catalog":      true,
					"error":           recErr.Error(),
				}), nil
			}
			if len(items) > 0 {
				return r.createPlexCatalogPlan(intent, items), nil
			}
			return r.createEmptyMediaPlan(intent, query, map[string]any{
				"found_in_plex":   false,
				"explicit_source": understanding.SourcePlex,
				"is_catalog":      true,
			}), nil
		}

		item, searchErr := r.plexSearcher.SearchFirst(ctx, query)
		if searchErr != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, ctxErr
			}
			return r.createEmptyMediaPlan(intent, query, map[string]any{
				"found_in_plex":   false,
				"explicit_source": understanding.SourcePlex,
				"error":           searchErr.Error(),
			}), nil
		}

		if item != nil {
			return r.createPlexPlan(intent, item, true), nil
		}

		return r.createEmptyMediaPlan(intent, query, map[string]any{
			"found_in_plex":   false,
			"explicit_source": understanding.SourcePlex,
			"not_found":       true,
		}), nil
	}

	if isYouTubeKind(kind) {
		return r.createYouTubePlan(intent, query, kind, map[string]any{
			"source": understanding.SourceYouTube,
			"kind":   kind,
		}), nil
	}

	if r.plexSearcher != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		item, searchErr := r.plexSearcher.SearchFirst(ctx, query)
		if searchErr != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, ctxErr
			}
		} else if item != nil {
			return r.createPlexPlan(intent, item, false), nil
		}
	}

	return r.createYouTubePlan(intent, query, kind, map[string]any{
		"found_in_plex":   false,
		"fallback_source": understanding.SourceYouTube,
		"query":           query,
	}), nil
}
