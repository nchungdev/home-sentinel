package automation

import (
	"sync"
	"time"

	"github.com/nchungdev/home-sentinel/agent/pkg/haclient"
)

type CameraConfig struct {
	IP            string `json:"ip"`
	Username      string `json:"username"`
	Password      string `json:"password"`
	CloudPassword string `json:"cloud_password"`
}

type Manager struct {
	haClient  *haclient.Client
	getCamera func() CameraConfig
	broadcast func(string)
	tvHost    string

	store *ConfigStore

	lastActivityTime time.Time
	lastDoorTrigger  time.Time
	lastTVPromptTime time.Time
	lastFanTrigger   time.Time
	activityMutex    sync.Mutex
	stopChan         chan struct{}
}

func NewManager(
	client *haclient.Client,
	getCamera func() CameraConfig,
	broadcast func(string),
	tvHost string,
	configPath string,
) *Manager {
	if configPath == "" {
		configPath = "sentinel_automation_config.json"
	}
	m := &Manager{
		haClient:         client,
		getCamera:        getCamera,
		broadcast:        broadcast,
		tvHost:           tvHost,
		store:            NewConfigStore(configPath),
		lastActivityTime: time.Now(),
		stopChan:         make(chan struct{}),
	}
	go m.runInactivityLoop()
	return m
}

func (m *Manager) Stop() {
	close(m.stopChan)
}

func (m *Manager) GetConfig() Config {
	return m.store.Get()
}

func (m *Manager) UpdateConfig(cfg Config) {
	m.store.Update(cfg)
}

func (m *Manager) RecordActivity(reason string) {
	m.activityMutex.Lock()
	m.lastActivityTime = time.Now()
	m.activityMutex.Unlock()
}
