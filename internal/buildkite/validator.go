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
    // Log configured token (be careful with this in production!)
    log.Printf("Debug - Configured token: %s", v.token)

    // Extract token from header
    providedToken := r.Header.Get("X-Buildkite-Token")
    log.Printf("Debug - Received token: %s", providedToken)

    // Trim any whitespace
    providedToken = strings.TrimSpace(providedToken)

    // If no token is provided, validation fails
    if providedToken == "" {
        log.Printf("Debug - No token provided")
        return false
    }

    // Log comparison result
    result := subtle.ConstantTimeCompare([]byte(providedToken), []byte(v.token)) == 1
    log.Printf("Debug - Token comparison result: %v", result)

    return result
}
