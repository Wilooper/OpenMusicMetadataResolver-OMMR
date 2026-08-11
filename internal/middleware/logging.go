package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/ommr/ommr/internal/metrics"
)

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// Logger records structured logs with slog and updates Prometheus request metrics.
func Logger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(rw, r)

			duration := time.Since(start)
			reqID := GetRequestID(r.Context())

			logger.Info("http request",
				slog.String("request_id", reqID),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", rw.status),
				slog.Duration("duration", duration),
			)

			metrics.HTTPRequestsTotal.WithLabelValues(r.URL.Path, http.StatusText(rw.status)).Inc()
			metrics.HTTPRequestDurationSeconds.WithLabelValues(r.URL.Path).Observe(duration.Seconds())
		})
	}
}
