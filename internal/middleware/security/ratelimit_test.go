package security

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestWithRateLimit(t *testing.T) {
	tests := []struct {
		name              string
		requestsPerMinute int
		numRequests       int
		wantAllowed       int
	}{
		{
			name:              "respects rate limit",
			requestsPerMinute: 5,
			numRequests:       10,
			wantAllowed:       5,
		},
		{
			name:              "allows all under limit",
			requestsPerMinute: 10,
			numRequests:       5,
			wantAllowed:       5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := WithRateLimit(tt.requestsPerMinute)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			allowed := 0
			for i := 0; i < tt.numRequests; i++ {
				req := httptest.NewRequest(http.MethodGet, "/test", nil)
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, req)
				if w.Code == http.StatusOK {
					allowed++
				}
			}

			if allowed != tt.wantAllowed {
				t.Errorf("got %d requests allowed, want %d", allowed, tt.wantAllowed)
			}
		})
	}
}

func TestWithIPRateLimit(t *testing.T) {
	tests := []struct {
		name              string
		requestsPerMinute int
		requests          []struct {
			ip        string
			forwarded string
		}
		wantAllowedPerIP map[string]int
	}{
		{
			name:              "limits per IP",
			requestsPerMinute: 2,
			requests: []struct {
				ip        string
				forwarded string
			}{
				{ip: "1.1.1.1:1234", forwarded: ""},
				{ip: "1.1.1.1:1234", forwarded: ""},
				{ip: "1.1.1.1:1234", forwarded: ""},
				{ip: "2.2.2.2:1234", forwarded: ""},
				{ip: "2.2.2.2:1234", forwarded: ""},
				{ip: "2.2.2.2:1234", forwarded: ""},
			},
			wantAllowedPerIP: map[string]int{
				"1.1.1.1": 2,
				"2.2.2.2": 2,
			},
		},
		{
			name:              "handles X-Forwarded-For",
			requestsPerMinute: 2,
			requests: []struct {
				ip        string
				forwarded string
			}{
				{ip: "10.0.0.1:1234", forwarded: "1.1.1.1"},
				{ip: "10.0.0.1:1234", forwarded: "1.1.1.1"},
				{ip: "10.0.0.1:1234", forwarded: "1.1.1.1"},
				{ip: "10.0.0.1:1234", forwarded: "2.2.2.2"},
				{ip: "10.0.0.1:1234", forwarded: "2.2.2.2"},
				{ip: "10.0.0.1:1234", forwarded: "2.2.2.2"},
			},
			wantAllowedPerIP: map[string]int{
				"1.1.1.1": 2,
				"2.2.2.2": 2,
			},
		},
		{
			name:              "handles multiple IPs in X-Forwarded-For",
			requestsPerMinute: 2,
			requests: []struct {
				ip        string
				forwarded string
			}{
				{ip: "10.0.0.1:1234", forwarded: "1.1.1.1, 192.168.1.1"},
				{ip: "10.0.0.1:1234", forwarded: "1.1.1.1, 192.168.1.2"},
				{ip: "10.0.0.1:1234", forwarded: "1.1.1.1, 192.168.1.3"},
			},
			wantAllowedPerIP: map[string]int{
				"1.1.1.1": 2,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := WithIPRateLimit(tt.requestsPerMinute)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			allowed := make(map[string]int)
			for _, req := range tt.requests {
				r := httptest.NewRequest(http.MethodGet, "/test", nil)
				r.RemoteAddr = req.ip
				if req.forwarded != "" {
					r.Header.Set("X-Forwarded-For", req.forwarded)
				}
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, r)
				if w.Code == http.StatusOK {
					ip := getIP(r)
					allowed[ip]++
				}
			}

			for ip, want := range tt.wantAllowedPerIP {
				if got := allowed[ip]; got != want {
					t.Errorf("IP %s: got %d requests allowed, want %d", ip, got, want)
				}
			}
		})
	}
}

func TestConcurrentRateLimit(t *testing.T) {
	handler := WithRateLimit(10)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	var wg sync.WaitGroup
	results := make(chan int, 20)

	// Make concurrent requests
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			results <- w.Code
		}()
	}

	wg.Wait()
	close(results)

	// Count results
	allowed := 0
	for code := range results {
		if code == http.StatusOK {
			allowed++
		}
	}

	if allowed > 10 {
		t.Errorf("concurrent requests: got %d allowed, want <= 10", allowed)
	}
}

func TestIPRateLimiterCleanup(t *testing.T) {
	limiter := NewIPRateLimiter(10)

	// Add some IPs
	ips := []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"}
	for _, ip := range ips {
		l := limiter.GetLimiter(ip)
		l.Allow() // Create an event
	}

	// Exhaust the limiters
	for _, ip := range ips {
		l := limiter.GetLimiter(ip)
		// Exhaust the limit
		for i := 0; i < 20; i++ {
			l.Allow()
		}
	}

	// Run cleanup
	limiter.CleanupExpired()

	// Check that limiters are still present but exhausted
	for _, ip := range ips {
		l := limiter.GetLimiter(ip)
		if l.Allow() {
			t.Errorf("limiter for IP %s should be exhausted", ip)
		}
	}
}
