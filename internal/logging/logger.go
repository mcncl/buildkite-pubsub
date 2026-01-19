package logging

import (
	"log/slog"
	"net/http"
	"os"
)

// NewLogger creates a new slog.Logger with the specified level and format.
func NewLogger(level, format string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: lvl}

	var handler slog.Handler
	if format == "text" || format == "dev" {
		handler = slog.NewTextHandler(os.Stderr, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	}

	return slog.New(handler)
}

// LogResponseWriter wraps http.ResponseWriter to capture status code and response size
type LogResponseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int
}

// NewLogResponseWriter creates a new LogResponseWriter
func NewLogResponseWriter(w http.ResponseWriter) *LogResponseWriter {
	return &LogResponseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
}

// WriteHeader captures the status code
func (w *LogResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

// Write captures the response size
func (w *LogResponseWriter) Write(b []byte) (int, error) {
	size, err := w.ResponseWriter.Write(b)
	w.size += size
	return size, err
}

// StatusCode returns the captured status code
func (w *LogResponseWriter) StatusCode() int {
	return w.statusCode
}

// Size returns the response size
func (w *LogResponseWriter) Size() int {
	return w.size
}
