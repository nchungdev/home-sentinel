package camera

type RtspTestRequest struct {
	IP       string `json:"ip"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Path     string `json:"path"`
}

type SpeakRequest struct {
	IP            string  `json:"ip"`
	Port          int     `json:"port"`
	User          string  `json:"user"`
	Password      string  `json:"password"`
	CloudPassword string  `json:"cloudPassword"`
	Path          string  `json:"path"`
	Text          string  `json:"text"`
	Voice         string  `json:"voice"`
	Speed         float64 `json:"speed"`
}

type TtsPreviewRequest struct {
	Text  string  `json:"text"`
	Voice string  `json:"voice"`
	Speed float64 `json:"speed"`
}

type SecurityConfigRequest struct {
	AlarmSpeaker *bool `json:"alarmSpeaker"`
	AlarmTv      *bool `json:"alarmTv"`
}

type WhitelistIPRequest struct {
	IP string `json:"ip"`
}

type CameraPTZRequest struct {
	IP            string `json:"ip"`
	Direction     string `json:"direction"`
	Step          int    `json:"step"`
	X             int    `json:"x"`
	Y             int    `json:"y"`
	User          string `json:"user"`
	Password      string `json:"password"`
	CloudPassword string `json:"cloudPassword"`
}

type CameraPrivacyRequest struct {
	IP            string `json:"ip"`
	Enabled       *bool  `json:"enabled"`
	User          string `json:"user"`
	Password      string `json:"password"`
	CloudPassword string `json:"cloudPassword"`
}

type CameraEventRequest struct {
	ZoneID       string   `json:"zone_id"`
	EventType    string   `json:"event_type"`
	Timestamp    int64    `json:"timestamp"`
	Headcount    int      `json:"headcount,omitempty"`
	Postures     []string `json:"postures,omitempty"`
	SnapshotPath string   `json:"snapshot_path,omitempty"`
}
