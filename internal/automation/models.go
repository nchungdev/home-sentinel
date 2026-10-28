package automation

type TriggerItem struct {
	Type        string `json:"type"` // "idle_room", "person_detected", "clap_sound", "time_schedule"
	IdleMinutes int    `json:"idle_minutes,omitempty"`
	Preset      string `json:"preset,omitempty"`
	Time        string `json:"time,omitempty"`
}

type ConditionItem struct {
	Type      string `json:"type"` // "time_range", "tv_state"
	StartHour int    `json:"start_hour,omitempty"`
	EndHour   int    `json:"end_hour,omitempty"`
	TVState   string `json:"tv_state,omitempty"` // "off", "on"
}

type ActionItem struct {
	Type           string   `json:"type"` // "entities", "tv", "camera", "script"
	Service        string   `json:"service,omitempty"` // "turn_off", "turn_on"
	TargetEntities []string `json:"target_entities,omitempty"`
	TargetTV       string   `json:"target_tv,omitempty"`
	CameraPreset   string   `json:"camera_preset,omitempty"`
	ScriptID       string   `json:"script_id,omitempty"`
}

type Rule struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Icon        string          `json:"icon,omitempty"`
	Enabled     bool            `json:"enabled"`
	Triggers    []TriggerItem   `json:"triggers,omitempty"`
	Conditions  []ConditionItem `json:"conditions,omitempty"`
	Actions     []ActionItem    `json:"actions,omitempty"`

	// Flat backward-compatibility fields:
	TriggerType    string   `json:"trigger_type,omitempty"`
	TriggerPreset  string   `json:"trigger_preset,omitempty"`
	IdleMinutes    int      `json:"idle_minutes,omitempty"`
	TimeCondition  string   `json:"time_condition,omitempty"`
	StartHour      int      `json:"start_hour,omitempty"`
	EndHour        int      `json:"end_hour,omitempty"`
	TVCondition    string   `json:"tv_condition,omitempty"`
	CooldownSec    int      `json:"cooldown_sec,omitempty"`
	ActionType     string   `json:"action_type,omitempty"`
	TargetEntities []string `json:"target_entities,omitempty"`
	TargetTV       string   `json:"target_tv,omitempty"`
	CameraPreset   string   `json:"camera_preset,omitempty"`
	ScriptID       string   `json:"script_id,omitempty"`
}

type CustomRule struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Enabled        bool     `json:"enabled"`
	TriggerType    string   `json:"trigger_type"`
	IdleMinutes    int      `json:"idle_minutes"`
	StartHour      int      `json:"start_hour"`
	EndHour        int      `json:"end_hour"`
	ActionType     string   `json:"action_type"`
	TargetEntities []string `json:"target_entities"`
	CameraPreset   string   `json:"camera_preset"`
	CheckTVOff     bool     `json:"check_tv_off"`
}

type SofaClapRule struct {
	Enabled        bool     `json:"enabled"`
	TargetFan      string   `json:"target_fan"`
	TargetEntities []string `json:"target_entities,omitempty"`
	TargetTV       string   `json:"target_tv"`
	WindowSeconds  int      `json:"window_seconds"`
}

type EveningArrivalRule struct {
	Enabled        bool     `json:"enabled"`
	TargetLight    string   `json:"target_light"`
	TargetEntities []string `json:"target_entities,omitempty"`
	TargetPreset   string   `json:"target_preset"`
	CooldownSec    int      `json:"cooldown_sec"`
	StartHour      int      `json:"start_hour"`
	EndHour        int      `json:"end_hour"`
	SoundKeywords  []string `json:"sound_keywords"`
}

type IdleLivingRoomRule struct {
	Enabled         bool     `json:"enabled"`
	IdleMinutes     int      `json:"idle_minutes"`
	TargetLights    []string `json:"target_lights"`
	TurnOffScript   string   `json:"turn_off_script"`
	CheckTVPlayback bool     `json:"check_tv_playback"`
}

type TVWelcomeGreetingRule struct {
	Enabled     bool   `json:"enabled"`
	CooldownSec int    `json:"cooldown_sec"`
	PromptText  string `json:"prompt_text"`
	PresetToken string `json:"preset_token"`
}
