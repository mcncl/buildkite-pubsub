package logging

import (
	"net/http"
	"time"

	"github.com/mcncl/buildkite-pubsub/internal/logging"
	"github.com/mcncl/buildkite-pubsub/internal/middleware/request"
)

// WithStructuredLogging adds structured logging to the request/response cycle
// using the new structured logging package from internal/logging
func WithStructuredLogging(logger logging.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Create a response writer that captures status code and size
			lrw := logging.NewLogResponseWriter(w)

			// Get request ID from context or header
			requestID := r.Header.Get(request.RequestIDHeader)
			if requestID == "" {
				if id := r.Context().Value(request.RequestIDKey); id != nil {
					requestID = id.(string)
				} else {
					requestID = "unknown"
				}
			}

			// Create a logger with request details
			reqLogger := logger.
				WithField("method", r.Method).
				WithField("path", r.URL.Path).
				WithField("remote_addr", r.RemoteAddr).
				WithField("request_id", requestID)

			// Add user agent if available
			if userAgent := r.Header.Get("User-Agent"); userAgent != "" {
				reqLogger = reqLogger.WithField("user_agent", userAgent)
			}

			// Add content type if available
			if contentType := r.Header.Get("Content-Type"); contentType != "" {
				reqLogger = reqLogger.WithField("content_type", contentType)
			}

			// Log the request start
			reqLogger.Info("Request started")

			// Add logger to context and process the request
			ctx := logging.WithLogger(r.Context(), reqLogger)
			next.ServeHTTP(lrw, r.WithContext(ctx))

			// Calculate duration
			duration := time.Since(start)

			// Log the response with timing and status information
			reqLogger.WithField("status", lrw.StatusCode()).
				WithField("duration_ms", duration.Milliseconds()).
				WithField("size", lrw.Size()).
				Info("Request completed")
		})
	}
}
