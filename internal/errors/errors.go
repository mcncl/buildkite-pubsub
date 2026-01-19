package errors

import (
	"errors"
	"fmt"
)

// Sentinel errors
var (
	ErrAuth       = errors.New("authentication error")
	ErrValidation = errors.New("validation error")
	ErrRateLimit  = errors.New("rate limit exceeded")
	ErrPublish    = errors.New("publish error")
	ErrConnection = errors.New("connection error")
	ErrNotFound   = errors.New("not found error")
	ErrInternal   = errors.New("internal error")
)

// Constructor functions

func NewAuthError(msg string) error {
	return fmt.Errorf("%w: %s", ErrAuth, msg)
}

func NewValidationError(msg string) error {
	return fmt.Errorf("%w: %s", ErrValidation, msg)
}

func NewRateLimitError(msg string) error {
	return fmt.Errorf("%w: %s", ErrRateLimit, msg)
}

func NewPublishError(msg string, cause error) error {
	if cause != nil {
		return fmt.Errorf("%w: %s: %v", ErrPublish, msg, cause)
	}
	return fmt.Errorf("%w: %s", ErrPublish, msg)
}

func NewConnectionError(msg string) error {
	return fmt.Errorf("%w: %s", ErrConnection, msg)
}

func NewNotFoundError(msg string) error {
	return fmt.Errorf("%w: %s", ErrNotFound, msg)
}

func NewInternalError(msg string) error {
	return fmt.Errorf("%w: %s", ErrInternal, msg)
}

// Type checking functions

func IsAuthError(err error) bool {
	return errors.Is(err, ErrAuth)
}

func IsValidationError(err error) bool {
	return errors.Is(err, ErrValidation)
}

func IsRateLimitError(err error) bool {
	return errors.Is(err, ErrRateLimit)
}

func IsPublishError(err error) bool {
	return errors.Is(err, ErrPublish)
}

func IsConnectionError(err error) bool {
	return errors.Is(err, ErrConnection)
}

func IsNotFoundError(err error) bool {
	return errors.Is(err, ErrNotFound)
}

func IsInternalError(err error) bool {
	return errors.Is(err, ErrInternal)
}

func IsRetryable(err error) bool {
	return errors.Is(err, ErrConnection) || errors.Is(err, ErrPublish) || errors.Is(err, ErrRateLimit)
}

// Wrap wraps an error with additional context
func Wrap(err error, msg string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", msg, err)
}

// Format returns err.Error() or empty string if nil
func Format(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
