package cache

import (
	"context"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

type Redis struct {
	client *redis.Client
}

func FromEnvironment() (Store, error) {
	address := os.Getenv("REDIS_URL")
	if address == "" {
		return NewMemory(128), nil
	}
	options, err := redis.ParseURL(address)
	if err != nil {
		return NewMemory(128), err
	}
	options.DialTimeout = 100 * time.Millisecond
	options.ReadTimeout = 100 * time.Millisecond
	options.WriteTimeout = 100 * time.Millisecond
	options.PoolTimeout = 100 * time.Millisecond
	options.MaxRetries = -1
	options.ContextTimeoutEnabled = true
	options.PoolSize = 4

	return &Redis{client: redis.NewClient(options)}, nil
}

func (r *Redis) Get(ctx context.Context, key string) ([]byte, error) {
	value, err := r.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, ErrMiss
	}

	return value, err
}

func (r *Redis) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return r.client.Set(ctx, key, value, ttl).Err()
}

func (r *Redis) Close() error { return r.client.Close() }
