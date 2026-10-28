package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
	"time"
)

var ErrMiss = errors.New("cache_miss")

type Store interface {
	Get(context.Context, string) ([]byte, error)
	Set(context.Context, string, []byte, time.Duration) error
}

type entry struct {
	value   []byte
	expires time.Time
}

type Memory struct {
	mutex   sync.Mutex
	entries map[string]entry
	limit   int
}

func NewMemory(limit int) *Memory {
	if limit < 1 {
		limit = 128
	}

	return &Memory{entries: map[string]entry{}, limit: limit}
}

func Key(parts ...string) string {
	serialized, _ := json.Marshal(parts)
	digest := sha256.Sum256(serialized)
	return "sentinel:search:v1:" + hex.EncodeToString(digest[:])
}

func (m *Memory) Get(ctx context.Context, key string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mutex.Lock()
	defer m.mutex.Unlock()
	cached, found := m.entries[key]
	if !found || !cached.expires.After(time.Now()) {
		delete(m.entries, key)
		return nil, ErrMiss
	}

	return append([]byte(nil), cached.value...), nil
}

func (m *Memory) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if ttl <= 0 || len(value) > 128<<10 {
		return nil
	}
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if len(m.entries) >= m.limit {
		for candidate := range m.entries {
			delete(m.entries, candidate)
			break
		}
	}
	m.entries[key] = entry{append([]byte(nil), value...), time.Now().Add(ttl)}

	return nil
}
