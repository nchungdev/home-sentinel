package cache

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMemoryBoundsAndValueIsolation(t *testing.T) {
	store := NewMemory(1)
	input := []byte("first")
	_ = store.Set(context.Background(), "a", input, time.Minute)
	input[0] = 'X'
	value, err := store.Get(context.Background(), "a")
	if err != nil || string(value) != "first" {
		t.Fatalf("caller mutated cached value: %s, %v", value, err)
	}
	value[0] = 'X'
	value, _ = store.Get(context.Background(), "a")
	if string(value) != "first" {
		t.Fatal("read result aliases cache storage")
	}
	_ = store.Set(context.Background(), "b", []byte("second"), time.Minute)
	if _, err := store.Get(context.Background(), "a"); !errors.Is(err, ErrMiss) {
		t.Fatal("cache exceeded entry capacity")
	}
}

func TestExpiredMemoryEntryIsNotServed(t *testing.T) {
	store := NewMemory(10)
	store.entries["expired"] = entry{[]byte("stale"), time.Now().Add(-time.Minute)}
	if _, err := store.Get(context.Background(), "expired"); !errors.Is(err, ErrMiss) {
		t.Fatal("stale data served as fresh")
	}
}

func TestCacheKeysKeepPartBoundariesAndScope(t *testing.T) {
	if Key("a\x00b", "c") == Key("a", "b\x00c") || Key("user1", "q") == Key("user2", "q") {
		t.Fatal("cache keys collide across query/scope boundaries")
	}
}

func TestUnavailableRedisRespectsContext(t *testing.T) {
	t.Setenv("REDIS_URL", "redis://127.0.0.1:1/0")
	store, err := FromEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	defer store.(*Redis).Close()
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if _, err := store.Get(ctx, "test"); err == nil {
		t.Fatal("unavailable Redis returned successful read")
	}
}
