package request

// Package request provides HTTP middleware components for request handling.
//
// It includes middleware for:
//   - Request ID generation and propagation
//   - Request timeout management
//
// The middleware in this package is designed to be used with standard
// http.Handler interfaces and can be easily chained together.
//
// Example usage:
//
//	handler := request.WithRequestID(
//		request.WithTimeout(5*time.Second)(
//			yourHandler,
//		),
//	)
