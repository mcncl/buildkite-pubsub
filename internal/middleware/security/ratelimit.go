package security

import (
	"net/http"
	"time"

	"github.com/mcncl/buildkite-pubsub/internal/metrics"
	"golang.org/x/time/rate"
)

// RateLimiter provides global rate limiting
type RateLimiter struct {
	limiter *rate.Limiter
}

// NewRateLimiter creates a new rate limiter with the given requests per minute
func NewRateLimiter(requestsPerMinute int) *RateLimiter {
	if requestsPerMinute <= 0 {
		requestsPerMinute = 60 // default
	}
	r := rate.Every(time.Minute / time.Duration(requestsPerMinute))
	return &RateLimiter{
		limiter: rate.NewLimiter(r, requestsPerMinute),
	}
}

// Allow checks if a request is allowed
func (rl *RateLimiter) Allow() bool {
	return rl.limiter.Allow()
}

// WithRateLimit returns middleware that applies rate limiting
func WithRateLimit(requestsPerMinute int) func(http.Handler) http.Handler {
	limiter := NewRateLimiter(requestsPerMinute)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !limiter.Allow() {
				metrics.RateLimitExceeded.WithLabelValues("http").Inc()
				w.Header().Set("Retry-After", "60")
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
