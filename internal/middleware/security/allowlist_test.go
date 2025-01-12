package security

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mcncl/buildkite-pubsub/internal/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

func TestIPAllowList(t *testing.T) {
	// Initialize metrics for tests
	reg := prometheus.NewRegistry()
	err := metrics.InitMetrics(reg)
	if err != nil {
		t.Fatalf("failed to initialize metrics: %v", err)
	}

	tests := []struct {
		name           string
		remoteAddr     string
		forwardedFor   string
		allowedIPs     []string
		wantStatusCode int
	}{
		{
			name:           "allowed IP direct",
			remoteAddr:     "100.24.182.113:12345",
			allowedIPs:     []string{"100.24.182.113", "35.172.45.249"},
			wantStatusCode: http.StatusOK,
		},
		{
			name:           "allowed IP via X-Forwarded-For",
			remoteAddr:     "10.0.0.1:12345",
			forwardedFor:   "100.24.182.113",
			allowedIPs:     []string{"100.24.182.113", "35.172.45.249"},
			wantStatusCode: http.StatusOK,
		},
		{
			name:           "forbidden IP direct",
			remoteAddr:     "1.2.3.4:12345",
			allowedIPs:     []string{"100.24.182.113", "35.172.45.249"},
			wantStatusCode: http.StatusForbidden,
		},
		{
			name:           "forbidden IP via X-Forwarded-For",
			remoteAddr:     "10.0.0.1:12345",
			forwardedFor:   "1.2.3.4",
			allowedIPs:     []string{"100.24.182.113", "35.172.45.249"},
			wantStatusCode: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create AllowList with test IPs
			wl := &IPAllowList{
				allowedIPs: make(map[string]struct{}),
			}
			for _, ip := range tt.allowedIPs {
				wl.allowedIPs[ip] = struct{}{}
			}

			// Create test handler
			handler := wl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			// Create request
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.forwardedFor != "" {
				req.Header.Set("X-Forwarded-For", tt.forwardedFor)
			}

			// Record response
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != tt.wantStatusCode {
				t.Errorf("got status %d, want %d", w.Code, tt.wantStatusCode)
			}
		})
	}
}

func TestIPAllowListConcurrency(t *testing.T) {
	// Initialize metrics for tests
	reg := prometheus.NewRegistry()
	metrics.InitMetrics(reg)

	wl := &IPAllowList{
		allowedIPs: map[string]struct{}{
			"100.24.182.113": {},
		},
	}

	// Run concurrent requests
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.RemoteAddr = "100.24.182.113:12345"
			w := httptest.NewRecorder()

			handler := wl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			handler.ServeHTTP(w, req)
			done <- true
		}()
	}

	// Wait for all goroutines to finish
	for i := 0; i < 10; i++ {
		<-done
	}
}
