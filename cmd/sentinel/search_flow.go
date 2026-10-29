package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/nchungdev/home-sentinel/internal/cache"
	"github.com/nchungdev/home-sentinel/internal/requestflow"
	websearch "github.com/nchungdev/home-sentinel/internal/search/web"
	"github.com/nchungdev/home-sentinel/internal/skills"
)

func (s *Server) searchRunner() requestflow.Runner {
	s.flowOnce.Do(func() {
		store, err := cache.FromEnvironment()
		if err != nil {
			log.Print("Invalid REDIS_URL; using bounded in-memory search cache")
		}
		web := skills.WebSearch{Searcher: websearch.New(os.Getenv("SEARXNG_URL"), store)}
		registry := requestflow.Registry{
			"web.search":           web,
			"documentation.search": skills.DocumentationSearch{Searcher: web.Searcher},
			"movie.metadata":       newMetadataSkill(),
		}
		if s.MediaSearch != nil {
			s.MediaSearch.SetCache(store)
			media := skills.MediaSearch{Service: s.MediaSearch}
			for _, name := range []string{"library.search", "catalog.search", "video.search"} {
				registry[name] = media
			}
		}
		s.flowRunner = requestflow.Runner{
			Classifier: requestflow.RuleClassifier{}, Skills: registry,
			FilterPlan: configuredSearchPlan(registry),
		}
	})

	return s.flowRunner
}

func runnablePlan(plan requestflow.Plan, registry requestflow.Registry) bool {
	for _, task := range plan.Tasks {
		if _, found := registry[task.Skill]; found {
			return true
		}
	}

	return false
}

func newMetadataSkill() skills.MovieMetadata {
	client := &http.Client{Timeout: 5 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}}
	return skills.MovieMetadata{Token: os.Getenv("TMDB_READ_ACCESS_TOKEN"),
		APIKey:  os.Getenv("TMDB_API_KEY"),
		BaseURL: "https://api.themoviedb.org/3", Client: client}
}

func configuredSearchPlan(registry requestflow.Registry) func(requestflow.Plan) requestflow.Plan {
	return func(plan requestflow.Plan) requestflow.Plan {
		if len(plan.Understanding.Sources) > 0 {
			return plan
		}
		tasks := []requestflow.Task{}
		for _, task := range plan.Tasks {
			skill, found := registry[task.Skill]
			if !found {
				continue
			}
			configured, hasConfiguration := skill.(interface{ Configured(requestflow.Task) bool })
			if hasConfiguration && !configured.Configured(task) {
				continue
			}
			tasks = append(tasks, task)
		}
		plan.Tasks = tasks
		return plan
	}
}
