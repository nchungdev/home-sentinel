package skills

import (
	"context"
	"net/url"
	"strings"

	"github.com/nchungdev/home-sentinel/internal/requestflow"
	websearch "github.com/nchungdev/home-sentinel/internal/search/web"
)

type DocumentationSearch struct {
	Searcher *websearch.Searcher
}

func (s DocumentationSearch) Execute(
	ctx context.Context, task requestflow.Task,
) requestflow.Result {
	hosts := documentationHosts(strings.ToLower(task.Query))
	if len(hosts) == 0 {
		return requestflow.Result{Status: "skipped", ErrorCode: "no_official_source_mapping",
			Cache: "bypassed", Items: []requestflow.Item{}}
	}
	clauses := make([]string, len(hosts))
	for index, host := range hosts {
		clauses[index] = "site:" + host
	}
	task.Query += " (" + strings.Join(clauses, " OR ") + ")"
	result := (WebSearch{Searcher: s.Searcher}).Execute(ctx, task)
	items := []requestflow.Item{}
	for _, item := range result.Items {
		parsed, err := url.Parse(item.URL)
		if err != nil {
			continue
		}
		for _, host := range hosts {
			if strings.EqualFold(parsed.Hostname(), host) {
				items = append(items, item)
				break
			}
		}
	}
	result.Items = items
	if result.Status == "ok" && len(items) == 0 {
		result.Status = "empty"
	}

	return result
}

func documentationHosts(query string) []string {
	for _, entry := range []struct {
		keyword string
		hosts   []string
	}{
		{"windows", []string{"learn.microsoft.com", "support.microsoft.com"}},
		{"docker", []string{"docs.docker.com"}},
		{"android", []string{"developer.android.com", "support.google.com"}},
		{"dolby", []string{"www.dolby.com", "professional.dolby.com"}},
		{"home assistant", []string{"www.home-assistant.io"}},
		{"redis", []string{"redis.io"}},
		{"python", []string{"docs.python.org"}},
	} {
		if strings.Contains(query, entry.keyword) {
			return entry.hosts
		}
	}

	return nil
}
