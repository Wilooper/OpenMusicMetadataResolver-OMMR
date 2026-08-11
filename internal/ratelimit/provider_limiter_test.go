package ratelimit

import (
	"context"
	"testing"
	"time"
)

func TestProviderLimiter(t *testing.T) {
	pl := NewProviderLimiter()
	ctx := context.Background()

	err := pl.Wait(ctx, "deezer")
	if err != nil {
		t.Errorf("expected no rate limit wait error for deezer, got %v", err)
	}

	// Unregistered provider should pass without error
	err = pl.Wait(ctx, "unknown")
	if err != nil {
		t.Errorf("expected no error for unknown provider, got %v", err)
	}

	// Custom limit
	pl.SetLimit("test_prov", 100, 10)
	err = pl.Wait(ctx, "test_prov")
	if err != nil {
		t.Errorf("expected no error for test_prov, got %v", err)
	}
	_ = time.Second
}
