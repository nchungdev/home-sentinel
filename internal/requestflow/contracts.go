package requestflow

import "context"

type Input struct {
	Text    string       `json:"text"`
	Context InputContext `json:"context,omitempty"`
}

type InputContext struct {
	Title   string   `json:"title,omitempty"`
	Sources []string `json:"sources,omitempty"`
}

type Understanding struct {
	Sources    []string          `json:"sources,omitempty"`
	Domain     string            `json:"domain"`
	Intent     string            `json:"intent"`
	Query      string            `json:"query"`
	Entities   map[string]string `json:"entities"`
	NeedsInput bool              `json:"needs_input"`
	Question   string            `json:"question,omitempty"`
	Method     string            `json:"method"`
}

type Classifier interface {
	Understand(context.Context, Input) (Understanding, error)
}

type Task struct {
	ID       string `json:"id"`
	Skill    string `json:"skill"`
	Source   string `json:"source"`
	Query    string `json:"query"`
	Strategy string `json:"strategy"`
}

type Plan struct {
	Understanding Understanding `json:"understanding"`
	Tasks         []Task        `json:"tasks"`
}

type Result struct {
	TaskID    string `json:"task_id"`
	Source    string `json:"source"`
	Status    string `json:"status"`
	ErrorCode string `json:"error_code,omitempty"`
	Cache     string `json:"cache"`
	Items     []Item `json:"items"`
}

type Item struct {
	ID      string       `json:"id"`
	Kind    string       `json:"kind"`
	Title   string       `json:"title"`
	URL     string       `json:"url,omitempty"`
	Content string       `json:"content,omitempty"`
	Poster  string       `json:"poster,omitempty"`
	Year    int          `json:"year,omitempty"`
	Season  int          `json:"season,omitempty"`
	Episode int          `json:"episode,omitempty"`
	Actions []ItemAction `json:"actions,omitempty"`
}

type ItemAction struct {
	Type     string `json:"type"`
	Label    string `json:"label"`
	URL      string `json:"url,omitempty"`
	SourceID string `json:"source_id,omitempty"`
	ItemID   string `json:"item_id,omitempty"`
}

type Event struct {
	Version   int     `json:"version"`
	RequestID string  `json:"request_id"`
	Sequence  int     `json:"sequence"`
	Type      string  `json:"type"`
	Status    string  `json:"status,omitempty"`
	Plan      *Plan   `json:"plan,omitempty"`
	Task      *Task   `json:"task,omitempty"`
	Result    *Result `json:"result,omitempty"`
}

type Skill interface {
	Execute(context.Context, Task) Result
}
