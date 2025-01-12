package buildkite

import (
	"crypto/subtle"
	"log"
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
	providedToken := r.Header.Get("X-Buildkite-Token")
	providedToken = strings.TrimSpace(providedToken)
	if providedToken == "" {
		log.Printf("Debug - No token provided")
		return false
	}

	result := subtle.ConstantTimeCompare([]byte(providedToken), []byte(v.token)) == 1
	log.Printf("Debug - Token is valid: %v", result)

	return result
}
