package main

import (
	"context"
	"testing"

	"github.com/nchungdev/home-sentinel/internal/contracts"
	"github.com/nchungdev/home-sentinel/internal/requestflow"
	websearch "github.com/nchungdev/home-sentinel/internal/search/web"
	"github.com/nchungdev/home-sentinel/internal/skills"
)

func TestLivePlanOmitsUnconfiguredSources(t *testing.T) {
	registry := requestflow.Registry{
		"web.search":     skills.WebSearch{Searcher: websearch.New("", nil)},
		"catalog.search": skills.MediaSearch{},
	}
	plan := requestflow.BuildPlan(requestflow.Understanding{
		Domain: "media", Intent: "find_to_watch", Entities: map[string]string{},
	})
	filtered := configuredSearchPlan(registry)(plan)
	if len(filtered.Tasks) != 0 {
		t.Fatalf("unconfigured or unavailable sources leaked into live plan: %+v", filtered.Tasks)
	}
}

func TestLivePlanKeepsExplicitPhimAPISource(t *testing.T) {
	registry := requestflow.Registry{
		"catalog.search": skills.MediaSearch{Service: configuredMediaSource{"phimapi"}},
	}
	plan := requestflow.BuildPlan(requestflow.Understanding{
		Domain: "media", Intent: "find_to_watch", Sources: []string{"phimapi"},
		Entities: map[string]string{},
	})
	filtered := configuredSearchPlan(registry)(plan)
	if len(filtered.Tasks) != 1 || filtered.Tasks[0].Source != "phimapi" {
		t.Fatalf("configured catalog source was not retained: %+v", filtered.Tasks)
	}
}

func TestExplicitSourceStillExplainsConfigurationErrors(t *testing.T) {
	plan := requestflow.BuildPlan(requestflow.Understanding{
		Domain: "media", Intent: "find_to_watch", Sources: []string{"plex"},
		Entities: map[string]string{},
	})
	filtered := configuredSearchPlan(requestflow.Registry{})(plan)
	if len(filtered.Tasks) != 1 || filtered.Tasks[0].Source != "plex" {
		t.Fatal("explicit source request lost its failure explanation")
	}
}

type configuredMediaSource struct{ source string }

func (s configuredMediaSource) SearchSource(
	_ context.Context, _, _ string,
) contracts.MediaSourceResults {
	return contracts.MediaSourceResults{}
}

func (s configuredMediaSource) Configured(source string) bool { return source == s.source }
