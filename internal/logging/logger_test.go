package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mcncl/buildkite-pubsub/internal/middleware/request"
)

func TestLogLevels(t *testing.T) {
	tests := []struct {
		name     string
		logLevel Level
		fn       func(Logger)
		contains []string
		excludes []string
	}{
		{
			name:     "debug logs show in debug level",
			logLevel: LevelDebug,
			fn: func(l Logger) {
				l.Debug("debug message")
			},
			contains: []string{"debug message", "level\":\"debug"},
		},
		{
			name:     "info logs show in debug level",
			logLevel: LevelDebug,
			fn: func(l Logger) {
				l.Info("info message")
			},
			contains: []string{"info message", "level\":\"info"},
		},
		{
			name:     "debug logs don't show in info level",
			logLevel: LevelInfo,
			fn: func(l Logger) {
				l.Debug("debug message")
			},
			excludes: []string{"debug message"},
		},
		{
			name:     "info logs show in info level",
			logLevel: LevelInfo,
			fn: func(l Logger) {
				l.Info("info message")
			},
			contains: []string{"info message"},
		},
		{
			name:     "warn logs show in info level",
			logLevel: LevelInfo,
			fn: func(l Logger) {
				l.Warn("warn message")
			},
			contains: []string{"warn message", "level\":\"warn"},
		},
		{
			name:     "error logs show in warn level",
			logLevel: LevelWarn,
			fn: func(l Logger) {
				l.Error("error message")
			},
			contains: []string{"error message", "level\":\"error"},
		},
		{
			name:     "warn logs don't show in error level",
			logLevel: LevelError,
			fn: func(l Logger) {
				l.Warn("warn message")
			},
			excludes: []string{"warn message"},
		},
		{
			name:     "error logs show in error level",
			logLevel: LevelError,
			fn: func(l Logger) {
				l.Error("error message")
			},
			contains: []string{"error message"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := NewLogger(Config{
				Output:   &buf,
				Level:    tt.logLevel,
				Format:   FormatJSON,
				AppName:  "test-app",
				Hostname: "test-host",
			})

			tt.fn(logger)

			logOutput := buf.String()
			for _, s := range tt.contains {
				if !strings.Contains(logOutput, s) {
					t.Errorf("expected log to contain %q, got %q", s, logOutput)
				}
			}
			for _, s := range tt.excludes {
				if strings.Contains(logOutput, s) {
					t.Errorf("expected log to NOT contain %q, got %q", s, logOutput)
				}
			}
		})
	}
}

func TestLogFields(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(Config{
		Output:   &buf,
		Level:    LevelDebug,
		Format:   FormatJSON,
		AppName:  "test-app",
		Hostname: "test-host",
	})

	logger.WithField("string_field", "value").
		WithField("int_field", 42).
		WithField("bool_field", true).
		WithField("float_field", 3.14).
		Info("test message with fields")

	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse JSON log entry: %v", err)
	}

	fieldTests := map[string]interface{}{
		"string_field": "value",
		"int_field":    float64(42), // JSON numbers are parsed as float64
		"bool_field":   true,
		"float_field":  3.14,
	}

	for field, expected := range fieldTests {
		if value, ok := logEntry[field]; !ok {
			t.Errorf("log entry missing field %q", field)
		} else if value != expected {
			t.Errorf("log field %q = %v, want %v", field, value, expected)
		}
	}

	// Check standard fields
	if msg, ok := logEntry["message"]; !ok || msg != "test message with fields" {
		t.Errorf("log message = %v, want %q", msg, "test message with fields")
	}

	if app, ok := logEntry["app"]; !ok || app != "test-app" {
		t.Errorf("app = %v, want %q", app, "test-app")
	}

	if host, ok := logEntry["hostname"]; !ok || host != "test-host" {
		t.Errorf("hostname = %v, want %q", host, "test-host")
	}

	if _, ok := logEntry["timestamp"]; !ok {
		t.Error("log entry missing timestamp")
	}
}

