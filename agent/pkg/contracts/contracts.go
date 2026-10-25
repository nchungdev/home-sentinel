package contracts

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type ActionStatus string

const (
	ActionStatusAccepted   ActionStatus = "accepted"
	ActionStatusRunning    ActionStatus = "running"
	ActionStatusNeedsInput ActionStatus = "needs_input"
	ActionStatusSucceeded  ActionStatus = "succeeded"
	ActionStatusFailed     ActionStatus = "failed"
	ActionStatusCancelled  ActionStatus = "cancelled"
	ActionStatusUnknown    ActionStatus = "unknown"
)

var (
	ErrInvalidInputEvent           = errors.New("input event missing required request_id or text")
	ErrInvalidRequestContext       = errors.New("request context missing required client_id")
	ErrInvalidIntent               = errors.New("intent missing required name")
	ErrInvalidActionStep           = errors.New("action step missing step_id or action_type")
	ErrInvalidActionPlan           = errors.New("action plan missing required plan_id")
	ErrInvalidExecutionResult      = errors.New("execution result missing step_id or status")
	ErrInvalidPresentationResponse = errors.New("presentation response empty")
	ErrInvalidSuggestion           = errors.New("suggestion missing required id or title")
	ErrInvalidContextSnapshot      = errors.New("context snapshot invalid timestamp")
	ErrInvalidProactiveChip        = errors.New("proactive chip missing required chip_id or label")
)

func ParseActionStatus(rawStatus string) ActionStatus {
	normalized := strings.ToLower(strings.TrimSpace(rawStatus))
	switch ActionStatus(normalized) {
	case ActionStatusAccepted:
		return ActionStatusAccepted
	case ActionStatusRunning:
		return ActionStatusRunning
	case ActionStatusNeedsInput:
		return ActionStatusNeedsInput
	case ActionStatusSucceeded:
		return ActionStatusSucceeded
	case ActionStatusFailed:
		return ActionStatusFailed
	case ActionStatusCancelled:
		return ActionStatusCancelled
	case ActionStatusUnknown:
		return ActionStatusUnknown
	default:
		return ActionStatusUnknown
	}
}

func (s ActionStatus) IsValid() bool {
	switch s {
	case ActionStatusAccepted,
		ActionStatusRunning,
		ActionStatusNeedsInput,
		ActionStatusSucceeded,
		ActionStatusFailed,
		ActionStatusCancelled,
		ActionStatusUnknown:
		return true
	default:
		return false
	}
}

func (s *ActionStatus) UnmarshalJSON(data []byte) error {
	var rawString string
	if err := json.Unmarshal(data, &rawString); err != nil {
		return err
	}
	*s = ParseActionStatus(rawString)
	return nil
}

type InputEvent struct {
	RequestID    string   `json:"request_id"`
	SessionID    string   `json:"session_id"`
	UtteranceID  string   `json:"utterance_id,omitempty"`
	Text         string   `json:"text"`
	Alternatives []string `json:"alternatives,omitempty"`
	Locale       string   `json:"locale"`
	InputType    string   `json:"input_type"`
}

func (e *InputEvent) Validate() error {
	if strings.TrimSpace(e.RequestID) == "" || strings.TrimSpace(e.Text) == "" {
		return ErrInvalidInputEvent
	}
	return nil
}

