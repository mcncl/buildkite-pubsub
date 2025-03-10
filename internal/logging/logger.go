package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/mcncl/buildkite-pubsub/internal/middleware/request"
)

// Level represents a logging level
type Level int

const (
	// LevelDebug enables all logs
	LevelDebug Level = iota
	// LevelInfo enables info and above logs
	LevelInfo
	// LevelWarn enables warn and above logs
	LevelWarn
	// LevelError enables only error logs
	LevelError
)

// String returns a string representation of the log level
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "debug"
	case LevelInfo:
		return "info"
	case LevelWarn:
		return "warn"
	case LevelError:
		return "error"
	default:
		return "unknown"
	}
}

// Format represents a logging format
type Format int

const (
	// FormatJSON outputs logs in JSON format
	FormatJSON Format = iota
	// FormatText outputs logs in key=value format
	FormatText
	// FormatDevelopment outputs logs in a human-friendly format with colors
	FormatDevelopment
)

// LoggerContextKey is the context key for the logger
type LoggerContextKey struct{}

// defaultLogger is used when a logger can't be found in context
var (
	defaultLogger Logger
	defaultOnce   sync.Once
)

// initDefaultLogger initializes the default logger
func initDefaultLogger() {
	defaultLogger = NewLogger(Config{
		Output:   os.Stderr,
		Level:    LevelInfo,
		Format:   FormatJSON,
		AppName:  "buildkite-webhook",
		Hostname: getHostname(),
	})
}

// getHostname returns the hostname or "unknown" if it can't be determined
func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}

// Config holds configuration for a logger
type Config struct {
	// Output writer for logs (defaults to os.Stderr)
	Output io.Writer
	// Log level (defaults to LevelInfo)
	Level Level
	// Output format (defaults to FormatJSON)
	Format Format
	// Application name to include in logs
	AppName string
	// Hostname to include in logs
	Hostname string
}

// Logger interface defines structured logging methods
type Logger interface {
	// Debug logs a debug message
	Debug(msg string)
	// Info logs an info message
	Info(msg string)
	// Warn logs a warning message
	Warn(msg string)
	// Error logs an error message
	Error(msg string)

	// WithField adds a field to the logger
	WithField(key string, value interface{}) Logger
	// WithError adds an error to the logger
	WithError(err error) Logger
	// WithContext adds context fields to the logger
	WithContext(ctx context.Context) Logger
}

// stdLogger is the standard implementation of Logger
type stdLogger struct {
	config Config
	fields map[string]interface{}
	mu     sync.Mutex
}

// NewLogger creates a new structured logger
func NewLogger(config Config) Logger {
	// Set defaults
	if config.Output == nil {
		config.Output = os.Stderr
	}

	return &stdLogger{
		config: config,
		fields: make(map[string]interface{}),
	}
}

// clone creates a copy of the logger with copied fields
func (l *stdLogger) clone() *stdLogger {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Create new fields map
	fields := make(map[string]interface{}, len(l.fields))
	for k, v := range l.fields {
		fields[k] = v
	}

	// Return a new logger with the copied fields
	return &stdLogger{
		config: l.config,
		fields: fields,
	}
}

// WithField adds a field to the logger
func (l *stdLogger) WithField(key string, value interface{}) Logger {
	logger := l.clone()
	logger.fields[key] = value
	return logger
}

// WithError adds an error to the logger
func (l *stdLogger) WithError(err error) Logger {
	if err == nil {
		return l
	}

	logger := l.clone()
	// Create a structured error object
	errObj := map[string]interface{}{
		"message": err.Error(),
	}

	// If the error has a stack trace, add it
	if stackErr, ok := err.(interface{ Stack() string }); ok {
		errObj["stack"] = stackErr.Stack()
	}

	logger.fields["error"] = errObj
	return logger
}

// WithContext adds context fields to the logger
func (l *stdLogger) WithContext(ctx context.Context) Logger {
	if ctx == nil {
		return l
	}

	logger := l.clone()

	// Add request ID if present
	if reqID, ok := ctx.Value(request.RequestIDKey).(string); ok {
		logger.fields["request_id"] = reqID
	}

	return logger
}

// Debug logs a debug message
func (l *stdLogger) Debug(msg string) {
	if l.config.Level > LevelDebug {
		return
	}
	l.log(LevelDebug, msg)
}

// Info logs an info message
func (l *stdLogger) Info(msg string) {
	if l.config.Level > LevelInfo {
		return
	}
	l.log(LevelInfo, msg)
}

// Warn logs a warning message
func (l *stdLogger) Warn(msg string) {
	if l.config.Level > LevelWarn {
		return
	}
	l.log(LevelWarn, msg)
}

// Error logs an error message
func (l *stdLogger) Error(msg string) {
	if l.config.Level > LevelError {
		return
	}
	l.log(LevelError, msg)
}

// log handles the actual logging
func (l *stdLogger) log(level Level, msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Create entry with standard fields
	entry := map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"level":     level.String(),
		"message":   msg,
		"app":       l.config.AppName,
		"hostname":  l.config.Hostname,
	}

	// Add custom fields
	for k, v := range l.fields {
		entry[k] = v
	}

	// Format according to config
	var output []byte
	var err error

	switch l.config.Format {
	case FormatJSON:
		output, err = json.Marshal(entry)
		if err == nil {
			output = append(output, '\n')
		}
	case FormatText:
		output = []byte(formatAsText(entry))
	case FormatDevelopment:
		output = []byte(formatForDevelopment(level, entry))
	}

	if err != nil {
		// Fallback if marshaling fails
		fmt.Fprintf(l.config.Output, "ERROR MARSHALING LOG: %v\n", err)
		return
	}

	l.config.Output.Write(output)
}

