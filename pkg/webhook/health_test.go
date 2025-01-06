package webhook

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthCheck(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		setReady     bool
		wantStatus   int
		wantResponse map[string]string
	}{
		{
			name:       "health check returns healthy",
			path:       "/health",
			setReady:   false, // health should return ok regardless of ready state
			wantStatus: http.StatusOK,
			wantResponse: map[string]string{
				"status": "healthy",
			},
		},
		{
			name:       "readiness check when ready",
			path:       "/ready",
			setReady:   true,
			wantStatus: http.StatusOK,
			wantResponse: map[string]string{
				"status": "ready",
			},
		},
		{
			name:         "readiness check when not ready",
			path:         "/ready",
			setReady:     false,
			wantStatus:   http.StatusServiceUnavailable,
			wantResponse: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create health check instance
			hc := NewHealthCheck()
			hc.SetReady(tt.setReady)

			// Create request
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()

			// Call appropriate handler
			switch tt.path {
			case "/health":
				hc.HealthHandler(w, req)
			case "/ready":
				hc.ReadyHandler(w, req)
			}

			// Check status code
			if w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.wantStatus)
			}

			// For successful responses, check the body
			if tt.wantResponse != nil {
				var got map[string]string
				if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if got["status"] != tt.wantResponse["status"] {
					t.Errorf("got response %v, want %v", got, tt.wantResponse)
				}
			}
		})
	}
}

func TestHealthCheckConcurrency(t *testing.T) {
	hc := NewHealthCheck()

	// Test concurrent access to ready state
	go func() {
		for i := 0; i < 100; i++ {
			hc.SetReady(true)
		}
	}()

	go func() {
		for i := 0; i < 100; i++ {
			hc.SetReady(false)
		}
	}()

	// Concurrent reads
	for i := 0; i < 100; i++ {
		req := httptest.NewRequest(http.MethodGet, "/ready", nil)
		w := httptest.NewRecorder()
		hc.ReadyHandler(w, req)
		// We don't check the specific response as it could be either ready or not
		// Just ensure we don't have any race conditions
	}
}
