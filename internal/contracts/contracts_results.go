package contracts

import (
	"encoding/json"
	"strings"
)

func (p *ActionPlan) FromJSON(data []byte) error {
	return json.Unmarshal(data, p)
}

func (p *ActionPlan) FromJson(data []byte) error {
	return p.FromJSON(data)
}

func ActionPlanFromJSON(data []byte) (*ActionPlan, error) {
	var plan ActionPlan
	if err := plan.FromJSON(data); err != nil {
		return nil, err
	}
	return &plan, nil
}

type ExecutionResult struct {
	StepID      string         `json:"step_id"`
	Status      ActionStatus   `json:"status"`
	ErrorCode   *string        `json:"error_code,omitempty"`
	Detail      *string        `json:"detail,omitempty"`
	ActualState map[string]any `json:"actual_state,omitempty"`
	CanRetry    bool           `json:"can_retry"`
}

func (r *ExecutionResult) Validate() error {
	if strings.TrimSpace(r.StepID) == "" || !r.Status.IsValid() {
		return ErrInvalidExecutionResult
	}
	return nil
}

func (r *ExecutionResult) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}

func (r *ExecutionResult) ToJson() ([]byte, error) {
	return r.ToJSON()
}

func (r *ExecutionResult) FromJSON(data []byte) error {
	return json.Unmarshal(data, r)
}

func (r *ExecutionResult) FromJson(data []byte) error {
	return r.FromJSON(data)
}

func ExecutionResultFromJSON(data []byte) (*ExecutionResult, error) {
	var result ExecutionResult
	if err := result.FromJSON(data); err != nil {
		return nil, err
	}
	return &result, nil
}

type PresentationResponse struct {
	MediaSearch   *MediaSearchResponse `json:"media_search,omitempty"`
	SpeechText    string               `json:"speech_text"`
	Cards         []map[string]any     `json:"cards,omitempty"`
	QuickChips    []string             `json:"quick_chips,omitempty"`
	ClientActions []map[string]any     `json:"client_actions,omitempty"`
	Mood          string               `json:"mood"`
	Handler       *Handler             `json:"handler,omitempty"`
}

type Handler struct {
	Route  string `json:"route"`
	Intent string `json:"intent"`
	Engine string `json:"engine,omitempty"`
}

func (p *PresentationResponse) Validate() error {
	if strings.TrimSpace(p.SpeechText) == "" && len(p.Cards) == 0 && len(p.QuickChips) == 0 {
		return ErrInvalidPresentationResponse
	}
	return nil
}

func (p *PresentationResponse) ToJSON() ([]byte, error) {
	return json.Marshal(p)
}

func (p *PresentationResponse) ToJson() ([]byte, error) {
	return p.ToJSON()
}

func (p *PresentationResponse) FromJSON(data []byte) error {
	return json.Unmarshal(data, p)
}

func (p *PresentationResponse) FromJson(data []byte) error {
	return p.FromJSON(data)
}

func PresentationResponseFromJSON(data []byte) (*PresentationResponse, error) {
	var response PresentationResponse
	if err := response.FromJSON(data); err != nil {
		return nil, err
	}
	return &response, nil
}

type Suggestion struct {
	ID          string         `json:"id"`
	Kind        string         `json:"kind"`
	Title       string         `json:"title"`
	Message     string         `json:"message"`
	TargetID    string         `json:"target_id,omitempty"`
	Action      ActionStep     `json:"action"`
	Evidence    map[string]any `json:"evidence,omitempty"`
	GeneratedAt int64          `json:"generated_at"`
	ExpiresAt   int64          `json:"expires_at"`
	DedupeKey   string         `json:"dedupe_key,omitempty"`
}

func (s *Suggestion) Validate() error {
	if strings.TrimSpace(s.ID) == "" || strings.TrimSpace(s.Title) == "" {
		return ErrInvalidSuggestion
	}
	return nil
}

func (s *Suggestion) ToJSON() ([]byte, error) {
	return json.Marshal(s)
}

func (s *Suggestion) ToJson() ([]byte, error) {
	return s.ToJSON()
}

func (s *Suggestion) FromJSON(data []byte) error {
	return json.Unmarshal(data, s)
}

func (s *Suggestion) FromJson(data []byte) error {
	return s.FromJSON(data)
}

func SuggestionFromJSON(data []byte) (*Suggestion, error) {
	var suggestion Suggestion
	if err := suggestion.FromJSON(data); err != nil {
		return nil, err
	}
	return &suggestion, nil
}

type ContextSnapshot struct {
	TimestampEpochMs int64            `json:"timestamp_epoch_ms"`
	TimeOfDay        string           `json:"time_of_day"`
	ViewerProfile    *string          `json:"viewer_profile,omitempty"`
	Environment      map[string]any   `json:"environment,omitempty"`
	RecentMedia      []map[string]any `json:"recent_media,omitempty"`
}

func (c *ContextSnapshot) Validate() error {
	if c.TimestampEpochMs <= 0 {
		return ErrInvalidContextSnapshot
	}
	return nil
}

func (c *ContextSnapshot) ToJSON() ([]byte, error) {
	return json.Marshal(c)
}

func (c *ContextSnapshot) ToJson() ([]byte, error) {
	return c.ToJSON()
}

func (c *ContextSnapshot) FromJSON(data []byte) error {
	return json.Unmarshal(data, c)
}

func (c *ContextSnapshot) FromJson(data []byte) error {
	return c.FromJSON(data)
}

func ContextSnapshotFromJSON(data []byte) (*ContextSnapshot, error) {
	var snapshot ContextSnapshot
	if err := snapshot.FromJSON(data); err != nil {
		return nil, err
	}
	return &snapshot, nil
}

type ProactiveChip struct {
	ChipID        string         `json:"chip_id"`
	Label         string         `json:"label"`
	IconKey       string         `json:"icon_key,omitempty"`
	ActionPayload map[string]any `json:"action_payload,omitempty"`
}

func (c *ProactiveChip) Validate() error {
	if strings.TrimSpace(c.ChipID) == "" || strings.TrimSpace(c.Label) == "" {
		return ErrInvalidProactiveChip
	}
	return nil
}

func (c *ProactiveChip) ToJSON() ([]byte, error) {
	return json.Marshal(c)
}

func (c *ProactiveChip) ToJson() ([]byte, error) {
	return c.ToJSON()
}

func (c *ProactiveChip) FromJSON(data []byte) error {
	return json.Unmarshal(data, c)
}

func (c *ProactiveChip) FromJson(data []byte) error {
	return c.FromJSON(data)
}

func ProactiveChipFromJSON(data []byte) (*ProactiveChip, error) {
	var chip ProactiveChip
	if err := chip.FromJSON(data); err != nil {
		return nil, err
	}
	return &chip, nil
}
