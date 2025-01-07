package logging

import (
	"net/http"
)

// ResponseWriter wraps http.ResponseWriter to capture the status code
type ResponseWriter struct {
	http.ResponseWriter
	status int
}

// NewResponseWriter creates a new ResponseWriter
func NewResponseWriter(w http.ResponseWriter) *ResponseWriter {
	return &ResponseWriter{
		ResponseWriter: w,
		status:         http.StatusOK, // Default to 200 OK
	}
}

// WriteHeader captures the status code and passes it to the underlying ResponseWriter
func (w *ResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

// Status returns the captured status code
func (w *ResponseWriter) Status() int {
	return w.status
}
