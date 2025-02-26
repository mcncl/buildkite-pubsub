// Package errors provides a standardized error handling framework for the Buildkite PubSub Webhook application.
// It defines common error types, wrapping functions, and classification methods to ensure consistent
// error handling across the application.
package errors

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Standard error types for the application
var (
	ErrAuthentication = errors.New("authentication error")
	ErrValidation     = errors.New("validation error")
	ErrRateLimit      = errors.New("rate limit error")
	ErrPublish        = errors.New("publish error")
	ErrConnection     = errors.New("connection error")
	ErrNotFound       = errors.New("not found error")
	ErrInternal       = errors.New("internal error")
)

// errorType is a custom error with a specific type
type errorType struct {
	baseErr error
	msg     string
	cause   error
	details map[string]interface{}
	// Flag to indicate if the error is retryable
	retryable bool
}

type ErrorWithDetails interface {
	Error() string
	Details() map[string]interface{}
}

// Error implements the error interface
func (e *errorType) Error() string {
	if e == nil {
		return ""
	}

	base := fmt.Sprintf("%s: %s", e.baseErr.Error(), e.msg)

	if e.details != nil && len(e.details) > 0 {
		detailsJSON, err := json.Marshal(e.details)
		if err == nil {
			base += fmt.Sprintf(" - details: %s", detailsJSON)
		}
	}

	if e.cause != nil {
		base += fmt.Sprintf(" - caused by: %v", e.cause)
	}

	return base
}

// Unwrap returns the underlying cause of the error
func (e *errorType) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// Is reports whether the error is of the specified type
func (e *errorType) Is(target error) bool {
	if e == nil {
		return target == nil
	}
	return errors.Is(e.baseErr, target)
}

// NewAuthError creates a new authentication error
func NewAuthError(msg string) error {
	return &errorType{
		baseErr:   ErrAuthentication,
		msg:       msg,
		retryable: false,
	}
}

// NewValidationError creates a new validation error
func NewValidationError(msg string) error {
	return &errorType{
		baseErr:   ErrValidation,
		msg:       msg,
		retryable: false,
	}
}

// NewRateLimitError creates a new rate limit error
func NewRateLimitError(msg string) error {
	return &errorType{
		baseErr:   ErrRateLimit,
		msg:       msg,
		retryable: true,
	}
}

// NewPublishError creates a new publish error
func NewPublishError(msg string, cause error) error {
	return &errorType{
		baseErr:   ErrPublish,
		msg:       msg,
		cause:     cause,
		retryable: true,
	}
}

// NewConnectionError creates a new connection error
func NewConnectionError(msg string) error {
	return &errorType{
		baseErr:   ErrConnection,
		msg:       msg,
		retryable: true,
	}
}

// NewNotFoundError creates a new not found error
func NewNotFoundError(msg string) error {
	return &errorType{
		baseErr:   ErrNotFound,
		msg:       msg,
		retryable: false,
	}
}

// NewInternalError creates a new internal error
func NewInternalError(msg string) error {
	return &errorType{
		baseErr:   ErrInternal,
		msg:       msg,
		retryable: false,
	}
}

// Wrap wraps an error with additional context
func Wrap(err error, msg string) error {
	if err == nil {
		return nil
	}

	// Check if it's our custom type
	if customErr, ok := err.(*errorType); ok {
		return &errorType{
			baseErr:   customErr.baseErr,
			msg:       msg + ": " + customErr.msg,
			cause:     customErr.cause,
			details:   customErr.details,
			retryable: customErr.retryable,
		}
	}

	// If it's a standard error, wrap it as an internal error
	return &errorType{
		baseErr:   ErrInternal,
		msg:       msg,
		cause:     err,
		retryable: false,
	}
}

// Unwrap returns the wrapped error, following Go 1.13 error unwrapping convention
func Unwrap(err error) error {
	return errors.Unwrap(err)
}

// WithDetails adds detail information to an error
func WithDetails(err error, details map[string]interface{}) error {
	if err == nil {
		return nil
	}

	if customErr, ok := err.(*errorType); ok {
		return &errorType{
			baseErr:   customErr.baseErr,
			msg:       customErr.msg,
			cause:     customErr.cause,
			details:   details,
			retryable: customErr.retryable,
		}
	}

	return &errorType{
		baseErr:   ErrInternal,
		msg:       err.Error(),
		details:   details,
		retryable: false,
	}
}

