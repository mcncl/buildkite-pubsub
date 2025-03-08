package security

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/mcncl/buildkite-pubsub/internal/errors"
	"github.com/mcncl/buildkite-pubsub/internal/metrics"
	"golang.org/x/time/rate"
)

// RateLimiter defines the interface for different rate limiting strategies
type RateLimiter interface {
	// Allow checks if the request is allowed based on the key
	Allow(ctx context.Context, key string) bool
	
	// AllowWithError returns nil if allowed, or an appropriate error if not allowed
	AllowWithError(ctx context.Context, key string) error
	
	// CleanupExpired removes expired rate limiters
	CleanupExpired()
	
	// GetRequestsPerMinute returns the configured requests per minute
	GetRequestsPerMinute() int
}

// BaseRateLimiter provides the base implementation for rate limiters
type BaseRateLimiter struct {
	requestsPerMinute int
	items             sync.Map // map[string]*rate.Limiter
	cleanupInterval   time.Duration
	lastCleanup       time.Time
	mu                sync.Mutex // protects lastCleanup
}

// NewBaseRateLimiter creates a new base rate limiter
func NewBaseRateLimiter(requestsPerMinute int) *BaseRateLimiter {
	return &BaseRateLimiter{
		requestsPerMinute: requestsPerMinute,
		cleanupInterval:   10 * time.Minute,
		lastCleanup:       time.Now(),
	}
}

// GetRequestsPerMinute returns the configured requests per minute
func (b *BaseRateLimiter) GetRequestsPerMinute() int {
	return b.requestsPerMinute
}

// Get or create a rate limiter for a key
func (b *BaseRateLimiter) getLimiter(key string) *rate.Limiter {
	// If key is empty or rate limit is 0, always limit
	if key == "" || b.requestsPerMinute <= 0 {
		// Return a limiter that always rejects
		return rate.NewLimiter(rate.Limit(0), 0)
	}

	// Check for cleanup need
	b.checkCleanup()

	// Get or create limiter
	value, _ := b.items.LoadOrStore(key, b.newLimiter())
	return value.(*rate.Limiter)
}

// Create a new limiter with the configured rate
func (b *BaseRateLimiter) newLimiter() *rate.Limiter {
	r := rate.Every(time.Minute / time.Duration(b.requestsPerMinute))
	return rate.NewLimiter(r, b.requestsPerMinute)
}

// Check if cleanup is needed and run if necessary
func (b *BaseRateLimiter) checkCleanup() {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	if time.Since(b.lastCleanup) >= b.cleanupInterval {
		go b.CleanupExpired()
		b.lastCleanup = time.Now()
	}
}

// CleanupExpired removes expired rate limiters
func (b *BaseRateLimiter) CleanupExpired() {
	now := time.Now()
	
	var keysToDelete []string
	
	// First pass: identify keys for deletion
	b.items.Range(func(key, value interface{}) bool {
		limiter := value.(*rate.Limiter)
		// Check if limiter has been inactive for the threshold period
		// This is an approximation since rate.Limiter doesn't expose last use time
		// We can check if the token bucket is full as a heuristic
		if limiter.TokensAt(now) >= float64(limiter.Burst()) {
			keysToDelete = append(keysToDelete, key.(string))
		}
		return true
	})
	
	// Second pass: delete identified keys
	for _, key := range keysToDelete {
		b.items.Delete(key)
	}
}

// Allow checks if the request is allowed based on the key
func (b *BaseRateLimiter) Allow(ctx context.Context, key string) bool {
	// Handle context cancellation
	if ctx.Err() != nil {
		return false
	}

	return b.getLimiter(key).Allow()
}

// AllowWithError returns nil if allowed, or an appropriate error if not allowed
func (b *BaseRateLimiter) AllowWithError(ctx context.Context, key string) error {
	// Handle context cancellation
	if ctx.Err() != nil {
		return errors.WithDetails(
			errors.NewConnectionError("context cancelled"),
			map[string]interface{}{
				"context_error": ctx.Err().Error(),
			},
		)
	}

	if !b.getLimiter(key).Allow() {
		return errors.WithDetails(
			errors.NewRateLimitError("rate limit exceeded"),
			map[string]interface{}{
				"key": key,
				"rate_limit": b.requestsPerMinute,
				"retry_after": 60, // Suggest retry after 60 seconds
			},
		)
	}
	
	return nil
}

