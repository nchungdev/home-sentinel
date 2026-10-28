package requestflow

import "strconv"

func BuildPlan(understanding Understanding) Plan {
	plan := Plan{Understanding: understanding, Tasks: []Task{}}
	if understanding.NeedsInput {
		return plan
	}

	add := func(skill, source, strategy string) {
		if len(understanding.Sources) > 0 && !includesSource(understanding.Sources, source) {
			return
		}
		plan.Tasks = append(plan.Tasks, Task{
			ID: "t" + strconv.Itoa(len(plan.Tasks)+1), Skill: skill, Source: source,
			Query: taskQuery(understanding, skill), Strategy: strategy,
		})
	}
	switch understanding.Intent {
	case "inspect_url", "summarize_url":
		add("page.extract", "url", "api_embedded_json_html")
		if len(plan.Tasks) > 0 {
			plan.Tasks[0].Query = understanding.Entities["url"]
		}
	case "control":
		add("device.control", "home_assistant", "explicit_tool")
	case "chat":
		add("answer.compose", "model", "no_retrieval")
	case "request_library":
		add("library.check", "plex", "api")
		add("library.check", "jellyfin", "api")
		add("request.prepare", "radarr", "api")
		add("request.prepare", "sonarr", "api")
	case "find_to_watch":
		for _, source := range []string{"plex", "jellyfin"} {
			add("library.search", source, "api")
		}
		for _, source := range []string{"radarr", "sonarr", "phimapi", "ophim", "nguonc"} {
			add("catalog.search", source, "api")
		}
		for _, source := range []string{"motchill", "rophim", "phimtuoitho"} {
			add("catalog.search", source, "embedded_json_html")
		}
		add("video.search", "youtube", "api")
		add("web.search", "internet", "search_api")
	case "lookup":
		switch understanding.Domain {
		case "media":
			add("movie.metadata", "tmdb", "api")
		case "technical":
			add("documentation.search", "official_docs", "search_api")
		}
		add("web.search", "internet", "search_api")
	}

	return plan
}