// MakeRetryable marks an error as retryable
func MakeRetryable(err error) error {
	if err == nil {
		return nil
	}

	if customErr, ok := err.(*errorType); ok {
		return &errorType{
			baseErr:   customErr.baseErr,
			msg:       customErr.msg,
			cause:     customErr.cause,
			details:   customErr.details,
			retryable: true,
		}
	}

	return &errorType{
		baseErr:   ErrInternal,
		msg:       err.Error(),
		retryable: true,
	}
}

// IsAuthError checks if the error is an authentication error
func IsAuthError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrAuthentication)
}

// IsValidationError checks if the error is a validation error
func IsValidationError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrValidation)
}

// IsRateLimitError checks if the error is a rate limit error
func IsRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrRateLimit)
}

// IsPublishError checks if the error is a publish error
func IsPublishError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrPublish)
}

// IsConnectionError checks if the error is a connection error
func IsConnectionError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrConnection)
}

// IsNotFoundError checks if the error is a not found error
func IsNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrNotFound)
}

// IsInternalError checks if the error is an internal error
func IsInternalError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrInternal)
}

// IsRetryable checks if the error is retryable
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	customErr, ok := err.(*errorType)
	if !ok {
		return false
	}

	return customErr.retryable
}

// Format returns a properly formatted error string
func Format(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// Make sure our errorType implements ErrorWithDetails
func (e *errorType) Details() map[string]interface{} {
	if e == nil {
		return nil
	}
	return e.details
}

// GetDetails returns error details if available, nil otherwise
func GetDetails(err error) map[string]interface{} {
	if err == nil {
		return nil
	}

	if detailedErr, ok := err.(ErrorWithDetails); ok {
		return detailedErr.Details()
	}

	return nil
}

// WithRetryOption adds a retry duration suggestion to an error
func WithRetryOption(err error, retrySeconds int) error {
	if err == nil {
		return nil
	}

	if customErr, ok := err.(*errorType); ok {
		// Add retry info to details
		details := customErr.details
		if details == nil {
			details = make(map[string]interface{})
		}
		details["retry_after"] = retrySeconds

		return &errorType{
			baseErr:   customErr.baseErr,
			msg:       customErr.msg,
			cause:     customErr.cause,
			details:   details,
			retryable: true, // Always make it retryable if we suggest retry
		}
	}

	// If it's not our custom error type, wrap it
	return WithDetails(
		MakeRetryable(Wrap(err, "temporary failure")),
		map[string]interface{}{
			"retry_after": retrySeconds,
		},
	)
}

// GetRetryOption extracts the retry duration from an error if available
func GetRetryOption(err error) (int, bool) {
	details := GetDetails(err)
	if details == nil {
		return 0, false
	}

	if retry, ok := details["retry_after"]; ok {
		if retryInt, ok := retry.(int); ok {
			return retryInt, true
		}
	}

	return 0, false
}

// ErrorResponse provides a consistent structure for error responses
type ErrorResponse struct {
	Status     string                 `json:"status"`
	Message    string                 `json:"message"`
	ErrorType  string                 `json:"error_type"`
	RetryAfter int                    `json:"retry_after,omitempty"`
	Details    map[string]interface{} `json:"details,omitempty"`
}

// ToErrorResponse converts an error to a standardized ErrorResponse
func ToErrorResponse(err error) ErrorResponse {
	if err == nil {
		return ErrorResponse{
			Status:  "error",
			Message: "Unknown error",
		}
	}

	response := ErrorResponse{
		Status:  "error",
		Message: Format(err),
		Details: GetDetails(err),
	}

	// Set error type
	switch {
	case IsAuthError(err):
		response.ErrorType = "auth"
	case IsValidationError(err):
		response.ErrorType = "validation"
	case IsRateLimitError(err):
		response.ErrorType = "rate_limit"
	case IsConnectionError(err):
		response.ErrorType = "connection"
	case IsPublishError(err):
		response.ErrorType = "publish"
	case IsNotFoundError(err):
		response.ErrorType = "not_found"
	default:
		response.ErrorType = "internal"
	}

	// Add retry info if available
	if retry, ok := GetRetryOption(err); ok {
		response.RetryAfter = retry
	} else if IsRetryable(err) {
		// Default retry suggestion for retryable errors
		response.RetryAfter = 30
	}

	return response
}
