package buildkite

import (
	"net/http"
	"net/http/httptest"
	"testing"
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
