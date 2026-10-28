package tvmodes

import "strings"

const (
	NormalID      = "normal"
	ServiceOn     = "turn_on"
	ServiceOff    = "turn_off"
	restoreScene  = "sentinel_tv_mode_restore"
	maxModes      = 12
	maxActions    = 32
	maxNameLength = 40
)

type Action struct {
	Entity  string         `json:"entity"`
	Service string         `json:"service"`
	Data    map[string]any `json:"data,omitempty"`
}

type Mode struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Icon        string   `json:"icon"`
	Description string   `json:"description,omitempty"`
	Actions     []Action `json:"actions"`
}

type Config struct {
	Modes          []Mode `json:"modes"`
	Active         string `json:"active,omitempty"`
	Snapshotted    bool   `json:"snapshotted,omitempty"`
	AutoOnPlayback bool   `json:"auto_on_playback"`
}

type Summary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Icon        string `json:"icon"`
	Description string `json:"description,omitempty"`
}

type State struct {
	Active string    `json:"active"`
	Modes  []Summary `json:"modes"`
}

func (a Action) domain() string {
	domain, _, found := strings.Cut(a.Entity, ".")
	if !found {
		return ""
	}
	return domain
}

func (a Action) valid() bool {
	if a.domain() == "" {
		return false
	}
	return a.Service == ServiceOn || a.Service == ServiceOff
}

func (m Mode) summary() Summary {
	return Summary{ID: m.ID, Name: m.Name, Icon: m.Icon, Description: m.Description}
}

func DefaultConfig() Config {
	return Config{Modes: []Mode{
		{ID: "movie", Name: "Xem phim", Icon: "🎬", Description: "Tắt đèn, bật quạt"},
		{ID: "dinner", Name: "Ăn cơm", Icon: "🍚", Description: "Giữ đèn, bật quạt"},
	}}
}
