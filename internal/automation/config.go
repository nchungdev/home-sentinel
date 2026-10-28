package automation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type Config struct {
	Rules          []Rule             `json:"rules,omitempty"`
	EveningArrival EveningArrivalRule `json:"evening_arrival"`
	IdleLivingRoom IdleLivingRoomRule `json:"idle_living_room"`
	SofaClapAction SofaClapRule       `json:"sofa_clap_action"`
	CustomRules    []CustomRule       `json:"custom_rules"`
}

type ConfigStore struct {
	path   string
	config Config
	mu     sync.RWMutex
}

func NewConfigStore(path string) *ConfigStore {
	store := &ConfigStore{
		path:   path,
		config: DefaultConfig(),
	}
	store.load()
	return store
}

func DefaultConfig() Config {
	return Config{
		EveningArrival: EveningArrivalRule{
			Enabled:      true,
			TargetLight:  "light.dqsmart_020119140c_1",
			TargetPreset: "3",
			CooldownSec:  300,
			StartHour:    18,
			EndHour:      6,
			SoundKeywords: []string{
				"mở cửa", "về rồi", "về nhà", "đã về", "cạch", "két", "hello", "chào",
			},
		},
		IdleLivingRoom: IdleLivingRoomRule{
			Enabled:     true,
			IdleMinutes: 1,
			TargetLights: []string{
				"light.dqsmart_0201bff7b7_1",
				"light.dqsmart_0201bff7b7_2",
				"light.dqsmart_0201bff7b7_4",
				"light.dqsmart_0201c007ce_1",
				"light.dqsmart_0201c007ce_2",
				"light.dqsmart_0201c007ce_4",
				"light.dqsmart_020119140c_1",
				"light.dqsmart_020119140c_2",
				"light.dqsmart_020119140c_3",
			},
			TurnOffScript:   "script.dqsmart_1792859129663",
			CheckTVPlayback: true,
		},
		SofaClapAction: SofaClapRule{
			Enabled:       true,
			TargetFan:     "fan.dmaker_sg_548577868_p39_s_2_fan",
			TargetTV:      "192.168.1.50",
			WindowSeconds: 60,
		},
	}
}

func (s *ConfigStore) load() {
	data, err := os.ReadFile(s.path)
	if err != nil {
		s.config.NormalizeRules()
		s.save()
		return
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err == nil {
		cfg.NormalizeRules()
		s.config = cfg
	}
}

func (s *ConfigStore) save() {
	if dir := filepath.Dir(s.path); dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0755)
	}
	data, err := json.MarshalIndent(s.config, "", "  ")
	if err == nil {
		_ = os.WriteFile(s.path, data, 0644)
	}
}

func (s *ConfigStore) Get() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

func (s *ConfigStore) Update(cfg Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cfg.NormalizeRules()
	cfg.SyncLegacy()
	s.config = cfg
	s.save()
}

func (c *Config) NormalizeRules() {
	if len(c.Rules) == 0 {
		c.buildDefaultRulesFromLegacy()
	}
	for i := range c.Rules {
		r := &c.Rules[i]
		if len(r.Triggers) == 0 && r.TriggerType != "" {
			r.Triggers = []TriggerItem{{
				Type:        r.TriggerType,
				IdleMinutes: r.IdleMinutes,
				Preset:      r.TriggerPreset,
			}}
		}
		if len(r.Conditions) == 0 {
			if r.TimeCondition == "range" {
				r.Conditions = append(r.Conditions, ConditionItem{
					Type: "time_range", StartHour: r.StartHour, EndHour: r.EndHour,
				})
			}
			if r.TVCondition == "off" || r.TVCondition == "on" {
				r.Conditions = append(r.Conditions, ConditionItem{
					Type: "tv_state", TVState: r.TVCondition,
				})
			}
		}
		if len(r.Actions) == 0 {
			if len(r.TargetEntities) > 0 || r.ActionType != "" {
				r.Actions = append(r.Actions, ActionItem{
					Type: "entities", Service: r.ActionType, TargetEntities: r.TargetEntities,
				})
			}
			if r.TargetTV != "" {
				r.Actions = append(r.Actions, ActionItem{Type: "tv", TargetTV: r.TargetTV})
			}
			if r.CameraPreset != "" {
				r.Actions = append(r.Actions, ActionItem{Type: "camera", CameraPreset: r.CameraPreset})
			}
			if r.ScriptID != "" {
				r.Actions = append(r.Actions, ActionItem{Type: "script", ScriptID: r.ScriptID})
			}
		}
	}
}

