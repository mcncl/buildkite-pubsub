package request

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

const (
	// RequestIDHeader is the header used for request ID propagation
	RequestIDHeader = "X-Request-ID"
	// RequestIDKey is the context key for request ID
	RequestIDKey = "requestID"
)

// WithRequestID adds a request ID to the request context and response headers
func WithRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get(RequestIDHeader)
		if requestID == "" {
			requestID = uuid.New().String()
		}
		ctx := context.WithValue(r.Context(), RequestIDKey, requestID)
		w.Header().Set(RequestIDHeader, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
