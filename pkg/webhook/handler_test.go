package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

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
					"branch": "main",
					"commit": "abc123",
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
					// Check event type attribute
					if attrs, ok := lastPub.Attributes["event_type"]; ok {
						if attrs != tt.wantEventType {
							t.Errorf("Handler published wrong event type: got %v want %v",
								attrs, tt.wantEventType)
						}
					} else {
						t.Error("Missing event_type attribute")
					}

					// Check origin attribute
					if origin, ok := lastPub.Attributes["origin"]; !ok || origin != "buildkite-webhook" {
						t.Errorf("Handler published wrong origin: got %v want buildkite-webhook", origin)
					}

					// Check pipeline attribute
					if pipeline, ok := lastPub.Attributes["pipeline"]; !ok {
						t.Error("Missing pipeline attribute")
					} else if pipeline != "Test Pipeline" {
						t.Errorf("Handler published wrong pipeline: got %v want Test Pipeline", pipeline)
					}

					// Check build_state attribute
					if state, ok := lastPub.Attributes["build_state"]; !ok {
						t.Error("Missing build_state attribute")
					} else if state != "started" {
						t.Errorf("Handler published wrong build_state: got %v want started", state)
					}

					// Check branch attribute
					if branch, ok := lastPub.Attributes["branch"]; !ok {
						t.Error("Missing branch attribute")
					} else if branch != "main" {
						t.Errorf("Handler published wrong branch: got %v want main", branch)
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

// TestHandlerPublishAttributes verifies all Pub/Sub attributes are set correctly
func TestHandlerPublishAttributes(t *testing.T) {
	// Setup test registry
	reg := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = reg
	prometheus.DefaultGatherer = reg

	if err := metrics.InitMetrics(reg); err != nil {
		t.Fatalf("failed to initialize metrics: %v", err)
	}

	// Create mock publisher
	mockPub := publisher.NewMockPublisher()

	// Create handler
	handler := NewHandler(Config{
		BuildkiteToken: "test-token",
		Publisher:      mockPub,
	})

	// Test payload with various attributes
	payload := `{
		"event": "build.finished",
		"build": {
			"id": "test-build-123",
			"url": "https://api.buildkite.com/v2/organizations/test-org/pipelines/production-deploy/builds/456",
			"number": 456,
			"state": "failed",
			"branch": "release/v2.0",
			"commit": "def456abc123",
			"created_at": "2024-01-09T10:00:00Z",
			"started_at": "2024-01-09T10:01:00Z",
			"finished_at": "2024-01-09T10:15:00Z"
		},
		"pipeline": {
			"slug": "production-deploy",
			"name": "Production Deployment"
		},
		"organization": {
			"slug": "test-org"
		}
	}`

	// Create and execute request
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBufferString(payload))
	req.Header.Set("X-Buildkite-Token", "test-token")
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Verify status
	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	// Get published message
	mp := mockPub.(*publisher.MockPublisher)
	lastPub := mp.LastPublished()
	if lastPub == nil {
		t.Fatal("Expected message to be published")
	}

	// Verify all attributes are present and correct
	expectedAttrs := map[string]string{
		"origin":      "buildkite-webhook",
		"event_type":  "build.finished",
		"pipeline":    "Production Deployment",
		"build_state": "failed",
		"branch":      "release/v2.0",
	}

	for key, expectedValue := range expectedAttrs {
		actualValue, exists := lastPub.Attributes[key]
		if !exists {
			t.Errorf("Missing required attribute: %s", key)
			continue
		}
		if actualValue != expectedValue {
			t.Errorf("Attribute %s: expected %q, got %q", key, expectedValue, actualValue)
		}
	}

	// Verify no unexpected attributes (optional, but good practice)
	for key := range lastPub.Attributes {
		if _, expected := expectedAttrs[key]; !expected {
			t.Errorf("Unexpected attribute: %s", key)
		}
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

// Helper function to generate HMAC signature for testing
func generateTestHMACSignature(secret, timestamp, body string) string {
	message := fmt.Sprintf("%s.%s", timestamp, body)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

func TestHandlerWithHMACSignature(t *testing.T) {
	hmacSecret := "test-hmac-secret"
	payload := `{
		"event": "build.started",
		"build": {
			"id": "123",
			"url": "https://buildkite.com/test",
			"number": 1,
			"state": "started",
			"branch": "main",
			"commit": "abc123",
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
	}`

	tests := []struct {
		name          string
		hmacSecret    string
		timestamp     string
		generateSig   bool
		customSig     string
		wantStatus    int
		wantPublished bool
	}{
		{
			name:          "valid HMAC signature",
			hmacSecret:    hmacSecret,
			timestamp:     strconv.FormatInt(time.Now().Unix(), 10),
			generateSig:   true,
			wantStatus:    http.StatusOK,
			wantPublished: true,
		},
		{
			name:          "invalid HMAC signature",
			hmacSecret:    hmacSecret,
			timestamp:     strconv.FormatInt(time.Now().Unix(), 10),
			generateSig:   false,
			customSig:     "invalid-signature",
			wantStatus:    http.StatusUnauthorized,
			wantPublished: false,
		},
		{
			name:          "expired timestamp",
			hmacSecret:    hmacSecret,
			timestamp:     strconv.FormatInt(time.Now().Unix()-400, 10), // 400 seconds ago
			generateSig:   true,
			wantStatus:    http.StatusUnauthorized,
			wantPublished: false,
		},
		{
			name:          "future timestamp",
			hmacSecret:    hmacSecret,
			timestamp:     strconv.FormatInt(time.Now().Unix()+400, 10), // 400 seconds in future
			generateSig:   true,
			wantStatus:    http.StatusUnauthorized,
			wantPublished: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test registry
			reg := prometheus.NewRegistry()
			prometheus.DefaultRegisterer = reg
			prometheus.DefaultGatherer = reg

			if err := metrics.InitMetrics(reg); err != nil {
				t.Fatalf("failed to initialize metrics: %v", err)
			}

			// Create mock publisher
			mockPub := publisher.NewMockPublisher()

			// Create handler with HMAC secret
			handler := NewHandler(Config{
				BuildkiteToken: "", // No token, using HMAC
				HMACSecret:     tt.hmacSecret,
				Publisher:      mockPub,
			})

			// Generate or use custom signature
			var signature string
			if tt.generateSig {
				signature = generateTestHMACSignature(tt.hmacSecret, tt.timestamp, payload)
			} else {
				signature = tt.customSig
			}

			headerValue := fmt.Sprintf("timestamp=%s,signature=%s", tt.timestamp, signature)

			// Create request
			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBufferString(payload))
			req.Header.Set("X-Buildkite-Signature", headerValue)
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
		})
	}
}

func TestHandlerHMACAndTokenFallback(t *testing.T) {
	token := "test-token"
	hmacSecret := "test-hmac-secret"
	payload := `{
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
	}`

	tests := []struct {
		name          string
		useHMAC       bool
		useToken      bool
		tokenValue    string
		wantStatus    int
		wantPublished bool
	}{
		{
			name:          "HMAC takes precedence when both present",
			useHMAC:       true,
			useToken:      true,
			tokenValue:    token,
			wantStatus:    http.StatusOK,
			wantPublished: true,
		},
		{
			name:          "Falls back to token when no HMAC",
			useHMAC:       false,
			useToken:      true,
			tokenValue:    token,
			wantStatus:    http.StatusOK,
			wantPublished: true,
		},
		{
			name:          "Token fails when wrong and no HMAC",
			useHMAC:       false,
			useToken:      true,
			tokenValue:    "wrong-token",
			wantStatus:    http.StatusUnauthorized,
			wantPublished: false,
		},
		{
			name:          "Fails when neither present",
			useHMAC:       false,
			useToken:      false,
			wantStatus:    http.StatusUnauthorized,
			wantPublished: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test registry
			reg := prometheus.NewRegistry()
			prometheus.DefaultRegisterer = reg
			prometheus.DefaultGatherer = reg

			if err := metrics.InitMetrics(reg); err != nil {
				t.Fatalf("failed to initialize metrics: %v", err)
			}

			// Create mock publisher
			mockPub := publisher.NewMockPublisher()

			// Create handler with both token and HMAC secret
			handler := NewHandler(Config{
				BuildkiteToken: token,
				HMACSecret:     hmacSecret,
				Publisher:      mockPub,
			})

			// Create request
			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBufferString(payload))

			// Add HMAC signature if requested
			if tt.useHMAC {
				timestamp := strconv.FormatInt(time.Now().Unix(), 10)
				signature := generateTestHMACSignature(hmacSecret, timestamp, payload)
				headerValue := fmt.Sprintf("timestamp=%s,signature=%s", timestamp, signature)
				req.Header.Set("X-Buildkite-Signature", headerValue)
			}

			// Add token if requested
			if tt.useToken {
				req.Header.Set("X-Buildkite-Token", tt.tokenValue)
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
		})
	}
}
