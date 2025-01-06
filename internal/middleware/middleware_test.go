package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWithRequestID(t *testing.T) {
	// Existing test remains the same
}

func TestWithRequestTimeout(t *testing.T) {
	// Existing test remains the same
}

func TestWithSecurity(t *testing.T) {
	tests := []struct {
		name            string
		config          SecurityConfig
		method          string
		headers         map[string]string
		wantStatus      int
		wantHeaders     map[string]string
		wantCORSHeaders bool
	}{
		{
			name:   "adds security headers",
			config: DefaultSecurityConfig(),
			method: http.MethodGet,
			wantHeaders: map[string]string{
				"X-Content-Type-Options":    "nosniff",
				"X-Frame-Options":           "DENY",
				"X-XSS-Protection":          "1; mode=block",
				"Referrer-Policy":           "strict-origin-when-cross-origin",
				"Content-Security-Policy":   "default-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'; require-trusted-types-for 'script'",
				"Strict-Transport-Security": "max-age=31536000; includeSubDomains; preload",
			},
		},
		{
			name: "handles CORS preflight",
			config: SecurityConfig{
				AllowedOrigins: []string{"https://example.com"},
				AllowedMethods: []string{"POST", "OPTIONS"},
				AllowedHeaders: []string{"X-Test-Header"},
				MaxAge:         3600,
			},
			method: http.MethodOptions,
			headers: map[string]string{
				"Origin": "https://example.com",
			},
			wantStatus:      http.StatusOK,
			wantCORSHeaders: true,
		},
		{
			name: "blocks disallowed origin",
			config: SecurityConfig{
				AllowedOrigins: []string{"https://example.com"},
				AllowedMethods: []string{"POST"},
			},
			method: http.MethodPost,
			headers: map[string]string{
				"Origin": "https://evil.com",
			},
			wantCORSHeaders: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := WithSecurity(tt.config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(tt.method, "/test", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			// Check status code
			if tt.wantStatus != 0 && w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.wantStatus)
			}

			// Check security headers
			for header, expected := range tt.wantHeaders {
				if got := w.Header().Get(header); got != expected {
					t.Errorf("header %s = %s, want %s", header, got, expected)
				}
			}

			// Check CORS headers when applicable
			if tt.wantCORSHeaders {
				corsHeaders := []string{
					"Access-Control-Allow-Origin",
					"Access-Control-Allow-Methods",
					"Access-Control-Allow-Headers",
					"Access-Control-Max-Age",
				}
				for _, header := range corsHeaders {
					if got := w.Header().Get(header); got == "" {
						t.Errorf("missing CORS header %s", header)
					}
				}
			}
		})
	}
}

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

func TestWithPerIPRateLimit(t *testing.T) {
	tests := []struct {
		name              string
		requestsPerMinute int
		ips               []string
		wantAllowed       map[string]int
	}{
		{
			name:              "limits per IP",
			requestsPerMinute: 2,
			ips:               []string{"1.1.1.1", "1.1.1.1", "2.2.2.2", "2.2.2.2"},
			wantAllowed: map[string]int{
				"1.1.1.1": 2,
				"2.2.2.2": 2,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := WithPerIPRateLimit(tt.requestsPerMinute)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			allowed := make(map[string]int)
			for _, ip := range tt.ips {
				req := httptest.NewRequest(http.MethodGet, "/test", nil)
				req.Header.Set("X-Forwarded-For", ip)
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, req)
				if w.Code == http.StatusOK {
					allowed[ip]++
				}
			}

			for ip, want := range tt.wantAllowed {
				if got := allowed[ip]; got != want {
					t.Errorf("IP %s: got %d requests allowed, want %d", ip, got, want)
				}
			}
		})
	}
}

func TestWithLogging(t *testing.T) {
	handler := WithLogging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req = req.WithContext(context.WithValue(req.Context(), "requestID", "test-id"))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", w.Code, http.StatusOK)
	}
}

func TestResponseWriter(t *testing.T) {
	// Existing test remains the same
}
