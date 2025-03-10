package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mcncl/buildkite-pubsub/internal/errors"
	"github.com/mcncl/buildkite-pubsub/internal/metrics"
	"github.com/mcncl/buildkite-pubsub/internal/publisher"
	"github.com/prometheus/client_golang/prometheus"
)

// MockPublisherWithError is a publisher that returns an error
type MockPublisherWithError struct {
	publisher.MockPublisher
	errorType string
}

func (m *MockPublisherWithError) Publish(ctx context.Context, data interface{}, attributes map[string]string) (string, error) {
	switch m.errorType {
	case "connection":
		return "", errors.NewConnectionError("connection refused")
	case "publish":
		return "", errors.NewPublishError("failed to publish message", fmt.Errorf("publish error"))
	case "rate_limit":
		return "", errors.NewRateLimitError("too many requests")
	default:
		return "", fmt.Errorf("unknown error")
	}
}

func TestHandler(t *testing.T) {
	tests := []struct {
		name          string
		method        string
		payload       string
		token         string
		publisherType string // normal, connection_error, publish_error, rate_limit
		wantStatus    int
		wantPublished bool
		wantEventType string
		wantErrorType string          // auth, validation, publish, etc.
		wantMetrics   map[string]bool // map of metric names to check existence
	}{
		{
			name:   "valid build request",
			method: http.MethodPost,
			payload: `{
				"event": "build.started",
				"build": {
					"id": "123",
					"url": "https://buildkite.com/test",
					"number": 1,
					"state": "started",
					"created_at": "2024-01-09T10:00:00Z",
					"started_at": "2024-01-09T10:00:10Z"
				},
				"pipeline": {
					"slug": "test",
					"name": "Test Pipeline"
				},
				"organization": {
					"slug": "org"
				}
			}`,
			token:         "test-token",
			publisherType: "normal",
			wantStatus:    http.StatusOK,
			wantPublished: true,
			wantEventType: "build.started",
			wantErrorType: "",
			wantMetrics: map[string]bool{
				"buildkite_webhook_requests_total":              true,
				"buildkite_webhook_request_duration_seconds":    true,
				"buildkite_message_size_bytes":                  true,
				"buildkite_payload_processing_duration_seconds": true,
				"buildkite_build_status_total":                  true,
				"buildkite_pipeline_builds_total":               true,
				"buildkite_queue_time_seconds":                  true,
				"buildkite_pubsub_message_size_bytes":           true,
				"buildkite_pubsub_publish_duration_seconds":     true,
				"buildkite_pubsub_publish_requests_total":       true,
			},
		},
		{
			name:          "invalid method",
			method:        http.MethodGet, // Using GET instead of POST
			payload:       "{}",
			token:         "test-token",
			publisherType: "normal",
			wantStatus:    http.StatusMethodNotAllowed, // 405
			wantPublished: false,
			wantErrorType: "validation", // We still classify it as validation error
			wantMetrics: map[string]bool{
				"buildkite_webhook_requests_total": true,
				"buildkite_errors_total":           true,
			},
		},
		{
			name:          "invalid token",
			method:        http.MethodPost,
			payload:       `{"event":"build.started"}`,
			token:         "wrong-token",
			publisherType: "normal",
			wantStatus:    http.StatusUnauthorized,
			wantPublished: false,
			wantErrorType: "auth",
			wantMetrics: map[string]bool{
				"buildkite_webhook_requests_total":      true,
				"buildkite_webhook_auth_failures_total": true,
				"buildkite_errors_total":                true,
			},
		},
		{
			name:          "invalid json",
			method:        http.MethodPost,
			payload:       `invalid json`,
			token:         "test-token",
			publisherType: "normal",
			wantStatus:    http.StatusBadRequest,
			wantPublished: false,
			wantErrorType: "validation",
			wantMetrics: map[string]bool{
				"buildkite_webhook_requests_total": true,
				"buildkite_errors_total":           true,
				"buildkite_message_size_bytes":     true,
			},
		},
		{
			name:          "ping event",
			method:        http.MethodPost,
			payload:       `{"event":"ping","service":{"id":"123"},"organization":{"id":"456"}}`,
			token:         "test-token",
			publisherType: "normal",
			wantStatus:    http.StatusOK,
			wantPublished: false,
			wantEventType: "ping",
			wantErrorType: "",
			wantMetrics: map[string]bool{
				"buildkite_webhook_requests_total":              true,
				"buildkite_message_size_bytes":                  true,
				"buildkite_payload_processing_duration_seconds": true,
			},
		},
		{
			name:   "connection error",
			method: http.MethodPost,
			payload: `{
				"event": "build.started",
				"build": {
					"id": "123",
					"url": "https://buildkite.com/test",
					"number": 1,
					"state": "started",
					"created_at": "2024-01-09T10:00:00Z",
					"started_at": "2024-01-09T10:00:10Z"
				},
				"pipeline": {
					"slug": "test",
					"name": "Test Pipeline"
				},
				"organization": {
					"slug": "org"
				}
			}`,
			token:         "test-token",
			publisherType: "connection_error",
			wantStatus:    http.StatusServiceUnavailable,
			wantPublished: false,
			wantEventType: "build.started",
			wantErrorType: "connection",
			wantMetrics: map[string]bool{
				"buildkite_webhook_requests_total":              true,
				"buildkite_message_size_bytes":                  true,
				"buildkite_payload_processing_duration_seconds": true,
				"buildkite_errors_total":                        true,
				"buildkite_pubsub_publish_requests_total":       true,
			},
		},
		{
			name:   "publish error",
			method: http.MethodPost,
			payload: `{
				"event": "build.started",
				"build": {
					"id": "123",
					"url": "https://buildkite.com/test",
					"number": 1,
					"state": "started",
					"created_at": "2024-01-09T10:00:00Z",
					"started_at": "2024-01-09T10:00:10Z"
				},
				"pipeline": {
					"slug": "test",
					"name": "Test Pipeline"
				},
				"organization": {
					"slug": "org"
				}
			}`,
			token:         "test-token",
			publisherType: "publish_error",
			wantStatus:    http.StatusInternalServerError,
			wantPublished: false,
			wantEventType: "build.started",
			wantErrorType: "publish",
			wantMetrics: map[string]bool{
				"buildkite_webhook_requests_total":              true,
				"buildkite_message_size_bytes":                  true,
				"buildkite_payload_processing_duration_seconds": true,
				"buildkite_errors_total":                        true,
				"buildkite_pubsub_publish_requests_total":       true,
			},
		},
		{
			name:   "rate limit error",
			method: http.MethodPost,
			payload: `{
				"event": "build.started",
				"build": {
					"id": "123",
					"url": "https://buildkite.com/test",
					"number": 1,
					"state": "started",
					"created_at": "2024-01-09T10:00:00Z",
					"started_at": "2024-01-09T10:00:10Z"
				},
				"pipeline": {
					"slug": "test",
					"name": "Test Pipeline"
				},
				"organization": {
					"slug": "org"
				}
			}`,
			token:         "test-token",
			publisherType: "rate_limit",
			wantStatus:    http.StatusTooManyRequests,
			wantPublished: false,
			wantEventType: "build.started",
			wantErrorType: "rate_limit",
			wantMetrics: map[string]bool{
				"buildkite_webhook_requests_total":              true,
				"buildkite_message_size_bytes":                  true,
				"buildkite_payload_processing_duration_seconds": true,
				"buildkite_errors_total":                        true,
				"buildkite_pubsub_publish_requests_total":       true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test registry and gatherer
			reg := prometheus.NewRegistry()
			prometheus.DefaultRegisterer = reg
			prometheus.DefaultGatherer = reg

			// Initialize metrics with test registry
			err := metrics.InitMetrics(reg)
			if err != nil {
				t.Fatalf("failed to initialize metrics: %v", err)
			}

			// Create the appropriate publisher
			var pub publisher.Publisher
			switch tt.publisherType {
			case "normal":
				pub = publisher.NewMockPublisher()
			case "connection_error":
				pub = &MockPublisherWithError{errorType: "connection"}
			case "publish_error":
				pub = &MockPublisherWithError{errorType: "publish"}
			case "rate_limit":
				pub = &MockPublisherWithError{errorType: "rate_limit"}
			default:
				pub = publisher.NewMockPublisher()
			}

			// Create handler with the expected token
			handler := NewHandler(Config{
				BuildkiteToken: "test-token", // This should match tt.token for valid cases
				Publisher:      pub,
			})

			// Create request
			req := httptest.NewRequest(tt.method, "/webhook", bytes.NewBufferString(tt.payload))
			if tt.token != "" {
				req.Header.Set("X-Buildkite-Token", tt.token)
			}
			req.Header.Set("Content-Type", "application/json")

			// Record response
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			// Check status code
			if w.Code != tt.wantStatus {
				t.Errorf("Handler returned wrong status code: got %v want %v", w.Code, tt.wantStatus)
			}

			// Check error type if expected
			if tt.wantErrorType != "" {
				// Parse response for error
				var errorResp map[string]interface{}
				if err := json.NewDecoder(w.Body).Decode(&errorResp); err != nil {
					t.Errorf("Failed to decode error response: %v", err)
				} else {
					if errorType, ok := errorResp["error_type"]; !ok || errorType != tt.wantErrorType {
						t.Errorf("Response has wrong error type: got %v want %v", errorType, tt.wantErrorType)
					}
				}
			}

			// Check publication status
			if tt.publisherType == "normal" {
				mp := pub.(*publisher.MockPublisher)
				lastPub := mp.LastPublished()
				hasPublished := lastPub != nil
				if hasPublished != tt.wantPublished {
					t.Errorf("Handler published = %v, want %v", hasPublished, tt.wantPublished)
				}

				// Additional checks for successful publishes
				if tt.wantPublished && lastPub != nil {
					// Check event type
					if attrs, ok := lastPub.Attributes["event_type"]; ok {
						if attrs != tt.wantEventType {
							t.Errorf("Handler published wrong event type: got %v want %v",
								attrs, tt.wantEventType)
						}
					}

					// Verify response structure
					var response struct {
						Status    string `json:"status"`
						Message   string `json:"message"`
						MessageID string `json:"message_id"`
					}
					if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
						t.Errorf("Failed to decode response: %v", err)
					}
					if response.MessageID != "mock-message-id" {
						t.Errorf("Handler returned wrong message ID: got %v want mock-message-id",
							response.MessageID)
					}
				}
			}

			// Check metrics
			for metricName, shouldExist := range tt.wantMetrics {
				if shouldExist {
					if !metricExists(metricName) {
						t.Errorf("Expected metric %s not found", metricName)
					}
				}
			}
		})
	}
}

// Helper function to check if a metric exists
func metricExists(metricName string) bool {
	metrics, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		return false
	}

	for _, mf := range metrics {
		if mf.GetName() == metricName {
			for _, m := range mf.GetMetric() {
				if m.GetCounter() != nil && m.GetCounter().GetValue() > 0 {
					return true
				}
				if m.GetGauge() != nil && m.GetGauge().GetValue() > 0 {
					return true
				}
				if m.GetHistogram() != nil && m.GetHistogram().GetSampleCount() > 0 {
					return true
				}
			}
		}
	}
	return false
}