func TestLogWithError(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(Config{
		Output:   &buf,
		Level:    LevelDebug,
		Format:   FormatJSON,
		AppName:  "test-app",
		Hostname: "test-host",
	})

	testErr := &testError{message: "test error"}
	logger.WithError(testErr).Error("error occurred")

	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse JSON log entry: %v", err)
	}

	// Check error fields
	errorObj, ok := logEntry["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("log entry missing error object, got %T: %v", logEntry["error"], logEntry["error"])
	}

	if msg, ok := errorObj["message"]; !ok || msg != "test error" {
		t.Errorf("error message = %v, want %q", msg, "test error")
	}
}

func TestLogWithContext(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(Config{
		Output:   &buf,
		Level:    LevelDebug,
		Format:   FormatJSON,
		AppName:  "test-app",
		Hostname: "test-host",
	})

	// Create context with request ID
	ctx := context.WithValue(context.Background(), request.RequestIDKey, "test-request-id")
	logger.WithContext(ctx).Info("message with context")

	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse JSON log entry: %v", err)
	}

	// Check request ID field
	if reqID, ok := logEntry["request_id"]; !ok || reqID != "test-request-id" {
		t.Errorf("request_id = %v, want %q", reqID, "test-request-id")
	}
}

func TestMiddleware(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(Config{
		Output:   &buf,
		Level:    LevelDebug,
		Format:   FormatJSON,
		AppName:  "test-app",
		Hostname: "test-host",
	})

	// Create a test handler with the logger middleware
	handler := NewLoggerMiddleware(logger)(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			// Get logger from request context and log something
			FromContext(r.Context()).Info("handler log message")
			w.WriteHeader(http.StatusOK)
		}))

	// Create a test request with a request ID
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set(request.RequestIDHeader, "test-request-id")
	recorder := httptest.NewRecorder()

	// Process the request
	handler.ServeHTTP(recorder, req)

	// Parse the log entries (there should be at least 2 - request start and handler message)
	logOutput := buf.String()
	logLines := strings.Split(strings.TrimSpace(logOutput), "\n")

	if len(logLines) < 2 {
		t.Fatalf("Expected at least 2 log lines, got %d", len(logLines))
	}

	// Check the handler's log message
	var logEntry map[string]interface{}
	if err := json.Unmarshal([]byte(logLines[1]), &logEntry); err != nil {
		t.Fatalf("Failed to parse JSON log entry: %v", err)
	}

	if msg, ok := logEntry["message"]; !ok || msg != "handler log message" {
		t.Errorf("log message = %v, want %q", msg, "handler log message")
	}

	if reqID, ok := logEntry["request_id"]; !ok || reqID != "test-request-id" {
		t.Errorf("request_id = %v, want %q", reqID, "test-request-id")
	}
}

func TestTextFormat(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(Config{
		Output:   &buf,
		Level:    LevelDebug,
		Format:   FormatText,
		AppName:  "test-app",
		Hostname: "test-host",
	})

	logger.WithField("field", "value").Info("text format test")

	logOutput := buf.String()
	// Text format includes quotes around values in the format: key="value"
	for _, expected := range []string{
		"level=info",
		"msg=\"text format test\"",
		"field=\"value\"",
		"app=\"test-app\"",
	} {
		if !strings.Contains(logOutput, expected) {
			t.Errorf("expected log to contain %q, got %q", expected, logOutput)
		}
	}
}

func TestDevelopmentFormat(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(Config{
		Output:   &buf,
		Level:    LevelDebug,
		Format:   FormatDevelopment,
		AppName:  "test-app",
		Hostname: "test-host",
	})

	logger.WithField("field", "value").Info("development format test")

	logOutput := buf.String()
	// Development format should be colorized and readable
	if !strings.Contains(logOutput, "INFO") || !strings.Contains(logOutput, "development format test") {
		t.Errorf("development format missing expected content, got: %q", logOutput)
	}
}

