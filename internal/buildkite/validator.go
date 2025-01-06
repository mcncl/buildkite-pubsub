package buildkite

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// Validator handles webhook token validation
type Validator struct {
	token string
}

// NewValidator creates a new validator with the given token
func NewValidator(token string) *Validator {
	return &Validator{token: token}
}

// ValidateToken checks if the provided token matches the expected token
func (v *Validator) ValidateToken(r *http.Request) bool {
	// If no token is configured, skip validation
	if v.token == "" {
		return true
	}

	// Extract token from header
	providedToken := r.Header.Get("X-Buildkite-Token")

	// Trim any whitespace
	providedToken = strings.TrimSpace(providedToken)

	// If no token is provided, validation fails
	if providedToken == "" {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(providedToken), []byte(v.token)) == 1
}
