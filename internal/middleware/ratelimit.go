package middleware

import (
	"net/http"

	"golang.org/x/time/rate"
)

// RateLimiter returns a token bucket rate-limiting middleware for HTTP endpoints.
func RateLimiter(limiter *rate.Limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !limiter.Allow() {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":"Rate limit exceeded. Try again later."}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
