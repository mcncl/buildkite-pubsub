package examples

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/mcncl/buildkite-pubsub/internal/errors"
	"github.com/mcncl/buildkite-pubsub/internal/publisher"
)

// This is an example file showing how to use the errors package

// ErrorResponse represents a standardized error response
type ErrorResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Code    int    `json:"code,omitempty"`
}

// ProcessWebhook demonstrates how to use the errors package in a webhook handler
func ProcessWebhook(w http.ResponseWriter, r *http.Request, pub publisher.Publisher) {
	// Extract request ID for tracing
	requestID := r.Header.Get("X-Request-ID")

	// Use validation function that returns our custom error
	if err := validateRequest(r); err != nil {
		handleError(w, err, requestID)
		return
	}

	// Extract payload
	var payload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		err = errors.NewValidationError("invalid JSON payload")
		err = errors.WithDetails(err, map[string]interface{}{
			"request_id": requestID,
		})
		handleError(w, err, requestID)
		return
	}

	// Attempt to publish (could fail with various errors)
	if err := publishEvent(r.Context(), pub, payload, requestID); err != nil {
		handleError(w, err, requestID)
		return
	}

	// Success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Event published successfully",
	})
}

// validateRequest checks if the request is valid
func validateRequest(r *http.Request) error {
	// Check method
	if r.Method != http.MethodPost {
		return errors.NewValidationError("invalid method, only POST is allowed")
	}

	// Check content type
	if contentType := r.Header.Get("Content-Type"); contentType != "application/json" {
		return errors.WithDetails(
			errors.NewValidationError("invalid content type, expected application/json"),
			map[string]interface{}{
				"received_content_type": contentType,
			},
		)
	}

	// Check authorization
	token := r.Header.Get("X-Buildkite-Token")
	if token == "" {
		return errors.NewAuthError("missing webhook token")
	}

	// Check if token is valid (simplified for example)
	if token != "valid-token" {
		return errors.WithDetails(
			errors.NewAuthError("invalid webhook token"),
			map[string]interface{}{
				"token_length": len(token),
			},
		)
	}

	return nil
}

// publishEvent publishes the event to Pub/Sub
func publishEvent(ctx context.Context, pub publisher.Publisher, payload map[string]interface{}, requestID string) error {
	// Extract event type (for demonstration)
	eventType, ok := payload["event"].(string)
	if !ok {
		return errors.NewValidationError("missing 'event' field")
	}

	// Add attributes
	attributes := map[string]string{
		"request_id": requestID,
		"event_type": eventType,
	}

	// Try to publish (might fail for various reasons)
	_, err := pub.Publish(ctx, payload, attributes)
	if err != nil {
		// Wrap the error with context
		pubErr := errors.NewPublishError("failed to publish event", err)

		// Add details for debugging
		return errors.WithDetails(pubErr, map[string]interface{}{
			"request_id": requestID,
			"event_type": eventType,
		})
	}

	return nil
}

// handleError handles different types of errors and returns appropriate HTTP responses
func handleError(w http.ResponseWriter, err error, requestID string) {
	// Always log the full error details
	log.Printf("[%s] Error: %s", requestID, errors.Format(err))

	// Create response with appropriate status code
	var statusCode int
	var response ErrorResponse

	switch {
	case errors.IsAuthError(err):
		statusCode = http.StatusUnauthorized
		response = ErrorResponse{
			Status:  "error",
			Message: "Authentication failed",
			Code:    401,
		}

	case errors.IsValidationError(err):
		statusCode = http.StatusBadRequest
		response = ErrorResponse{
			Status:  "error",
			Message: "Invalid request",
			Code:    400,
		}

	case errors.IsRateLimitError(err):
		statusCode = http.StatusTooManyRequests
		response = ErrorResponse{
			Status:  "error",
			Message: "Rate limit exceeded",
			Code:    429,
		}

	case errors.IsNotFoundError(err):
		statusCode = http.StatusNotFound
		response = ErrorResponse{
			Status:  "error",
			Message: "Resource not found",
			Code:    404,
		}

	default:
		// For all other errors (internal, connection, publish)
		statusCode = http.StatusInternalServerError
		response = ErrorResponse{
			Status:  "error",
			Message: "Internal server error",
			Code:    500,
		}
	}

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)

	// If error is retryable, add Retry-After header
	if errors.IsRetryable(err) {
		w.Header().Set("Retry-After", "60")
	}
}

// RetryWithBackoff demonstrates how to use retryable errors
func RetryWithBackoff(ctx context.Context, pub publisher.Publisher, payload map[string]interface{}) error {
	maxRetries := 3

	for attempt := 0; attempt < maxRetries; attempt++ {
		// Try to publish
		msgID, err := pub.Publish(ctx, payload, nil)
		if err == nil {
			fmt.Printf("Successfully published message with ID: %s\n", msgID)
			return nil
		}

		// Check if error is retryable
		if !errors.IsRetryable(err) {
			return fmt.Errorf("non-retryable error occurred: %w", err)
		}

		// Log retry attempt
		fmt.Printf("Retryable error occurred (attempt %d/%d): %v\n",
			attempt+1, maxRetries, err)

		// In a real implementation, you would add backoff here
		// time.Sleep(backoff.Calculate(attempt))
	}

	return errors.NewPublishError("max retries exceeded", nil)
}
