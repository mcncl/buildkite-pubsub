package buildkite

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestValidateToken(t *testing.T) {
	tests := []struct {
		name            string
		configuredToken string
		providedToken   string
		want            bool
	}{
		{
			name:            "valid token",
			configuredToken: "test-token",
			providedToken:   "test-token",
			want:            true,
		},
		{
			name:            "invalid token",
			configuredToken: "test-token",
			providedToken:   "wrong-token",
			want:            false,
		},
		{
			name:            "empty token",
			configuredToken: "test-token",
			providedToken:   "",
			want:            false,
		},
		{
			name:            "whitespace token",
			configuredToken: "test-token",
			providedToken:   "  test-token  ",
			want:            true,
		},
		{
			name:            "case-sensitive token",
			configuredToken: "Test-Token",
			providedToken:   "test-token",
			want:            false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create validator with configured token
			validator := NewValidator(tt.configuredToken)

			// Create a mock HTTP request
			req := httptest.NewRequest(http.MethodPost, "/webhook", nil)
			req.Header.Set("X-Buildkite-Token", tt.providedToken)

			// Validate the token
			got := validator.ValidateToken(req)

			if got != tt.want {
				t.Errorf("ValidateToken() = %v, want %v", got, tt.want)

				// Additional logging for debugging
				t.Logf("Configured Token: %q", tt.configuredToken)
				t.Logf("Provided Token:  %q", tt.providedToken)
			}
		})
	}
}

// generateHMACSignature creates a valid HMAC signature for testing
func generateHMACSignature(secret, timestamp, body string) string {
	message := fmt.Sprintf("%s.%s", timestamp, body)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

func TestValidateHMACSignature(t *testing.T) {
	secret := "test-hmac-secret"
	body := `{"event":"build.started","build":{"id":"123"}}`

	tests := []struct {
		name      string
		secret    string
		timestamp string
		body      string
		signature string
		want      bool
	}{
		{
			name:      "valid signature",
			secret:    secret,
			timestamp: strconv.FormatInt(time.Now().Unix(), 10),
			body:      body,
			signature: "", // Will be generated
			want:      true,
		},
		{
			name:      "invalid signature",
			secret:    secret,
			timestamp: strconv.FormatInt(time.Now().Unix(), 10),
			body:      body,
			signature: "invalid-signature",
			want:      false,
		},
		{
			name:      "wrong secret",
			secret:    "wrong-secret",
			timestamp: strconv.FormatInt(time.Now().Unix(), 10),
			body:      body,
			signature: "", // Will be generated with wrong secret
			want:      false,
		},
		{
			name:      "timestamp too old",
			secret:    secret,
			timestamp: strconv.FormatInt(time.Now().Unix()-400, 10), // 400 seconds ago (> 5 min)
			body:      body,
			signature: "", // Will be generated
			want:      false,
		},
		{
			name:      "timestamp in future",
			secret:    secret,
			timestamp: strconv.FormatInt(time.Now().Unix()+400, 10), // 400 seconds in future
			body:      body,
			signature: "", // Will be generated
			want:      false,
		},
		{
			name:      "missing timestamp",
			secret:    secret,
			timestamp: "",
			body:      body,
			signature: "some-signature",
			want:      false,
		},
		{
			name:      "missing signature",
			secret:    secret,
			timestamp: strconv.FormatInt(time.Now().Unix(), 10),
			body:      body,
			signature: "",
			want:      false,
		},
		{
			name:      "different body content",
			secret:    secret,
			timestamp: strconv.FormatInt(time.Now().Unix(), 10),
			body:      `{"event":"build.finished"}`, // Different body
			signature: "",                           // Will be generated with original body
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Generate signature if not provided or if test needs it
			var signature string
			if tt.signature == "" && tt.timestamp != "" && tt.name != "missing signature" {
				// For "wrong secret" test, use the wrong secret to generate signature
				secretToUse := tt.secret
				if tt.name == "wrong secret" {
					secretToUse = "wrong-secret"
				}
				// For "different body content" test, use original body to generate signature
				bodyToSign := tt.body
				if tt.name == "different body content" {
					bodyToSign = body
				}
				signature = generateHMACSignature(secretToUse, tt.timestamp, bodyToSign)
			} else {
				signature = tt.signature
			}

			// Create validator with HMAC secret
			validator := NewValidatorWithHMAC("", secret)

			// Build the signature header
			var headerValue string
			if tt.timestamp != "" || signature != "" {
				headerValue = fmt.Sprintf("timestamp=%s,signature=%s", tt.timestamp, signature)
			}

			// Create a mock HTTP request with body
			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBufferString(tt.body))
			if headerValue != "" {
				req.Header.Set("X-Buildkite-Signature", headerValue)
			}

			// Validate the signature
			got := validator.ValidateToken(req)

			if got != tt.want {
				t.Errorf("ValidateToken() with HMAC = %v, want %v", got, tt.want)
				t.Logf("Secret: %q", tt.secret)
				t.Logf("Timestamp: %q", tt.timestamp)
				t.Logf("Signature: %q", signature)
				t.Logf("Header: %q", headerValue)
			}
		})
	}
}

func TestValidatorPreference(t *testing.T) {
	secret := "test-hmac-secret"
	token := "test-token"
	body := `{"event":"build.started"}`
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	signature := generateHMACSignature(secret, timestamp, body)
	headerValue := fmt.Sprintf("timestamp=%s,signature=%s", timestamp, signature)

	tests := []struct {
		name           string
		setupValidator func() *Validator
		setupRequest   func() *http.Request
		want           bool
	}{
		{
			name: "HMAC signature takes precedence over token when both present",
			setupValidator: func() *Validator {
				return NewValidatorWithHMAC(token, secret)
			},
			setupRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBufferString(body))
				req.Header.Set("X-Buildkite-Token", token)
				req.Header.Set("X-Buildkite-Signature", headerValue)
				return req
			},
			want: true,
		},
		{
			name: "Falls back to token when no signature present",
			setupValidator: func() *Validator {
				return NewValidatorWithHMAC(token, secret)
			},
			setupRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBufferString(body))
				req.Header.Set("X-Buildkite-Token", token)
				return req
			},
			want: true,
		},
		{
			name: "Token validation fails when signature not present and token wrong",
			setupValidator: func() *Validator {
				return NewValidatorWithHMAC(token, secret)
			},
			setupRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBufferString(body))
				req.Header.Set("X-Buildkite-Token", "wrong-token")
				return req
			},
			want: false,
		},
		{
			name: "No HMAC secret configured, only validates token",
			setupValidator: func() *Validator {
				return NewValidator(token)
			},
			setupRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBufferString(body))
				req.Header.Set("X-Buildkite-Token", token)
				req.Header.Set("X-Buildkite-Signature", headerValue)
				return req
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := tt.setupValidator()
			req := tt.setupRequest()

			got := validator.ValidateToken(req)

			if got != tt.want {
				t.Errorf("ValidateToken() = %v, want %v", got, tt.want)
			}
		})
	}
}
