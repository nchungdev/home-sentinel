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
	haClient   *haclient.Client
	getCamera  func() CameraConfig
	broadcast  func(string)
	tvHost     string
	inactivity time.Duration

	lastActivityTime time.Time
	lastDoorTrigger  time.Time
	activityMutex    sync.Mutex
	stopChan         chan struct{}
}

func NewManager(
	client *haclient.Client,
	getCamera func() CameraConfig,
	broadcast func(string),
	tvHost string,
	inactivity time.Duration,
) *Manager {
	if inactivity <= 0 {
		inactivity = 15 * time.Minute
	}
	m := &Manager{
		haClient:         client,
		getCamera:        getCamera,
		broadcast:        broadcast,
		tvHost:           tvHost,
		inactivity:       inactivity,
		lastActivityTime: time.Now(),
		stopChan:         make(chan struct{}),
	}
	go m.runInactivityLoop()
	return m
}

func (m *Manager) Stop() {
	close(m.stopChan)
}

func (m *Manager) RecordActivity(reason string) {
	m.activityMutex.Lock()
	m.lastActivityTime = time.Now()
	m.activityMutex.Unlock()
}