//
// Specific Rate Limiter Implementations
//

// GlobalRateLimiter implements a global rate limiter
type GlobalRateLimiter struct {
	*BaseRateLimiter
}

// NewGlobalRateLimiter creates a new global rate limiter
func NewGlobalRateLimiter(requestsPerMinute int) *GlobalRateLimiter {
	return &GlobalRateLimiter{
		BaseRateLimiter: NewBaseRateLimiter(requestsPerMinute),
	}
}

// Allow for GlobalRateLimiter uses a fixed global key
func (g *GlobalRateLimiter) Allow(ctx context.Context, _ string) bool {
	return g.BaseRateLimiter.Allow(ctx, "global")
}

// AllowWithError for GlobalRateLimiter uses a fixed global key
func (g *GlobalRateLimiter) AllowWithError(ctx context.Context, _ string) error {
	return g.BaseRateLimiter.AllowWithError(ctx, "global")
}

// IPRateLimiter implements a per-IP rate limiter
type IPRateLimiter struct {
	*BaseRateLimiter
}

// NewIPRateLimiter creates a new IP-based rate limiter
func NewIPRateLimiter(requestsPerMinute int) *IPRateLimiter {
	return &IPRateLimiter{
		BaseRateLimiter: NewBaseRateLimiter(requestsPerMinute),
	}
}

// TokenRateLimiter implements a per-token rate limiter
type TokenRateLimiter struct {
	*BaseRateLimiter
}

// NewTokenRateLimiter creates a new token-based rate limiter
func NewTokenRateLimiter(requestsPerMinute int) *TokenRateLimiter {
	return &TokenRateLimiter{
		BaseRateLimiter: NewBaseRateLimiter(requestsPerMinute),
	}
}

//
// Middleware implementations
//

// WithRateLimiter returns middleware that applies the given rate limiter
func WithRateLimiter(limiter RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// For global limiters, we just use empty key
			// For IP limiters, we extract IP later
			key := ""
			
			// Determine rate limit key based on the limiter type
			switch limiter.(type) {
			case *IPRateLimiter:
				key = getIP(r)
			case *TokenRateLimiter:
				key = r.Header.Get("Authorization")
				if key == "" {
					key = r.Header.Get("X-API-Key")
				}
			}
			
			if err := limiter.AllowWithError(r.Context(), key); err != nil {
				metrics.RateLimitExceeded.WithLabelValues("http").Inc()
				
				// Set retry-after header if it's a rate limit error
				if errors.IsRateLimitError(err) {
					if retryAfter, ok := errors.GetRetryOption(err); ok {
						w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
					} else {
						w.Header().Set("Retry-After", "60") // Default to 60 seconds
					}
				}
				
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}
			
			next.ServeHTTP(w, r)
		})
	}
}

// Compatibility functions for backward compatibility

// WithRateLimit applies global rate limiting to requests
func WithRateLimit(requestsPerMinute int) func(http.Handler) http.Handler {
	return WithRateLimiter(NewGlobalRateLimiter(requestsPerMinute))
}

// WithIPRateLimit applies per-IP rate limiting to requests
func WithIPRateLimit(requestsPerMinute int) func(http.Handler) http.Handler {
	return WithRateLimiter(NewIPRateLimiter(requestsPerMinute))
}

// getIP extracts the client IP from the request
func getIP(r *http.Request) string {
	// Check X-Forwarded-For header
	ip := r.Header.Get("X-Forwarded-For")
	if ip != "" {
		// Take the first IP if multiple are present
		if i := strings.Index(ip, ","); i > -1 {
			ip = strings.TrimSpace(ip[:i])
		}
		return ip
	}

	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
