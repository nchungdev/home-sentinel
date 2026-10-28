package watching

const (
	StatePlaying = "playing"
	StatePaused  = "paused"
	StateStopped = "stopped"
)

type Details struct {
	Kind    string `json:"kind,omitempty"`
	Year    int    `json:"year,omitempty"`
	Season  int    `json:"season,omitempty"`
	Episode int    `json:"episode,omitempty"`
	Channel string `json:"channel,omitempty"`
}

type Event struct {
	PackageName string `json:"package_name"`
	AppLabel    string `json:"app_label"`
	Title       string `json:"title"`
	Subtitle    string `json:"subtitle,omitempty"`
	MediaID     string `json:"media_id,omitempty"`
	PositionMs  int64  `json:"position_ms"`
	DurationMs  int64  `json:"duration_ms,omitempty"`
	State       string `json:"state"`
	ArtworkURL  string `json:"artwork_url,omitempty"`
	At          int64  `json:"at"`
	Details
}

func (e Event) key() string {
	return e.PackageName + "|" + e.Title
}

func (e Event) valid() bool {
	return e.PackageName != "" && e.Title != ""
}

type Action struct {
	ActionType string         `json:"action_type"`
	Parameters map[string]any `json:"parameters"`
}

type Item struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Subtitle    string  `json:"subtitle,omitempty"`
	AppLabel    string  `json:"app_label"`
	PackageName string  `json:"package_name"`
	Progress    float64 `json:"progress"`
	PosterURL   string  `json:"poster_url,omitempty"`
	RemainingMs int64   `json:"remaining_ms,omitempty"`
	Action      Action  `json:"action"`
	Details
	UpdatedAt int64 `json:"-"`
}
