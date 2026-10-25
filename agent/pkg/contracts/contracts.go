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

