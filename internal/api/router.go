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
	mux.HandleFunc("/v1/capabilities", h.withDefaults("capabilities", h.HandleCapabilities))
	mux.HandleFunc("/v1/albums", h.withDefaults("albums", h.HandleAlbums))
	mux.HandleFunc("/v1/artists", h.withDefaults("artists", h.HandleArtists))
	mux.HandleFunc("/v1/catalog", h.withDefaults("catalog", h.HandleCatalog))
	mux.HandleFunc("/v1/resolve", h.withDefaults("resolve", h.HandleResolve))
	mux.HandleFunc("/v1/links", h.withDefaults("links", h.HandleLinks))
	mux.HandleFunc("/v1/extract", h.withDefaults("extract", h.HandleExtract))
	mux.HandleFunc("/v1/bulk", h.withDefaults("bulk", h.HandleBulk))
	mux.HandleFunc("/v1/search", h.withDefaults("search", h.HandleSearch))
	mux.HandleFunc("/v1/fingerprint", h.withDefaults("fingerprint", h.HandleFingerprint))
	mux.HandleFunc("/v1/health", h.withDefaults("health", h.HandleHealth))

	mux.HandleFunc("/v1/query", h.withDefaults("query", h.HandleQuery))
	mux.HandleFunc("/v1/providers", h.withDefaults("providers", h.HandleProviders))

	// Metrics endpoint
	if h.cfg.API.Enabled("metrics") {
		mux.Handle("/metrics", promhttp.Handler())
	}

	// Attach middleware pipeline
	apiLimiter := rate.NewLimiter(rate.Limit(rps), int(rps*2))
	var handler http.Handler = mux
	handler = middleware.RateLimiter(apiLimiter)(handler)
	handler = middleware.Logger(logger)(handler)
	handler = middleware.Recovery(logger)(handler)
	handler = middleware.Compression(handler)
	handler = middleware.RequestID(handler)

	return handler
}
