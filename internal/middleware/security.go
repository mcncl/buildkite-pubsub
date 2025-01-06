package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type SecurityConfig struct {
	AllowedOrigins []string
	AllowedMethods []string
	AllowedHeaders []string
	MaxAge         int // in seconds
}

func DefaultSecurityConfig() SecurityConfig {
	return SecurityConfig{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"POST", "OPTIONS"},
		AllowedHeaders: []string{
			"Accept",
			"Content-Type",
			"Content-Length",
			"Accept-Encoding",
			"Authorization",
			"X-CSRF-Token",
			"X-Buildkite-Token",
			"X-Request-ID",
		},
		MaxAge: 3600,
	}
}

func WithSecurity(config SecurityConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Security Headers
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

			// Content Security Policy
			w.Header().Set("Content-Security-Policy", strings.Join([]string{
				"default-src 'none'",
				"frame-ancestors 'none'",
				"base-uri 'none'",
				"form-action 'none'",
				"require-trusted-types-for 'script'",
			}, "; "))

			// HSTS
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")

			// Handle CORS
			origin := r.Header.Get("Origin")
			if origin != "" {
				// Check if origin is allowed
				allowed := false
				for _, allowedOrigin := range config.AllowedOrigins {
					if allowedOrigin == "*" || allowedOrigin == origin {
						allowed = true
						break
					}
				}

				if allowed {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Access-Control-Allow-Methods", strings.Join(config.AllowedMethods, ", "))
					w.Header().Set("Access-Control-Allow-Headers", strings.Join(config.AllowedHeaders, ", "))
					w.Header().Set("Access-Control-Max-Age", strconv.Itoa(config.MaxAge))
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}
			}

			// Handle preflight requests
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Rate limiting middleware
func WithRateLimit(requestsPerMinute int) func(http.Handler) http.Handler {
	limiter := rate.NewLimiter(rate.Every(time.Minute/time.Duration(requestsPerMinute)), requestsPerMinute)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !limiter.Allow() {
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Add basic IP-based rate limiting
func WithPerIPRateLimit(requestsPerMinute int) func(http.Handler) http.Handler {
	limiters := sync.Map{}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get IP from X-Forwarded-For or RemoteAddr
			ip := r.Header.Get("X-Forwarded-For")
			if ip == "" {
				ip = r.RemoteAddr
			}

			// Get or create limiter for this IP
			limiterI, _ := limiters.LoadOrStore(ip, rate.NewLimiter(
				rate.Every(time.Minute/time.Duration(requestsPerMinute)),
				requestsPerMinute,
			))
			limiter := limiterI.(*rate.Limiter)

			if !limiter.Allow() {
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
