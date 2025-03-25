package logging

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mcncl/buildkite-pubsub/internal/logging"
	"github.com/mcncl/buildkite-pubsub/internal/middleware/request"
)

func TestWithStructuredLogging(t *testing.T) {
	tests := []struct {
		name           string
		requestID      string
		method         string
		path           string
		requestHandler func(w http.ResponseWriter, r *http.Request)
		wantStatus     int
		wantLogFields  map[string]interface{}
	}{
		{
			name:      "logs successful request",
			requestID: "test-id",
			method:    http.MethodGet,
			path:      "/test",
			requestHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte("success"))
				if err != nil {
					t.Fatalf("expected success, got %v", err)
				}
			},
			wantStatus: http.StatusOK,
			wantLogFields: map[string]interface{}{
				"request_id": "test-id",
				"method":     "GET",
				"path":       "/test",
				"status":     float64(200), // JSON numbers are parsed as float64
			},
		},
		{
			name:      "logs error request",
			requestID: "error-id",
			method:    http.MethodPost,
			path:      "/error",
			requestHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				_, err := w.Write([]byte("error"))
				if err != nil {
					t.Fatalf("expected err, got %v", err)
				}
			},
			wantStatus: http.StatusBadRequest,
			wantLogFields: map[string]interface{}{
				"request_id": "error-id",
				"method":     "POST",
				"path":       "/error",
				"status":     float64(400),
			},
		},
		{
			name:   "handles missing request ID",
			method: http.MethodGet,
			path:   "/test",
			requestHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			wantStatus: http.StatusOK,
			wantLogFields: map[string]interface{}{
				"method": "GET",
				"path":   "/test",
				"status": float64(200),
			},
		},
		{
			name:      "logs from handler context",
			requestID: "context-id",
			method:    http.MethodGet,
			path:      "/context",
			requestHandler: func(w http.ResponseWriter, r *http.Request) {
				// Get logger from context and add more logs
				logger := logging.FromContext(r.Context())
				logger.WithField("custom_field", "custom_value").Info("Handler log message")
				w.WriteHeader(http.StatusCreated)
			},
			wantStatus: http.StatusCreated,
			wantLogFields: map[string]interface{}{
				"request_id": "context-id",
				"method":     "GET",
				"path":       "/context",
				"status":     float64(201),
				// custom_field will be checked separately in the handler log
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture log output
			var buf bytes.Buffer
			logger := logging.NewLogger(logging.Config{
				Output:   &buf,
				Level:    logging.LevelDebug,
				Format:   logging.FormatJSON,
				AppName:  "test-app",
				Hostname: "test-host",
			})

			// Create handler with middleware
			handler := WithStructuredLogging(logger)(http.HandlerFunc(tt.requestHandler))

			// Create request
			req := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.requestID != "" {
				req.Header.Set(request.RequestIDHeader, tt.requestID)
			}

			// Record response
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			// Check status code
			if w.Code != tt.wantStatus {
				t.Errorf("Status code = %d, want %d", w.Code, tt.wantStatus)
			}

			// Parse log output - we expect at least two log entries (request start and complete)
			logOutput := buf.String()
			logLines := strings.Split(strings.TrimSpace(logOutput), "\n")
			if len(logLines) < 2 {
				t.Fatalf("Expected at least 2 log lines, got %d: %s", len(logLines), logOutput)
			}

			// Check the completion log entry (last one)
			var completionLog map[string]interface{}
			if err := json.Unmarshal([]byte(logLines[len(logLines)-1]), &completionLog); err != nil {
				t.Fatalf("Failed to parse completion log JSON: %v", err)
			}

			// Verify expected fields in completion log
			for field, want := range tt.wantLogFields {
				if got, ok := completionLog[field]; !ok {
					t.Errorf("Completion log missing field %q", field)
				} else if got != want {
					t.Errorf("Completion log field %q = %v, want %v", field, got, want)
				}
			}

			// Verify common fields
			if msg, ok := completionLog["message"]; !ok || msg != "Request completed" {
				t.Errorf("Expected 'Request completed' message, got %v", msg)
			}

			if _, ok := completionLog["duration_ms"]; !ok {
				t.Errorf("Log missing 'duration_ms' field")
			}

			// Check for context logging if applicable
			if tt.name == "logs from handler context" {
				// Find the handler's log message
				var handlerLog map[string]interface{}
				var foundHandlerLog bool

				for _, line := range logLines {
					var entry map[string]interface{}
					if err := json.Unmarshal([]byte(line), &entry); err != nil {
						continue
					}

					if msg, ok := entry["message"].(string); ok && msg == "Handler log message" {
						handlerLog = entry
						foundHandlerLog = true
						break
					}
				}

				if !foundHandlerLog {
					t.Error("Expected to find handler log message")
				} else {
					// Check that the custom field is present in handler log
					if value, ok := handlerLog["custom_field"].(string); !ok || value != "custom_value" {
						t.Errorf("Handler log missing or has incorrect custom_field, got %v", value)
					}
				}
			}
		})
	}
}
