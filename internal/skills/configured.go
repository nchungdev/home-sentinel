package skills

import (
	"github.com/nchungdev/home-sentinel/internal/requestflow"
	websearch "github.com/nchungdev/home-sentinel/internal/search/web"
	"strings"
)

func (s MediaSearch) Configured(task requestflow.Task) bool {
	configured, ok := s.Service.(interface{ Configured(string) bool })
	return ok && configured.Configured(task.Source)
}

func (s WebSearch) Configured(requestflow.Task) bool {
	return s.Searcher != nil && websearch.ValidURL(s.Searcher.BaseURL)
}

func (s DocumentationSearch) Configured(task requestflow.Task) bool {
	return (WebSearch{Searcher: s.Searcher}).Configured(task) &&
		len(documentationHosts(strings.ToLower(task.Query))) > 0
}

func (s MovieMetadata) Configured(requestflow.Task) bool {
	return s.BaseURL != "" && (s.Token != "" || s.APIKey != "")
}
