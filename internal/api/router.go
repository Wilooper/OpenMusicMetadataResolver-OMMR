package api

import (
	"log/slog"
	"net/http"

	"github.com/ommr/ommr/internal/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/time/rate"
)

// NewRouter constructs the HTTP handler with middleware pipeline attached.
func NewRouter(h *Handler, logger *slog.Logger, rps float64) http.Handler {
	mux := http.NewServeMux()

	// REST API Endpoints
	mux.HandleFunc("/v1/resolve", h.HandleResolve)
	mux.HandleFunc("/v1/links", h.HandleLinks)
	mux.HandleFunc("/v1/extract", h.HandleExtract)
	mux.HandleFunc("/v1/bulk", h.HandleBulk)
	mux.HandleFunc("/v1/search", h.HandleSearch)
	mux.HandleFunc("/v1/health", h.HandleHealth)

	// Metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())

	// Attach middleware pipeline
	apiLimiter := rate.NewLimiter(rate.Limit(rps), int(rps*2))
	var handler http.Handler = mux
	handler = middleware.RateLimiter(apiLimiter)(handler)
	handler = middleware.Logger(logger)(handler)
	handler = middleware.Recovery(logger)(handler)
	handler = middleware.RequestID(handler)

	return handler
}
