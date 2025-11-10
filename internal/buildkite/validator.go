package buildkite

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Validator handles webhook token and HMAC signature validation
type Validator struct {
	token      string
	hmacSecret string
}

// NewValidator creates a new validator with the given token and optional HMAC secret
func NewValidator(token string) *Validator {
	return &Validator{token: token}
}

// NewValidatorWithHMAC creates a new validator with HMAC signature support
func NewValidatorWithHMAC(token, hmacSecret string) *Validator {
	return &Validator{
		token:      token,
		hmacSecret: hmacSecret,
	}
}

// ValidateToken checks if the provided token matches the expected token or validates HMAC signature
func (v *Validator) ValidateToken(r *http.Request) bool {
	// First, check if HMAC signature is present
	signature := r.Header.Get("X-Buildkite-Signature")
	if signature != "" && v.hmacSecret != "" {
		return v.validateHMACSignature(r, signature)
	}

	// Fall back to token validation
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

// validateHMACSignature validates the HMAC-SHA256 signature from Buildkite
func (v *Validator) validateHMACSignature(r *http.Request, headerValue string) bool {
	// Parse the header value (format: "timestamp=1619071700,signature=...")
	parts := strings.Split(headerValue, ",")
	var timestamp, signature string

	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		value := strings.TrimSpace(kv[1])

		switch key {
		case "timestamp":
			timestamp = value
		case "signature":
			signature = value
		}
	}

	if timestamp == "" || signature == "" {
		log.Printf("Debug - Invalid signature format: missing timestamp or signature")
		return false
	}

	// Validate timestamp to prevent replay attacks (within 5 minutes)
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		log.Printf("Debug - Invalid timestamp format: %v", err)
		return false
	}

	// Check if timestamp is within acceptable window (5 minutes)
	now := time.Now().Unix()
	timeDiff := now - ts
	if timeDiff < 0 {
		timeDiff = -timeDiff
	}
	if timeDiff > 300 { // 5 minutes
		log.Printf("Debug - Timestamp too old or in future: %d seconds difference", timeDiff)
		return false
	}

	// Read the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Debug - Failed to read request body: %v", err)
		return false
	}
	// Restore the body for later use
	r.Body = io.NopCloser(strings.NewReader(string(body)))

	// Compute expected signature: HMAC-SHA256(secret, "timestamp.body")
	message := fmt.Sprintf("%s.%s", timestamp, string(body))
	mac := hmac.New(sha256.New, []byte(v.hmacSecret))
	mac.Write([]byte(message))
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	// Compare signatures using constant-time comparison
	result := subtle.ConstantTimeCompare([]byte(signature), []byte(expectedSignature)) == 1
	log.Printf("Debug - HMAC signature is valid: %v", result)

	return result
}
