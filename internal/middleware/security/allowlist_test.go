package security

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestIPAllowList(t *testing.T) {
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
		{
			name:           "multiple IPs in X-Forwarded-For, first one allowed",
			remoteAddr:     "10.0.0.1:12345",
			forwardedFor:   "100.24.182.113, 1.2.3.4",
			allowedIPs:     []string{"100.24.182.113", "35.172.45.249"},
			wantStatusCode: http.StatusOK,
		},
		{
			name:           "malformed IP",
			remoteAddr:     "invalid-ip:12345",
			allowedIPs:     []string{"100.24.182.113", "35.172.45.249"},
			wantStatusCode: http.StatusForbidden,
		},
		{
			name:           "multiple IPs in X-Forwarded-For, first one allowed",
			remoteAddr:     "10.0.0.1:12345",
			forwardedFor:   "100.24.182.113, 1.2.3.4",
			allowedIPs:     []string{"100.24.182.113", "35.172.45.249"},
			wantStatusCode: http.StatusOK,
		},
		{
			name:           "multiple IPs with spaces in X-Forwarded-For, first one allowed",
			remoteAddr:     "10.0.0.1:12345",
			forwardedFor:   "100.24.182.113,    1.2.3.4,   5.6.7.8",
			allowedIPs:     []string{"100.24.182.113", "35.172.45.249"},
			wantStatusCode: http.StatusOK,
		},
		{
			name:           "multiple IPs in X-Forwarded-For, first one forbidden",
			remoteAddr:     "10.0.0.1:12345",
			forwardedFor:   "1.2.3.4, 100.24.182.113",
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

			// Create test request
			req := httptest.NewRequest(http.MethodGet, "/webhook", nil)
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

// MockBuildkiteAPI implements a test double for the Buildkite API
type mockBuildkiteAPI struct {
	server *httptest.Server
	ips    []string
}

func newMockBuildkiteAPI(ips []string) *mockBuildkiteAPI {
	mock := &mockBuildkiteAPI{
		ips: ips,
	}
	mock.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"webhook_ips":["100.24.182.113","35.172.45.249","54.85.125.32"]}`))
	}))
	return mock
}

func TestIPAllowListRefresh(t *testing.T) {
	// Create mock API
	mockAPI := newMockBuildkiteAPI([]string{
		"100.24.182.113",
		"35.172.45.249",
		"54.85.125.32",
	})
	defer mockAPI.server.Close()

	// Create AllowList
	wl := &IPAllowList{
		allowedIPs: make(map[string]struct{}),
	}

	// Test initial refresh
	err := wl.refreshIPs()
	if err != nil {
		t.Fatalf("refreshIPs() error = %v", err)
	}

	// Check that IPs were loaded
	expectedIPs := []string{
		"100.24.182.113",
		"35.172.45.249",
		"54.85.125.32",
	}
	for _, ip := range expectedIPs {
		if _, exists := wl.allowedIPs[ip]; !exists {
			t.Errorf("IP %s not found in AllowList", ip)
		}
	}

	// Check last update time
	if time.Since(wl.lastUpdate) > time.Second {
		t.Error("lastUpdate not set correctly")
	}
}

func TestIPAllowListConcurrency(t *testing.T) {
	wl := &IPAllowList{
		allowedIPs: map[string]struct{}{
			"100.24.182.113": {},
		},
	}

	// Run concurrent requests
	for i := 0; i < 100; i++ {
		go func() {
			req := httptest.NewRequest(http.MethodGet, "/webhook", nil)
			req.RemoteAddr = "100.24.182.113:12345"
			w := httptest.NewRecorder()

			handler := wl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			handler.ServeHTTP(w, req)
		}()
	}
}