func (c *Config) buildDefaultRulesFromLegacy() {
	idleTV := "off"
	if !c.IdleLivingRoom.CheckTVPlayback {
		idleTV = "any"
	}
	c.Rules = append(c.Rules, Rule{
		ID: "idle", Name: "Tắt đèn khi vắng người", Icon: "bedtime",
		Description: "Tự động tắt các thiết bị khi phòng khách không có người và TV tắt.",
		Enabled: c.IdleLivingRoom.Enabled, TriggerType: "idle_room",
		IdleMinutes: c.IdleLivingRoom.IdleMinutes, TVCondition: idleTV,
		ActionType: "turn_off", TargetEntities: c.IdleLivingRoom.TargetLights,
		ScriptID: c.IdleLivingRoom.TurnOffScript,
	})

	clapTargets := c.SofaClapAction.TargetEntities
	if len(clapTargets) == 0 && c.SofaClapAction.TargetFan != "" {
		clapTargets = []string{c.SofaClapAction.TargetFan}
	}
	c.Rules = append(c.Rules, Rule{
		ID: "clap", Name: "Ngồi Sofa vỗ tay", Icon: "front_hand",
		Description: "Camera phát hiện người ở Sofa kết hợp tiếng vỗ tay sẽ bật thiết bị và mở TV.",
		Enabled: c.SofaClapAction.Enabled, TriggerType: "clap_sound",
		TriggerPreset: "1", CooldownSec: c.SofaClapAction.WindowSeconds,
		ActionType: "turn_on", TargetEntities: clapTargets,
		TargetTV: c.SofaClapAction.TargetTV, CameraPreset: "1",
	})

	eveTargets := c.EveningArrival.TargetEntities
	if len(eveTargets) == 0 && c.EveningArrival.TargetLight != "" {
		eveTargets = []string{c.EveningArrival.TargetLight}
	}
	c.Rules = append(c.Rules, Rule{
		ID: "evening", Name: "Phát hiện người buổi tối", Icon: "wb_twilight",
		Description: "Phát hiện có người vào khung giờ buổi tối sẽ tự động bật đèn đón và quay camera.",
		Enabled: c.EveningArrival.Enabled, TriggerType: "person_detected",
		TriggerPreset: c.EveningArrival.TargetPreset, TimeCondition: "range",
		StartHour: c.EveningArrival.StartHour, EndHour: c.EveningArrival.EndHour,
		ActionType: "turn_on", TargetEntities: eveTargets, CameraPreset: c.EveningArrival.TargetPreset,
	})

	c.Rules = append(c.Rules, Rule{
		ID: "turn_off_tv_idle", Name: "Tự động tắt TV khi vắng người", Icon: "tv_off",
		Description: "Tự động đưa Android TV vào chế độ ngủ khi không có người sau thời gian chờ.",
		Enabled: true,
		Triggers: []TriggerItem{{Type: "idle_room", IdleMinutes: 5}},
		Conditions: []ConditionItem{{Type: "tv_state", TVState: "on"}},
		Actions: []ActionItem{{Type: "tv", Service: "turn_off", TargetTV: "192.168.1.50"}},
	})
}

func (c *Config) SyncLegacy() {
	for i := range c.Rules {
		r := &c.Rules[i]
		if len(r.Triggers) > 0 {
			r.TriggerType = r.Triggers[0].Type
			r.IdleMinutes = r.Triggers[0].IdleMinutes
			r.TriggerPreset = r.Triggers[0].Preset
		}
		for _, a := range r.Actions {
			if a.Type == "entities" {
				r.ActionType = a.Service
				r.TargetEntities = a.TargetEntities
			} else if a.Type == "tv" {
				r.TargetTV = a.TargetTV
			} else if a.Type == "camera" {
				r.CameraPreset = a.CameraPreset
			} else if a.Type == "script" {
				r.ScriptID = a.ScriptID
			}
		}
		switch r.ID {
		case "idle":
			c.IdleLivingRoom.Enabled = r.Enabled
			c.IdleLivingRoom.IdleMinutes = r.IdleMinutes
			c.IdleLivingRoom.TargetLights = r.TargetEntities
			c.IdleLivingRoom.TurnOffScript = r.ScriptID
			c.IdleLivingRoom.CheckTVPlayback = r.TVCondition == "off"
		case "clap":
			c.SofaClapAction.Enabled = r.Enabled
			c.SofaClapAction.TargetEntities = r.TargetEntities
			if len(r.TargetEntities) > 0 {
				c.SofaClapAction.TargetFan = r.TargetEntities[0]
			}
			c.SofaClapAction.TargetTV = r.TargetTV
			c.SofaClapAction.WindowSeconds = r.CooldownSec
		case "evening":
			c.EveningArrival.Enabled = r.Enabled
			c.EveningArrival.TargetEntities = r.TargetEntities
			if len(r.TargetEntities) > 0 {
				c.EveningArrival.TargetLight = r.TargetEntities[0]
			}
			c.EveningArrival.TargetPreset = r.CameraPreset
			c.EveningArrival.StartHour = r.StartHour
			c.EveningArrival.EndHour = r.EndHour
		}
	}
}
