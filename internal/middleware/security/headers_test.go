package security

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWithSecurityHeaders(t *testing.T) {
	tests := []struct {
		name            string
		config          SecurityConfig
		method          string
		headers         map[string]string
		wantStatus      int
		wantHeaders     map[string]string
		dontWantHeaders []string
	}{
		{
			name:   "adds security headers",
			config: DefaultConfig(),
			method: http.MethodGet,
			wantHeaders: map[string]string{
				"X-Content-Type-Options":    "nosniff",
				"X-Frame-Options":           "DENY",
				"X-XSS-Protection":          "1; mode=block",
				"Content-Security-Policy":   "default-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'; require-trusted-types-for 'script'",
				"Strict-Transport-Security": "max-age=31536000; includeSubDomains; preload",
				"Referrer-Policy":           "strict-origin-when-cross-origin",
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
			wantStatus: http.StatusOK,
			wantHeaders: map[string]string{
				"Access-Control-Allow-Origin":      "https://example.com",
				"Access-Control-Allow-Methods":     "POST, OPTIONS",
				"Access-Control-Allow-Credentials": "true",
				"Access-Control-Max-Age":           "3600",
			},
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
			dontWantHeaders: []string{
				"Access-Control-Allow-Origin",
				"Access-Control-Allow-Methods",
			},
		},
		{
			name:   "handles requests without origin",
			config: DefaultConfig(),
			method: http.MethodGet,
			dontWantHeaders: []string{
				"Access-Control-Allow-Origin",
				"Access-Control-Allow-Methods",
			},
		},
		{
			name: "allows wildcard origin",
			config: SecurityConfig{
				AllowedOrigins: []string{"*"},
				AllowedMethods: []string{"POST"},
			},
			method: http.MethodPost,
			headers: map[string]string{
				"Origin": "https://any-domain.com",
			},
			wantHeaders: map[string]string{
				"Access-Control-Allow-Origin": "https://any-domain.com",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := WithSecurityHeaders(tt.config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(tt.method, "/test", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			// Check status code if specified
			if tt.wantStatus != 0 && w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.wantStatus)
			}

			// Check required headers
			for header, want := range tt.wantHeaders {
				if got := w.Header().Get(header); got != want {
					t.Errorf("header %s = %q, want %q", header, got, want)
				}
			}

			// Check unwanted headers
			for _, header := range tt.dontWantHeaders {
				if got := w.Header().Get(header); got != "" {
					t.Errorf("header %s = %q, want empty", header, got)
				}
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	// Test default values
	tests := []struct {
		name     string
		check    func(SecurityConfig) bool
		wantPass bool
	}{
		{
			name: "has allowed methods",
			check: func(c SecurityConfig) bool {
				return len(c.AllowedMethods) > 0
			},
			wantPass: true,
		},
		{
			name: "has allowed headers",
			check: func(c SecurityConfig) bool {
				return len(c.AllowedHeaders) > 0
			},
			wantPass: true,
		},
		{
			name: "includes required headers",
			check: func(c SecurityConfig) bool {
				required := []string{"Content-Type", "Authorization", "X-Buildkite-Token"}
				for _, req := range required {
					found := false
					for _, h := range c.AllowedHeaders {
						if h == req {
							found = true
							break
						}
					}
					if !found {
						return false
					}
				}
				return true
			},
			wantPass: true,
		},
		{
			name: "has positive max age",
			check: func(c SecurityConfig) bool {
				return c.MaxAge > 0
			},
			wantPass: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.check(config); got != tt.wantPass {
				t.Errorf("DefaultConfig() check failed: %s", tt.name)
			}
		})
	}
}
