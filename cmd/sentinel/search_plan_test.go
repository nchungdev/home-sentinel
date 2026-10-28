package main

import (
	"testing"

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
