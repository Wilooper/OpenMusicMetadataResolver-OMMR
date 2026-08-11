package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/ommr/ommr/internal/cache"
	redis "github.com/redis/go-redis/v9"
)

type RedisCache struct {
	client *redis.Client
	prefix string
}

var _ cache.Cache = (*RedisCache)(nil)

// NewRedisCache creates a RedisCache connected to redisURL (e.g., redis://localhost:6379/0 or Upstash Redis URL).
func NewRedisCache(redisURL string, keyPrefix string) (*RedisCache, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid redis URL: %w", err)
	}

	client := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to ping redis: %w", err)
	}

	if keyPrefix == "" {
		keyPrefix = "ommr:"
	}

	return &RedisCache{
		client: client,
		prefix: keyPrefix,
	}, nil
}

func (r *RedisCache) formatKey(key string) string {
	return r.prefix + key
}

func (r *RedisCache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	val, err := r.client.Get(ctx, r.formatKey(key)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, false, nil
		}
		return nil, false, err
	}
	return val, true, nil
}

func (r *RedisCache) Set(ctx context.Context, key string, val []byte, ttl time.Duration) error {
	return r.client.Set(ctx, r.formatKey(key), val, ttl).Err()
}

func (r *RedisCache) Delete(ctx context.Context, key string) error {
	return r.client.Del(ctx, r.formatKey(key)).Err()
}

func (r *RedisCache) Close() error {
	return r.client.Close()
}
