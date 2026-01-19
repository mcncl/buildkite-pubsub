package logging

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/mcncl/buildkite-pubsub/internal/logging"
	"github.com/mcncl/buildkite-pubsub/internal/middleware/request"
)

// WithStructuredLogging adds structured logging to the request/response cycle
func WithStructuredLogging(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			lrw := logging.NewLogResponseWriter(w)

			requestID := r.Header.Get(request.RequestIDHeader)
			if requestID == "" {
				if id := r.Context().Value(request.RequestIDKey); id != nil {
					requestID = id.(string)
				} else {
					requestID = "unknown"
				}
			}

			logger.Info("Request started",
				"method", r.Method,
				"path", r.URL.Path,
				"remote_addr", r.RemoteAddr,
				"request_id", requestID,
			)

			next.ServeHTTP(lrw, r.WithContext(r.Context()))

			logger.Info("Request completed",
				"method", r.Method,
				"path", r.URL.Path,
				"request_id", requestID,
				"status", lrw.StatusCode(),
				"duration_ms", time.Since(start).Milliseconds(),
				"size", lrw.Size(),
			)
		})
	}
}
