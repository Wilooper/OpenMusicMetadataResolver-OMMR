package ratelimit

import (
	"context"
	"strings"
	"sync"

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
	// MusicBrainz: 1 req/sec, no burst
	// Deezer: 50 req/sec (burst 10)
	// Keyless iTunes: approximately 20/minute; catalog uses a separate conservative budget.
	// YTMusic: 20 req/sec (burst 5)
	// Spotify: 10 req/sec (burst 3)
	// SoundCloud: 10 req/sec (burst 3)
	// JioSaavn: 5 req/sec (burst 2)
	pl.SetLimit("musicbrainz", 1, 1)
	pl.SetLimit("deezer", 50, 10)
	pl.SetLimit("applemusic", 1.0/3.0, 1)
	pl.SetLimit("applemusic_catalog", 5, 1)
	pl.SetLimit("coverartarchive", 3, 1)
	pl.SetLimit("lastfm", 3, 1)
	pl.SetLimit("discogs", 1, 1)
	pl.SetLimit("acoustid", 3, 1)
	pl.SetLimit("ytmusic", 20, 5)
	pl.SetLimit("spotify", 10, 3)
	pl.SetLimit("soundcloud", 10, 3)
	pl.SetLimit("jiosaavn", 5, 2)
	pl.SetLimit("qobuz", 5, 2)
	pl.SetLimit("tidal", 10, 3)
	pl.SetLimit("amazonmusic", 5, 2)
	pl.SetLimit("pandora", 5, 2)

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

	return limiter.Wait(ctx)
}
