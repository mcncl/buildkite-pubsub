package middleware

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// Request ID middleware
func WithRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}
		ctx := context.WithValue(r.Context(), "requestID", requestID)
		w.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Request timeout middleware
func WithRequestTimeout(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Logging middleware
func WithLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a custom response writer to capture status code
		rw := &ResponseWriter{w, http.StatusOK}

		next.ServeHTTP(rw, r)

		// Log request details
		log.Printf("RequestID: %v Method: %v Path: %v Status: %v Duration: %v",
			r.Context().Value("requestID"),
			r.Method,
			r.URL.Path,
			rw.status,
			time.Since(start),
		)
	})
}

// Custom response writer to capture status code
type ResponseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *ResponseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}
