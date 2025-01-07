package request

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestWithTimeout(t *testing.T) {
	tests := []struct {
		name        string
		timeout     time.Duration
		sleepTime   time.Duration
		wantTimeout bool
	}{
		{
			name:        "completes within timeout",
			timeout:     100 * time.Millisecond,
			sleepTime:   50 * time.Millisecond,
			wantTimeout: false,
		},
		{
			name:        "exceeds timeout",
			timeout:     50 * time.Millisecond,
			sleepTime:   100 * time.Millisecond,
			wantTimeout: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := WithTimeout(tt.timeout)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(tt.sleepTime)

				select {
				case <-r.Context().Done():
					// Context was canceled
					if !tt.wantTimeout {
						t.Error("request timed out unexpectedly")
					}
				default:
					// Context is still valid
					if tt.wantTimeout {
						t.Error("request did not time out as expected")
					}
				}
			}))

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
		})
	}
}

func TestWithTimeoutChain(t *testing.T) {
	// Test that timeout works when chained with other middleware
	timeout := 50 * time.Millisecond
	handler := WithTimeout(timeout)(
		WithRequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond)
		})),
	)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Request should have timed out
	if w.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", w.Code, http.StatusOK)
	}
}