func (e *InputEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

func (e *InputEvent) ToJson() ([]byte, error) {
	return e.ToJSON()
}

func (e *InputEvent) FromJSON(data []byte) error {
	return json.Unmarshal(data, e)
}

func (e *InputEvent) FromJson(data []byte) error {
	return e.FromJSON(data)
}

func InputEventFromJSON(data []byte) (*InputEvent, error) {
	var event InputEvent
	if err := event.FromJSON(data); err != nil {
		return nil, err
	}
	return &event, nil
}

type RequestContext struct {
	ClientID           string   `json:"client_id"`
	IsAuthenticated    bool     `json:"is_authenticated"`
	DeadlineEpochMs    int64    `json:"deadline_epoch_ms,omitempty"`
	ClientCapabilities []string `json:"client_capabilities,omitempty"`
	ViewerProfile      *string  `json:"viewer_profile,omitempty"`
}

func (c *RequestContext) Validate() error {
	if strings.TrimSpace(c.ClientID) == "" {
		return ErrInvalidRequestContext
	}
	return nil
}

func (c *RequestContext) ToJSON() ([]byte, error) {
	return json.Marshal(c)
}

func (c *RequestContext) ToJson() ([]byte, error) {
	return c.ToJSON()
}

func (c *RequestContext) FromJSON(data []byte) error {
	return json.Unmarshal(data, c)
}

func (c *RequestContext) FromJson(data []byte) error {
	return c.FromJSON(data)
}

func RequestContextFromJSON(data []byte) (*RequestContext, error) {
	var context RequestContext
	if err := context.FromJSON(data); err != nil {
		return nil, err
	}
	return &context, nil
}

type Intent struct {
	Name           string         `json:"name"`
	Domain         string         `json:"domain"`
	Confidence     float64        `json:"confidence"`
	Slots          map[string]any `json:"slots,omitempty"`
	ExplicitSource string         `json:"explicit_source,omitempty"`
	MissingSlots   []string       `json:"missing_slots,omitempty"`
}

func (i *Intent) Validate() error {
	if strings.TrimSpace(i.Name) == "" {
		return ErrInvalidIntent
	}
	return nil
}

func (i *Intent) ToJSON() ([]byte, error) {
	return json.Marshal(i)
}

func (i *Intent) ToJson() ([]byte, error) {
	return i.ToJSON()
}

func (i *Intent) FromJSON(data []byte) error {
	return json.Unmarshal(data, i)
}

func (i *Intent) FromJson(data []byte) error {
	return i.FromJSON(data)
}

func IntentFromJSON(data []byte) (*Intent, error) {
	var intent Intent
	if err := intent.FromJSON(data); err != nil {
		return nil, err
	}
	return &intent, nil
}

type ActionStep struct {
	StepID       string         `json:"step_id"`
	ActionType   string         `json:"action_type"`
	TargetID     string         `json:"target_id,omitempty"`
	Parameters   map[string]any `json:"parameters,omitempty"`
	RunsOn       string         `json:"runs_on"`
	TimeoutMs    int            `json:"timeout_ms"`
	IsIdempotent bool           `json:"is_idempotent"`
}

func (s *ActionStep) Validate() error {
	if strings.TrimSpace(s.StepID) == "" || strings.TrimSpace(s.ActionType) == "" {
		return ErrInvalidActionStep
	}
	return nil
}

func (s *ActionStep) ToJSON() ([]byte, error) {
	return json.Marshal(s)
}

func (s *ActionStep) ToJson() ([]byte, error) {
	return s.ToJSON()
}

func (s *ActionStep) FromJSON(data []byte) error {
	return json.Unmarshal(data, s)
}

func (s *ActionStep) FromJson(data []byte) error {
	return s.FromJSON(data)
}

func ActionStepFromJSON(data []byte) (*ActionStep, error) {
	var step ActionStep
	if err := step.FromJSON(data); err != nil {
		return nil, err
	}
	return &step, nil
}

type ActionPlan struct {
	PlanID               string         `json:"plan_id"`
	IntentName           string         `json:"intent_name"`
	Steps                []ActionStep   `json:"steps"`
	ConfirmationRequired bool           `json:"confirmation_required"`
	Evidence             map[string]any `json:"evidence,omitempty"`
}

func (p *ActionPlan) Validate() error {
	if strings.TrimSpace(p.PlanID) == "" {
		return ErrInvalidActionPlan
	}
	for i := range p.Steps {
		if err := p.Steps[i].Validate(); err != nil {
			return fmt.Errorf("invalid step at index %d: %w", i, err)
		}
	}
	return nil
}

func (p *ActionPlan) ToJSON() ([]byte, error) {
	return json.Marshal(p)
}

func (p *ActionPlan) ToJson() ([]byte, error) {
	return p.ToJSON()
}

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
	SpeechText    string           `json:"speech_text"`
	Cards         []map[string]any `json:"cards,omitempty"`
	QuickChips    []string         `json:"quick_chips,omitempty"`
	ClientActions []map[string]any `json:"client_actions,omitempty"`
	Mood          string           `json:"mood"`
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
