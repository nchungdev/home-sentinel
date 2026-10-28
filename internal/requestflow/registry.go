package requestflow

import "context"

type Registry map[string]Skill

type PlaceholderSkill struct{}

func (PlaceholderSkill) Execute(ctx context.Context, task Task) Result {
	result := Result{
		TaskID: task.ID, Source: task.Source, Status: "skipped",
		ErrorCode: "not_implemented", Cache: "bypassed", Items: []Item{},
	}
	if ctx.Err() != nil {
		result.Status, result.ErrorCode = "cancelled", "cancelled"
	}

	return result
}

func PlaceholderRegistry() Registry {
	registry := Registry{}
	for _, name := range []string{
		"page.extract", "device.control", "answer.compose", "library.check",
		"request.prepare", "library.search", "catalog.search", "video.search",
		"web.search", "movie.metadata", "documentation.search",
	} {
		registry[name] = PlaceholderSkill{}
	}

	return registry
}
