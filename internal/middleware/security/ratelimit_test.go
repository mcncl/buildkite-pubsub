package security

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/mcncl/buildkite-pubsub/internal/errors"
)

func TestRateLimiterInterface(t *testing.T) {
	// Test that each implementation satisfies the RateLimiter interface
	var _ RateLimiter = &GlobalRateLimiter{}
	var _ RateLimiter = &IPRateLimiter{}
	var _ RateLimiter = &TokenRateLimiter{}
}

func TestGlobalRateLimiter(t *testing.T) {
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
		{
			name:              "zero limit allows none",
			requestsPerMinute: 0,
			numRequests:       5,
			wantAllowed:       0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limiter := NewGlobalRateLimiter(tt.requestsPerMinute)

			allowed := 0
			for i := 0; i < tt.numRequests; i++ {
				if limiter.Allow(context.Background(), "") {
					allowed++
				}
			}

			if allowed != tt.wantAllowed {
				t.Errorf("got %d requests allowed, want %d", allowed, tt.wantAllowed)
			}
		})
	}
}

func TestIPRateLimiter(t *testing.T) {
	tests := []struct {
		name              string
		requestsPerMinute int
		requests          []struct {
			ip string
		}
		wantAllowedPerIP map[string]int
	}{
		{
			name:              "limits per IP",
			requestsPerMinute: 2,
			requests: []struct {
				ip string
			}{
				{ip: "1.1.1.1"},
				{ip: "1.1.1.1"},
				{ip: "1.1.1.1"},
				{ip: "2.2.2.2"},
				{ip: "2.2.2.2"},
				{ip: "2.2.2.2"},
			},
			wantAllowedPerIP: map[string]int{
				"1.1.1.1": 2,
				"2.2.2.2": 2,
			},
		},
		{
			name:              "handles different IPs independently",
			requestsPerMinute: 1,
			requests: []struct {
				ip string
			}{
				{ip: "1.1.1.1"},
				{ip: "2.2.2.2"},
				{ip: "3.3.3.3"},
				{ip: "1.1.1.1"}, // This should be blocked
				{ip: "2.2.2.2"}, // This should be blocked
				{ip: "3.3.3.3"}, // This should be blocked
			},
			wantAllowedPerIP: map[string]int{
				"1.1.1.1": 1,
				"2.2.2.2": 1,
				"3.3.3.3": 1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limiter := NewIPRateLimiter(tt.requestsPerMinute)

			allowed := make(map[string]int)
			for _, req := range tt.requests {
				if limiter.Allow(context.Background(), req.ip) {
					allowed[req.ip]++
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

func TestTokenRateLimiter(t *testing.T) {
	tests := []struct {
		name              string
		requestsPerMinute int
		requests          []struct {
			token string
		}
		wantAllowedPerToken map[string]int
	}{
		{
			name:              "limits per token",
			requestsPerMinute: 2,
			requests: []struct {
				token string
			}{
				{token: "token1"},
				{token: "token1"},
				{token: "token1"},
				{token: "token2"},
				{token: "token2"},
				{token: "token2"},
			},
			wantAllowedPerToken: map[string]int{
				"token1": 2,
				"token2": 2,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limiter := NewTokenRateLimiter(tt.requestsPerMinute)

			allowed := make(map[string]int)
			for _, req := range tt.requests {
				if limiter.Allow(context.Background(), req.token) {
					allowed[req.token]++
				}
			}

			for token, want := range tt.wantAllowedPerToken {
				if got := allowed[token]; got != want {
					t.Errorf("Token %s: got %d requests allowed, want %d", token, got, want)
				}
			}
		})
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	tests := []struct {
		name        string
		limiter     RateLimiter
		numRequests int
		wantAllowed int
	}{
		{
			name:        "with global rate limiter",
			limiter:     NewGlobalRateLimiter(3),
			numRequests: 5,
			wantAllowed: 3,
		},
		{
			name:        "with IP rate limiter",
			limiter:     NewIPRateLimiter(3),
			numRequests: 5,
			wantAllowed: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := WithRateLimiter(tt.limiter)
			handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			allowed := 0
			for i := 0; i < tt.numRequests; i++ {
				req := httptest.NewRequest(http.MethodGet, "/test", nil)
				// Set a consistent IP for IP-based limiters
				req.RemoteAddr = "192.168.1.1:1234"
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, req)
				if w.Code == http.StatusOK {
					allowed++
				} else if w.Code != http.StatusTooManyRequests {
					t.Errorf("unexpected status code: %d", w.Code)
				}
			}

			if allowed != tt.wantAllowed {
				t.Errorf("middleware allowed %d requests, want %d", allowed, tt.wantAllowed)
			}
		})
	}
}

func TestConcurrentRateLimiting(t *testing.T) {
	limiter := NewGlobalRateLimiter(10)
	middleware := WithRateLimiter(limiter)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

func TestRateLimiterCleanup(t *testing.T) {
	t.Run("IP rate limiter cleanup", func(t *testing.T) {
		limiter := NewIPRateLimiter(10)

		// Add some IPs
		ips := []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"}
		for _, ip := range ips {
			limiter.Allow(context.Background(), ip) // Create an entry
		}

		// Run cleanup
		limiter.CleanupExpired()

		// Add a small check to ensure cleanup doesn't break functionality
		if !limiter.Allow(context.Background(), "4.4.4.4") {
			t.Error("new IP should be allowed after cleanup")
		}
	})

	t.Run("token rate limiter cleanup", func(t *testing.T) {
		limiter := NewTokenRateLimiter(10)

		// Add some tokens
		tokens := []string{"token1", "token2", "token3"}
		for _, token := range tokens {
			limiter.Allow(context.Background(), token) // Create an entry
		}

		// Run cleanup
		limiter.CleanupExpired()

		// Add a small check to ensure cleanup doesn't break functionality
		if !limiter.Allow(context.Background(), "token4") {
			t.Error("new token should be allowed after cleanup")
		}
	})
}

func TestRateLimiterErrors(t *testing.T) {
	limiter := NewGlobalRateLimiter(1)

	// First request should succeed
	if !limiter.Allow(context.Background(), "") {
		t.Error("first request should be allowed")
	}

	// Second request should fail with a rate limit error
	err := limiter.AllowWithError(context.Background(), "")

	if err == nil {
		t.Error("expected rate limit error, got nil")
	}

	if !errors.IsRateLimitError(err) {
		t.Errorf("expected rate limit error, got %T: %v", err, err)
	}

	// Make sure error contains request details
	details := errors.GetDetails(err)
	if details == nil {
		t.Error("error should have details")
	} else {
		if details["rate_limit"] == nil {
			t.Error("error details should contain rate_limit info")
		}
	}
}

func TestRateLimiterWithContextCancellation(t *testing.T) {
	limiter := NewGlobalRateLimiter(5)

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Rate limiting with a cancelled context should return an error
	err := limiter.AllowWithError(ctx, "")
	if err == nil || !errors.IsConnectionError(err) {
		t.Errorf("expected connection error for cancelled context, got %v", err)
	}
}
