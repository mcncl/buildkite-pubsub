package errors

import (
	"fmt"
)

type WebhookError struct {
    Code    int
    Message string
    Err     error
}

func (e *WebhookError) Error() string {
    if e.Err != nil {
        return fmt.Sprintf("%s: %v", e.Message, e.Err)
    }
    return e.Message
}

func NewWebhookError(code int, message string, err error) *WebhookError {
    return &WebhookError{
        Code:    code,
        Message: message,
        Err:     err,
    }
}
