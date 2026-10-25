package automation

import (
	"encoding/json"
	"os"
	"sync"
)

type Config struct {
	EveningArrival EveningArrivalRule `json:"evening_arrival"`
	IdleLivingRoom IdleLivingRoomRule `json:"idle_living_room"`
	SofaClapAction SofaClapRule       `json:"sofa_clap_action"`
}

type SofaClapRule struct {
	Enabled       bool   `json:"enabled"`
	TargetFan     string `json:"target_fan"`
	TargetTV      string `json:"target_tv"`
	WindowSeconds int    `json:"window_seconds"`
}

type EveningArrivalRule struct {
	Enabled       bool     `json:"enabled"`
	TargetLight   string   `json:"target_light"`
	TargetPreset  string   `json:"target_preset"`
	CooldownSec   int      `json:"cooldown_sec"`
	StartHour     int      `json:"start_hour"`
	EndHour       int      `json:"end_hour"`
	SoundKeywords []string `json:"sound_keywords"`
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
			TargetTV:      "192.168.1.8",
			WindowSeconds: 60,
		},
	}
}

func (s *ConfigStore) load() {
	data, err := os.ReadFile(s.path)
	if err != nil {
		s.save()
		return
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err == nil {
		s.config = cfg
	}
}

func (s *ConfigStore) save() {
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
	s.config = cfg
	s.save()
}