// formatAsText formats a log entry as key=value pairs
func formatAsText(entry map[string]interface{}) string {
	// Start with timestamp and level which we want at the beginning
	parts := []string{
		fmt.Sprintf("time=%s", entry["timestamp"]),
		fmt.Sprintf("level=%s", entry["level"]),
		fmt.Sprintf("msg=%q", entry["message"]),
	}
	delete(entry, "timestamp")
	delete(entry, "level")
	delete(entry, "message")

	// Add remaining fields
	for k, v := range entry {
		// Handle different value types
		var value string
		switch v := v.(type) {
		case string:
			value = fmt.Sprintf("%q", v)
		case error:
			value = fmt.Sprintf("%q", v.Error())
		case map[string]interface{}:
			// Simplify nested structures
			jsonBytes, err := json.Marshal(v)
			if err != nil {
				value = "\"{}\""
			} else {
				value = fmt.Sprintf("%q", string(jsonBytes))
			}
		default:
			value = fmt.Sprintf("%v", v)
		}
		parts = append(parts, fmt.Sprintf("%s=%s", k, value))
	}

	return strings.Join(parts, " ") + "\n"
}

// formatForDevelopment formats a log entry in a human-friendly way
func formatForDevelopment(level Level, entry map[string]interface{}) string {
	// Color code by level
	var levelColor, levelName string
	switch level {
	case LevelDebug:
		levelColor = "\033[36m" // Cyan
		levelName = "DEBUG"
	case LevelInfo:
		levelColor = "\033[32m" // Green
		levelName = "INFO"
	case LevelWarn:
		levelColor = "\033[33m" // Yellow
		levelName = "WARN"
	case LevelError:
		levelColor = "\033[31m" // Red
		levelName = "ERROR"
	}
	resetColor := "\033[0m"

	// Format time
	timestamp := entry["timestamp"].(string)
	timeStr := timestamp[11:19] // Get just the time part (HH:MM:SS)

	// Format message and fields
	msg := entry["message"].(string)
	delete(entry, "timestamp")
	delete(entry, "level")
	delete(entry, "message")

	// Build extra fields string
	var fields string
	if len(entry) > 0 {
		fieldParts := make([]string, 0, len(entry))
		for k, v := range entry {
			if k == "app" || k == "hostname" {
				continue // Skip these common fields for cleaner output
			}

			// Special handling for error
			if k == "error" {
				if errMap, ok := v.(map[string]interface{}); ok {
					if errMsg, ok := errMap["message"].(string); ok {
						fieldParts = append(fieldParts, fmt.Sprintf("%s=%q", k, errMsg))
						continue
					}
				}
			}

			fieldParts = append(fieldParts, fmt.Sprintf("%s=%v", k, v))
		}
		if len(fieldParts) > 0 {
			fields = " " + strings.Join(fieldParts, " ")
		}
	}

	// Format all parts together
	return fmt.Sprintf("%s %s%s%s: %s%s\n", 
		timeStr, 
		levelColor, 
		levelName, 
		resetColor,
		msg,
		fields,
	)
}

// WithLogger returns a context with the logger attached
func WithLogger(ctx context.Context, logger Logger) context.Context {
	return context.WithValue(ctx, LoggerContextKey{}, logger)
}

// FromContext retrieves a logger from the context
func FromContext(ctx context.Context) Logger {
	if ctx == nil {
		return getDefaultLogger()
	}
	if logger, ok := ctx.Value(LoggerContextKey{}).(Logger); ok {
		return logger
	}
	return getDefaultLogger()
}

// getDefaultLogger returns the default logger, initializing it if necessary
func getDefaultLogger() Logger {
	defaultOnce.Do(initDefaultLogger)
	return defaultLogger
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
		statusCode:     http.StatusOK, // Default to 200 OK
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

// NewLoggerMiddleware creates middleware that adds a logger to the request context
func NewLoggerMiddleware(logger Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Create a response writer that captures status code and size
			lrw := NewLogResponseWriter(w)

			// Get or create request ID
			requestID := r.Header.Get(request.RequestIDHeader)
			if requestID == "" {
				requestID = "unknown"
			}

			// Create a logger with request details
			reqLogger := logger.
				WithField("method", r.Method).
				WithField("path", r.URL.Path).
				WithField("request_id", requestID).
				WithField("remote_addr", r.RemoteAddr)

			// Add custom request headers if needed (be careful with sensitive data)
			if userAgent := r.Header.Get("User-Agent"); userAgent != "" {
				reqLogger = reqLogger.WithField("user_agent", userAgent)
			}

			// Log the request
			reqLogger.Info("Request started")

			// Add logger to context and process the request
			ctx := WithLogger(r.Context(), reqLogger)
			next.ServeHTTP(lrw, r.WithContext(ctx))

			// Calculate duration
			duration := time.Since(start)

			// Log the response
			reqLogger.WithField("status", lrw.StatusCode()).
				WithField("duration", duration.Milliseconds()).
				WithField("size", lrw.Size()).
				Info("Request completed")
		})
	}
}
