package errors

import (
	"fmt"
	"reflect"
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

func TestErrorWithDetails(t *testing.T) {
	// Create an error with details
	details := map[string]interface{}{
		"field": "username",
		"code":  123,
	}

	err := WithDetails(NewValidationError("invalid input"), details)

	// Test that we can get the details back
	if detailedErr, ok := err.(ErrorWithDetails); !ok {
		t.Error("Error should implement ErrorWithDetails")
	} else {
		gotDetails := detailedErr.Details()
		if !reflect.DeepEqual(gotDetails, details) {
			t.Errorf("Details() = %v, want %v", gotDetails, details)
		}
	}

	// Test with GetDetails helper
	gotDetails := GetDetails(err)
	if !reflect.DeepEqual(gotDetails, details) {
		t.Errorf("GetDetails() = %v, want %v", gotDetails, details)
	}

	// Test with nil error
	if GetDetails(nil) != nil {
		t.Error("GetDetails(nil) should return nil")
	}
}

func TestWithRetryOption(t *testing.T) {
	// Create basic error
	err := NewConnectionError("connection timeout")

	// Add retry option
	retryErr := WithRetryOption(err, 60)

	// Test that we can get the retry option back
	details := GetDetails(retryErr)
	if details == nil {
		t.Error("WithRetryOption should add details to error")
	} else {
		retryValue, ok := details["retry_after"]
		if !ok {
			t.Error("WithRetryOption should add retry_after to details")
		} else if retryValue != 60 {
			t.Errorf("retry_after = %v, want 60", retryValue)
		}
	}

	// Test GetRetryOption helper
	retrySeconds, ok := GetRetryOption(retryErr)
	if !ok {
		t.Error("GetRetryOption should return true for error with retry option")
	} else if retrySeconds != 60 {
		t.Errorf("GetRetryOption() = %v, want 60", retrySeconds)
	}

	// Test with nil error
	if WithRetryOption(nil, 30) != nil {
		t.Error("WithRetryOption(nil) should return nil")
	}

	// Test GetRetryOption with error without retry
	if _, ok := GetRetryOption(NewAuthError("invalid token")); ok {
		t.Error("GetRetryOption should return false for error without retry option")
	}
}

func TestToErrorResponse(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		wantType  string
		wantRetry bool
	}{
		{
			name:      "auth error",
			err:       NewAuthError("invalid token"),
			wantType:  "auth",
			wantRetry: false,
		},
		{
			name:      "validation error",
			err:       NewValidationError("invalid input"),
			wantType:  "validation",
			wantRetry: false,
		},
		{
			name:      "rate limit error",
			err:       NewRateLimitError("too many requests"),
			wantType:  "rate_limit",
			wantRetry: true,
		},
		{
			name:      "connection error",
			err:       NewConnectionError("connection timeout"),
			wantType:  "connection",
			wantRetry: true,
		},
		{
			name:      "publish error",
			err:       NewPublishError("publish failed", nil),
			wantType:  "publish",
			wantRetry: true,
		},
		{
			name:      "error with retry option",
			err:       WithRetryOption(NewValidationError("retry this"), 45),
			wantType:  "validation",
			wantRetry: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := ToErrorResponse(tt.err)

			// Check error type
			if resp.ErrorType != tt.wantType {
				t.Errorf("ErrorType = %v, want %v", resp.ErrorType, tt.wantType)
			}

			// Check message contains error text
			if resp.Message == "" {
				t.Error("Message should not be empty")
			}

			// Check retry info
			hasRetry := resp.RetryAfter > 0
			if hasRetry != tt.wantRetry {
				t.Errorf("hasRetry = %v, want %v", hasRetry, tt.wantRetry)
			}

			// For errors with specific retry values
			if retryErr, ok := tt.err.(*errorType); ok && retryErr.details != nil {
				if retry, ok := retryErr.details["retry_after"]; ok {
					if resp.RetryAfter != retry {
						t.Errorf("RetryAfter = %v, want %v", resp.RetryAfter, retry)
					}
				}
			}
		})
	}

	// Test with nil error
	resp := ToErrorResponse(nil)
	if resp.Status != "error" || resp.Message == "" {
		t.Errorf("ToErrorResponse(nil) should return default error response")
	}
}
