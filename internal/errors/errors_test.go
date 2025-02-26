package errors

import (
	"fmt"
	"strings"
	"testing"
)

func TestErrorTypes(t *testing.T) {
	tests := []struct {
		name            string
		err             error
		expectedMessage string
		isAuthError     bool
		isValidation    bool
		isRateLimit     bool
		isRetryable     bool
	}{
		{
			name:            "auth error",
			err:             NewAuthError("invalid token"),
			expectedMessage: "authentication error: invalid token",
			isAuthError:     true,
			isValidation:    false,
			isRateLimit:     false,
			isRetryable:     false,
		},
		{
			name:            "validation error",
			err:             NewValidationError("missing required field"),
			expectedMessage: "validation error: missing required field",
			isAuthError:     false,
			isValidation:    true,
			isRateLimit:     false,
			isRetryable:     false,
		},
		{
			name:            "rate limit error",
			err:             NewRateLimitError("too many requests"),
			expectedMessage: "rate limit error: too many requests",
			isAuthError:     false,
			isValidation:    false,
			isRateLimit:     true,
			isRetryable:     true,
		},
		{
			name:            "publish error",
			err:             NewPublishError("failed to publish message", nil),
			expectedMessage: "publish error: failed to publish message",
			isAuthError:     false,
			isValidation:    false,
			isRateLimit:     false,
			isRetryable:     true,
		},
		{
			name:            "connection error",
			err:             NewConnectionError("connection refused"),
			expectedMessage: "connection error: connection refused",
			isAuthError:     false,
			isValidation:    false,
			isRateLimit:     false,
			isRetryable:     true,
		},
		{
			name:            "not found error",
			err:             NewNotFoundError("resource not found"),
			expectedMessage: "not found error: resource not found",
			isAuthError:     false,
			isValidation:    false,
			isRateLimit:     false,
			isRetryable:     false,
		},
		{
			name:            "internal error",
			err:             NewInternalError("unexpected error"),
			expectedMessage: "internal error: unexpected error",
			isAuthError:     false,
			isValidation:    false,
			isRateLimit:     false,
			isRetryable:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.expectedMessage {
				t.Errorf("expected message %q, got %q", tt.expectedMessage, tt.err.Error())
			}

			if IsAuthError(tt.err) != tt.isAuthError {
				t.Errorf("IsAuthError(%v) = %v, want %v", tt.err, IsAuthError(tt.err), tt.isAuthError)
			}

			if IsValidationError(tt.err) != tt.isValidation {
				t.Errorf("IsValidationError(%v) = %v, want %v", tt.err, IsValidationError(tt.err), tt.isValidation)
			}

			if IsRateLimitError(tt.err) != tt.isRateLimit {
				t.Errorf("IsRateLimitError(%v) = %v, want %v", tt.err, IsRateLimitError(tt.err), tt.isRateLimit)
			}

			if IsRetryable(tt.err) != tt.isRetryable {
				t.Errorf("IsRetryable(%v) = %v, want %v", tt.err, IsRetryable(tt.err), tt.isRetryable)
			}
		})
	}
}

func TestErrorWrapping(t *testing.T) {
	originalErr := fmt.Errorf("original error")
	wrappedErr := Wrap(originalErr, "additional context")

	if !strings.Contains(wrappedErr.Error(), "original error") {
		t.Errorf("wrapped error %q does not contain original error message", wrappedErr.Error())
	}

	if !strings.Contains(wrappedErr.Error(), "additional context") {
		t.Errorf("wrapped error %q does not contain context message", wrappedErr.Error())
	}

	// Test unwrapping
	unwrappedErr := Unwrap(wrappedErr)
	if unwrappedErr.Error() != "original error" {
		t.Errorf("unwrapped error = %v, want %v", unwrappedErr, originalErr)
	}

	// Test with nil error
	nilWrapped := Wrap(nil, "context for nil")
	if nilWrapped != nil {
		t.Errorf("Wrap(nil, ...) = %v, want nil", nilWrapped)
	}
}

func TestErrorWithDetails(t *testing.T) {
	baseErr := NewValidationError("invalid input")
	detailedErr := WithDetails(baseErr, map[string]interface{}{
		"field":  "username",
		"reason": "too short",
		"min":    5,
	})

	expectedDetails := `{"field":"username","min":5,"reason":"too short"}`
	if !strings.Contains(detailedErr.Error(), expectedDetails) {
		t.Errorf("error with details %q does not contain expected details %q", detailedErr.Error(), expectedDetails)
	}

	// Test that original error type is preserved
	if !IsValidationError(detailedErr) {
		t.Errorf("error type not preserved after adding details")
	}

	// Test with nil error
	nilDetailed := WithDetails(nil, map[string]interface{}{"key": "value"})
	if nilDetailed != nil {
		t.Errorf("WithDetails(nil, ...) = %v, want nil", nilDetailed)
	}
}

func TestErrorRetryable(t *testing.T) {
	// Test making a non-retryable error retryable
	nonRetryable := NewValidationError("invalid input")
	retryable := MakeRetryable(nonRetryable)

	if !IsRetryable(retryable) {
		t.Errorf("MakeRetryable did not make error retryable")
	}

	// Original error type should still be detectable
	if !IsValidationError(retryable) {
		t.Errorf("original error type not preserved after making retryable")
	}

	// Test making an already retryable error retryable again
	alreadyRetryable := NewRateLimitError("too many requests")
	stillRetryable := MakeRetryable(alreadyRetryable)

	if !IsRetryable(stillRetryable) {
		t.Errorf("retryable status not preserved")
	}

	// Test with nil error
	nilRetryable := MakeRetryable(nil)
	if nilRetryable != nil {
		t.Errorf("MakeRetryable(nil) = %v, want nil", nilRetryable)
	}
}

func TestErrorFormatting(t *testing.T) {
	// Test structured error formatting
	err := NewValidationError("invalid input")
	err = WithDetails(err, map[string]interface{}{
		"field":  "username",
		"reason": "too short",
	})

	formatted := Format(err)
	expected := "validation error: invalid input - details: {\"field\":\"username\",\"reason\":\"too short\"}"
	if formatted != expected {
		t.Errorf("Format(%v) = %v, want %v", err, formatted, expected)
	}

	// Test with nil error
	nilFormatted := Format(nil)
	if nilFormatted != "" {
		t.Errorf("Format(nil) = %q, want empty string", nilFormatted)
	}
}
