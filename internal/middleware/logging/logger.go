package logging

import (
	"log"
	"net/http"
	"time"
)

type contextKey string

// LogEntry represents a log entry for a request
type LogEntry struct {
	RequestID  string
	Method     string
	Path       string
	Status     int
	Duration   time.Duration
	RemoteAddr string
	UserAgent  string
	Error      error
}

// WithLogging adds request logging to the handler
func WithLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a custom response writer to capture status code
		rw := NewResponseWriter(w)

		// Process request
		next.ServeHTTP(rw, r)

		// Create log entry
		entry := LogEntry{
			RequestID:  getRequestID(r),
			Method:     r.Method,
			Path:       r.URL.Path,
			Status:     rw.Status(),
			Duration:   time.Since(start),
			RemoteAddr: r.RemoteAddr,
			UserAgent:  r.UserAgent(),
		}

		// Log the entry
		logRequest(entry)
	})
}

// getRequestID retrieves request ID from context
func getRequestID(r *http.Request) string {
	if id := r.Context().Value("requestID"); id != nil {
		return id.(string)
	}
	return "unknown"
}

// logRequest logs the request details
func logRequest(entry LogEntry) {
	log.Printf("RequestID: %s Method: %s Path: %s Status: %d Duration: %v",
		entry.RequestID,
		entry.Method,
		entry.Path,
		entry.Status,
		entry.Duration,
	)
}
