package webhook

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mcncl/buildkite-pubsub/internal/publisher"
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
	}{
		{
			name:          "valid request",
			method:        http.MethodPost,
			payload:       `{"event":"build.started","build":{"id":"123","url":"https://buildkite.com/test","number":1,"state":"started"},"pipeline":{"slug":"test"},"organization":{"slug":"org"}}`,
			token:         "test-token",
			wantStatus:    http.StatusOK,
			wantPublished: true,
			wantEventType: "build.started",
		},
		{
			name:          "invalid method",
			method:        http.MethodGet,
			payload:       "{}",
			token:         "test-token",
			wantStatus:    http.StatusMethodNotAllowed,
			wantPublished: false,
		},
		{
			name:          "invalid token",
			method:        http.MethodPost,
			payload:       `{"event":"build.started"}`,
			token:         "wrong-token",
			wantStatus:    http.StatusUnauthorized,
			wantPublished: false,
		},
		{
			name:          "no token",
			method:        http.MethodPost,
			payload:       `{"event":"build.started"}`,
			token:         "",
			wantStatus:    http.StatusUnauthorized,
			wantPublished: false,
		},
		{
			name:          "invalid json",
			method:        http.MethodPost,
			payload:       `invalid json`,
			token:         "test-token",
			wantStatus:    http.StatusBadRequest,
			wantPublished: false,
		},
		{
			name:          "ping event",
			method:        http.MethodPost,
			payload:       `{"event":"ping","service":{"id":"123"},"organization":{"id":"456"}}`,
			token:         "test-token",
			wantStatus:    http.StatusOK,
			wantPublished: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mock publisher before each test
			mockPub := publisher.NewMockPublisher()
			handler := NewHandler(Config{
				BuildkiteToken: "test-token",
				Publisher:      mockPub,
			})

			// Create request
			req := httptest.NewRequest(tt.method, "/webhook", bytes.NewBufferString(tt.payload))

			// Set token header
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

			// Get mock publisher for type assertion
			mp := mockPub.(*publisher.MockPublisher)

			// Check publication status
			lastPub := mp.LastPublished()
			hasPublished := lastPub != nil
			if hasPublished != tt.wantPublished {
				t.Errorf("Handler published = %v, want %v", hasPublished, tt.wantPublished)
			}

			// For successful publishes, verify the event type
			if tt.wantPublished && lastPub != nil {
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
