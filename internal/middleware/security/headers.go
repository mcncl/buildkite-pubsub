// internal/middleware/security/headers.go
package security

import (
	"net/http"
	"strconv"
	"strings"
)

// SecurityConfig defines the configuration for security headers and CORS
type SecurityConfig struct {
	AllowedOrigins []string
	AllowedMethods []string
	AllowedHeaders []string
	MaxAge         int // in seconds
}

// DefaultConfig returns a default security configuration
func DefaultConfig() SecurityConfig {
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

// WithSecurityHeaders adds security headers to responses
func WithSecurityHeaders(config SecurityConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Security Headers
			setSecurityHeaders(w)

			// Handle CORS
			if handleCORS(w, r, config) {
				if r.Method == http.MethodOptions {
					w.WriteHeader(http.StatusOK)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

func setSecurityHeaders(w http.ResponseWriter) {
	// Basic security headers
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
}

func handleCORS(w http.ResponseWriter, r *http.Request, config SecurityConfig) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}

	// Check if origin is allowed
	allowed := false
	for _, allowedOrigin := range config.AllowedOrigins {
		if allowedOrigin == "*" || allowedOrigin == origin {
			allowed = true
			break
		}
	}

	if !allowed {
		return false
	}

	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Methods", strings.Join(config.AllowedMethods, ", "))
	w.Header().Set("Access-Control-Allow-Headers", strings.Join(config.AllowedHeaders, ", "))
	w.Header().Set("Access-Control-Max-Age", strconv.Itoa(config.MaxAge))
	w.Header().Set("Access-Control-Allow-Credentials", "true")

	return true
}
