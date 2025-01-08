package webhook

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mcncl/buildkite-pubsub/internal/metrics"
	"github.com/mcncl/buildkite-pubsub/internal/publisher"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestHandler(t *testing.T) {
	tests := []struct {
		name          string
		method        string
		payload       string
		token         string
		wantStatus    int
		wantPublished bool
		wantEventType string
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
			wantStatus:    http.StatusOK,
			wantPublished: true,
			wantEventType: "build.started",
			wantMetrics: map[string]bool{
				"buildkite_webhook_requests_total":                 true,
				"buildkite_webhook_request_duration_seconds":       true,
				"buildkite_message_size_bytes":                    true,
				"buildkite_payload_processing_duration_seconds":    true,
				"buildkite_build_status_total":                    true,
				"buildkite_pipeline_builds_total":                 true,
				"buildkite_queue_time_seconds":                    true,
				"buildkite_pubsub_message_size_bytes":            true,
				"buildkite_pubsub_publish_duration_seconds":       true,
				"buildkite_pubsub_publish_requests_total":         true,
			},
		},
		{
			name:          "invalid method",
			method:        http.MethodGet,
			payload:       "{}",
			token:         "test-token",
			wantStatus:    http.StatusMethodNotAllowed,
			wantPublished: false,
			wantMetrics: map[string]bool{
				"buildkite_webhook_requests_total": true,
				"buildkite_errors_total":          true,
			},
		},
		{
			name:          "invalid token",
			method:        http.MethodPost,
			payload:       `{"event":"build.started"}`,
			token:         "wrong-token",
			wantStatus:    http.StatusUnauthorized,
			wantPublished: false,
			wantMetrics: map[string]bool{
				"buildkite_webhook_requests_total":     true,
				"buildkite_webhook_auth_failures_total": true,
				"buildkite_errors_total":               true,
			},
		},
		{
			name:          "invalid json",
			method:        http.MethodPost,
			payload:       `invalid json`,
			token:         "test-token",
			wantStatus:    http.StatusBadRequest,
			wantPublished: false,
			wantMetrics: map[string]bool{
				"buildkite_webhook_requests_total": true,
				"buildkite_errors_total":          true,
				"buildkite_message_size_bytes":    true,
			},
		},
		{
			name:          "ping event",
			method:        http.MethodPost,
			payload:       `{"event":"ping","service":{"id":"123"},"organization":{"id":"456"}}`,
			token:         "test-token",
			wantStatus:    http.StatusOK,
			wantPublished: false,
			wantEventType: "ping",
			wantMetrics: map[string]bool{
				"buildkite_webhook_requests_total":              true,
				"buildkite_message_size_bytes":                 true,
				"buildkite_payload_processing_duration_seconds": true,
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
			metrics.InitMetrics(reg)

			// Create handler
			mockPub := publisher.NewMockPublisher()
			handler := NewHandler(Config{
				BuildkiteToken: "test-token",
				Publisher:     mockPub,
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

			// Check publication status
			mp := mockPub.(*publisher.MockPublisher)
			lastPub := mp.LastPublished()
			hasPublished := lastPub != nil
			if hasPublished != tt.wantPublished {
				t.Errorf("Handler published = %v, want %v", hasPublished, tt.wantPublished)
			}

			// Check metrics
			for metricName, shouldExist := range tt.wantMetrics {
				if shouldExist {
					if !metricExists(metricName) {
						t.Errorf("Expected metric %s not found", metricName)
					}
				}
			}

			// Check additional details for successful publishes
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

// Helper function to get metric value
func getMetricValue(metric prometheus.Metric) float64 {
	var m dto.Metric
	metric.Write(&m)
	if m.Counter != nil {
		return m.Counter.GetValue()
	}
	if m.Gauge != nil {
		return m.Gauge.GetValue()
	}
	if m.Histogram != nil {
		return float64(m.Histogram.GetSampleCount())
	}
	return 0
}
