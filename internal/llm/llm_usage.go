package llm

import (
	"sync"
	"time"
)

type Usage struct {
	EngineID string    `json:"engine_id"`
	At       time.Time `json:"at"`
}

type usageTracker struct {
	mutex sync.Mutex
	last  Usage
}

func (t *usageTracker) record(engineID string) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.last = Usage{EngineID: engineID, At: time.Now()}
}

func (t *usageTracker) snapshot() (Usage, bool) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	return t.last, t.last.EngineID != ""
}

func (m *Manager) LastUsage() (Usage, bool) {
	return m.usage.snapshot()
}
