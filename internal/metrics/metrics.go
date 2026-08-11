package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ommr_http_requests_total",
			Help: "Total number of HTTP requests processed by OMMR.",
		},
		[]string{"endpoint", "status"},
	)

	HTTPRequestDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ommr_http_request_duration_seconds",
			Help:    "Histogram of HTTP request durations in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"endpoint"},
	)

	CacheHitsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ommr_cache_hits_total",
			Help: "Total number of cache hit lookups.",
		},
		[]string{"cache_type"},
	)

	CacheMissesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ommr_cache_misses_total",
			Help: "Total number of cache miss lookups.",
		},
		[]string{"cache_type"},
	)

	ProviderLatencySeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ommr_provider_latency_seconds",
			Help:    "Latency of provider adapter HTTP requests in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"provider"},
	)

	ProviderErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ommr_provider_errors_total",
			Help: "Total number of provider adapter errors.",
		},
		[]string{"provider"},
	)
)
