package disk

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestDiskCache(t *testing.T) {
	dir, err := os.MkdirTemp("", "ommr-diskcache-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	cache, err := NewDiskCache(dir)
	if err != nil {
		t.Fatalf("failed to create DiskCache: %v", err)
	}

	ctx := context.Background()
	key := "raw:deezer:12345"
	val := []byte("hello deezer raw data")

	// Get non-existent
	_, found, err := cache.Get(ctx, key)
	if err != nil || found {
		t.Errorf("expected not found, got found=%v, err=%v", found, err)
	}

	// Set value
	err = cache.Set(ctx, key, val, 1*time.Hour)
	if err != nil {
		t.Fatalf("failed to set cache: %v", err)
	}

	// Get value
	got, found, err := cache.Get(ctx, key)
	if err != nil || !found {
		t.Fatalf("expected found, got found=%v, err=%v", found, err)
	}
	if string(got) != string(val) {
		t.Errorf("got %q, want %q", string(got), string(val))
	}

	// Test expiration
	expKey := "normalized:expired"
	err = cache.Set(ctx, expKey, val, 1*time.Millisecond)
	if err != nil {
		t.Fatalf("failed to set expKey: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	_, found, err = cache.Get(ctx, expKey)
	if found {
		t.Errorf("expected expired item to be not found")
	}
}
