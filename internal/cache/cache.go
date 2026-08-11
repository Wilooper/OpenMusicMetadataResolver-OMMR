package cache

import (
	"context"
	"time"
)

// Cache specifies the contract for caching raw responses, normalized candidates, and resolved tracks.
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, val []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Close() error
}
