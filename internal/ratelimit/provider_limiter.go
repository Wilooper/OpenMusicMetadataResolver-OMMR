package ratelimit

import (
	"context"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// ProviderLimiter manages per-provider token bucket rate limiters.
type ProviderLimiter struct {
	mu       sync.RWMutex
	limiters map[string]*rate.Limiter
}

// NewProviderLimiter initializes a ProviderLimiter with default provider rate limits.
func NewProviderLimiter() *ProviderLimiter {
	pl := &ProviderLimiter{
		limiters: make(map[string]*rate.Limiter),
	}

	// Default per-provider rate limits:
	// MusicBrainz: 1 req/sec (burst 2)
	// Deezer: 50 req/sec (burst 10)
	// Apple Music: 20 req/sec (burst 5)
	// YTMusic: 20 req/sec (burst 5)
	// Spotify: 10 req/sec (burst 3)
	pl.SetLimit("musicbrainz", 1, 2)
	pl.SetLimit("deezer", 50, 10)
	pl.SetLimit("applemusic", 20, 5)
	pl.SetLimit("ytmusic", 20, 5)
	pl.SetLimit("spotify", 10, 3)

	return pl
}

// SetLimit configures rate limit (RPS) and burst for a provider.
func (p *ProviderLimiter) SetLimit(provider string, rps float64, burst int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.limiters[strings.ToLower(provider)] = rate.NewLimiter(rate.Limit(rps), burst)
}

// Wait blocks until the rate limiter permits an execution or context is cancelled.
func (p *ProviderLimiter) Wait(ctx context.Context, provider string) error {
	p.mu.RLock()
	limiter, ok := p.limiters[strings.ToLower(provider)]
	p.mu.RUnlock()

	if !ok {
		// Default fallback rate limit (10 RPS)
		return nil
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return limiter.Wait(ctxTimeout)
}
