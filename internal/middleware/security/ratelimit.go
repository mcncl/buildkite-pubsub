package security

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter provides global rate limiting
type RateLimiter struct {
	limiter *rate.Limiter
}

// NewRateLimiter creates a new rate limiter with specified requests per minute
func NewRateLimiter(requestsPerMinute int) *RateLimiter {
	return &RateLimiter{
		limiter: rate.NewLimiter(
			rate.Every(time.Minute/time.Duration(requestsPerMinute)),
			requestsPerMinute,
		),
	}
}

// WithRateLimit applies global rate limiting to requests
func WithRateLimit(requestsPerMinute int) func(http.Handler) http.Handler {
	limiter := NewRateLimiter(requestsPerMinute)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !limiter.limiter.Allow() {
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// IPRateLimiter provides per-IP rate limiting
type IPRateLimiter struct {
	ips      sync.Map // map[string]*rate.Limiter
	rateFunc func() *rate.Limiter
}

// NewIPRateLimiter creates a new IP-based rate limiter
func NewIPRateLimiter(requestsPerMinute int) *IPRateLimiter {
	return &IPRateLimiter{
		rateFunc: func() *rate.Limiter {
			return rate.NewLimiter(
				rate.Every(time.Minute/time.Duration(requestsPerMinute)),
				requestsPerMinute,
			)
		},
	}
}

// GetLimiter returns the rate limiter for a specific IP
func (i *IPRateLimiter) GetLimiter(ip string) *rate.Limiter {
	limiter, _ := i.ips.LoadOrStore(ip, i.rateFunc())
	return limiter.(*rate.Limiter)
}

// CleanupExpired removes expired limiters (optional, for long-running services)
func (i *IPRateLimiter) CleanupExpired() {
	i.ips.Range(func(key, value interface{}) bool {
		limiter := value.(*rate.Limiter)
		// Check if the limiter is being used by trying to allow a request
		// If we can allow a request after a long period, the limiter hasn't been used
		if limiter.Allow() {
			i.ips.Delete(key)
		}
		return true
	})
}

// WithIPRateLimit applies per-IP rate limiting to requests
func WithIPRateLimit(requestsPerMinute int) func(http.Handler) http.Handler {
	limiter := NewIPRateLimiter(requestsPerMinute)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := getIP(r)
			if !limiter.GetLimiter(ip).Allow() {
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
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