func TestLoggerFromContext(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(Config{
		Output:   &buf,
		Level:    LevelDebug,
		Format:   FormatJSON,
		AppName:  "test-app",
		Hostname: "test-host",
	})

	// Create a context with the logger
	ctx := WithLogger(context.Background(), logger)

	// Retrieve and use the logger
	loggerFromCtx := FromContext(ctx)
	loggerFromCtx.WithField("test_field", "test_value").Info("logger from context")

	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse JSON log entry: %v", err)
	}

	if field, ok := logEntry["test_field"]; !ok || field != "test_value" {
		t.Errorf("test_field = %v, want %q", field, "test_value")
	}

	// Test default logger when context doesn't have one
	emptyCtx := context.Background()
	defaultLogger := FromContext(emptyCtx)
	if defaultLogger == nil {
		t.Error("FromContext with empty context should return default logger, got nil")
	}
}

func TestCloneWithFields(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(Config{
		Output:   &buf,
		Level:    LevelDebug,
		Format:   FormatJSON,
		AppName:  "test-app",
		Hostname: "test-host",
	})

	// Create a logger with some fields
	baseLogger := logger.WithField("base_field", "base_value")

	// Clone and add more fields
	clonedLogger := baseLogger.WithField("cloned_field", "cloned_value")

	// Use the cloned logger
	clonedLogger.Info("cloned logger test")

	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse JSON log entry: %v", err)
	}

	// Should have both fields
	if field, ok := logEntry["base_field"]; !ok || field != "base_value" {
		t.Errorf("base_field = %v, want %q", field, "base_value")
	}

	if field, ok := logEntry["cloned_field"]; !ok || field != "cloned_value" {
		t.Errorf("cloned_field = %v, want %q", field, "cloned_value")
	}

	// Now use the original logger - should only have the base field
	buf.Reset()
	baseLogger.Info("base logger test")

	logEntry = map[string]interface{}{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse JSON log entry: %v", err)
	}

	if field, ok := logEntry["base_field"]; !ok || field != "base_value" {
		t.Errorf("base_field = %v, want %q", field, "base_value")
	}

	if _, ok := logEntry["cloned_field"]; ok {
		t.Errorf("cloned_field should not be present in base logger output")
	}
}

// BenchmarkLogging benchmarks different logging operations
func BenchmarkLogging(b *testing.B) {
	benchmarks := []struct {
		name     string
		logLevel Level
		format   Format
		fn       func(Logger)
	}{
		{
			name:     "basic info log json",
			logLevel: LevelInfo,
			format:   FormatJSON,
			fn: func(l Logger) {
				l.Info("basic log message")
			},
		},
		{
			name:     "basic info log text",
			logLevel: LevelInfo,
			format:   FormatText,
			fn: func(l Logger) {
				l.Info("basic log message")
			},
		},
		{
			name:     "info log with fields json",
			logLevel: LevelInfo,
			format:   FormatJSON,
			fn: func(l Logger) {
				l.WithField("field1", "value1").
					WithField("field2", 42).
					WithField("field3", true).
					Info("log message with fields")
			},
		},
		{
			name:     "filtered out debug log",
			logLevel: LevelInfo,
			format:   FormatJSON,
			fn: func(l Logger) {
				l.Debug("debug message that should be filtered")
			},
		},
		{
			name:     "error log with error object",
			logLevel: LevelInfo,
			format:   FormatJSON,
			fn: func(l Logger) {
				err := &testError{message: "test error", stack: "test stack"}
				l.WithError(err).Error("error occurred")
			},
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			logger := NewLogger(Config{
				Output:   io.Discard, // Don't write to avoid I/O overhead
				Level:    bm.logLevel,
				Format:   bm.format,
				AppName:  "bench-app",
				Hostname: "bench-host",
			})

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				bm.fn(logger)
			}
		})
	}
}

// testError is a simple error implementation for testing
type testError struct {
	message string
	stack   string
}

func (e *testError) Error() string {
	return e.message
}
