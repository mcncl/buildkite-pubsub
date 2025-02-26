# Error Handling Package

This package provides a standardized error handling framework for the Buildkite PubSub Webhook application. It enables consistent error handling, error types, and error reporting across the application.

## Features

- Standard error types for common scenarios
- Error wrapping with context preservation
- Error classification (retryable vs non-retryable)
- Structured error formatting with details
- Compatibility with Go's standard `errors` package

## Usage Examples

### Creating Specific Error Types

```go
// Authentication error
err := errors.NewAuthError("invalid webhook token")

// Validation error
err := errors.NewValidationError("missing required field 'event_type'")

// Rate limit error
err := errors.NewRateLimitError("too many requests")

// Publishing error
err := errors.NewPublishError("failed to publish to Pub/Sub", pubsubErr)

// Connection error
err := errors.NewConnectionError("could not connect to Pub/Sub")

// Not found error
err := errors.NewNotFoundError("topic does not exist")

// Internal error
err := errors.NewInternalError("unexpected error occurred")
```

### Adding Context to Errors

```go
// Wrap an error with additional context
baseErr := someFunc()
err := errors.Wrap(baseErr, "failed to process webhook")

// Add structured details to an error
err = errors.WithDetails(err, map[string]interface{}{
    "request_id": requestID,
    "event_type": eventType,
    "payload_size": len(payload),
})
```

### Checking Error Types

```go
// Check for specific error types
if errors.IsAuthError(err) {
    // Handle authentication error
    return http.StatusUnauthorized, "Invalid authentication"
}

if errors.IsValidationError(err) {
    // Handle validation error
    return http.StatusBadRequest, "Invalid input"
}

if errors.IsRateLimitError(err) {
    // Handle rate limit error
    return http.StatusTooManyRequests, "Rate limit exceeded"
}

// Check if error is retryable
if errors.IsRetryable(err) {
    // Implement retry logic
    return retryOperation()
}
```

### Formatting Errors

```go
// Format an error with all details for logging
logEntry := map[string]interface{}{
    "error": errors.Format(err),
    "request_id": requestID,
    "timestamp": time.Now(),
}
logger.Info("Request failed", logEntry)
```

## Error Types

| Error Type | Function | Retryable | Typical HTTP Status |
|------------|----------|-----------|---------------------|
| Authentication | `NewAuthError()` | No | 401 Unauthorized |
| Validation | `NewValidationError()` | No | 400 Bad Request |
| Rate Limit | `NewRateLimitError()` | Yes | 429 Too Many Requests |
| Publish | `NewPublishError()` | Yes | 500 Internal Server Error |
| Connection | `NewConnectionError()` | Yes | 500 Internal Server Error |
| Not Found | `NewNotFoundError()` | No | 404 Not Found |
| Internal | `NewInternalError()` | No | 500 Internal Server Error |

## Best Practices

1. **Use specific error types** instead of generic errors wherever possible to provide better context.
2. **Add details** to errors when they contain useful debugging information.
3. **Check for retryable errors** to implement appropriate retry logic.
4. **Wrap errors** to provide context as they bubble up through the call stack.
5. **Never expose internal error details** to external clients, but do log them internally.
