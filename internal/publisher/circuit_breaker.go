package publisher

import (
	"context"
	"sync"
	"time"

	"github.com/mcncl/buildkite-pubsub/internal/errors"
)

// CircuitState represents the state of the circuit breaker
type CircuitState int

const (
	// StateClosed means the circuit breaker is closed and requests pass through
	StateClosed CircuitState = iota
	// StateOpen means the circuit breaker is open and requests fail immediately
	StateOpen
	// StateHalfOpen means the circuit breaker is testing if the service has recovered
	StateHalfOpen
)

func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreakerConfig holds configuration for the circuit breaker
type CircuitBreakerConfig struct {
	// FailureThreshold is the number of consecutive failures before opening the circuit
	FailureThreshold int
	// SuccessThreshold is the number of consecutive successes in half-open state to close the circuit
	SuccessThreshold int
	// Timeout is how long the circuit stays open before transitioning to half-open
	Timeout time.Duration
	// MaxHalfOpenRequests is the max number of requests allowed in half-open state
	MaxHalfOpenRequests int
}

// DefaultCircuitBreakerConfig returns sensible defaults for the circuit breaker
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold:    5,
		SuccessThreshold:    2,
		Timeout:             30 * time.Second,
		MaxHalfOpenRequests: 3,
	}
}

// CircuitBreaker implements the circuit breaker pattern for the publisher
type CircuitBreaker struct {
	publisher Publisher
	config    CircuitBreakerConfig

	mu                   sync.RWMutex
	state                CircuitState
	consecutiveFailures  int
	consecutiveSuccesses int
	halfOpenRequests     int
	lastFailureTime      time.Time
	lastStateChange      time.Time

	// Callbacks for state changes (optional, for metrics/logging)
	onStateChange func(from, to CircuitState)
}

// NewCircuitBreaker wraps a publisher with circuit breaker protection
func NewCircuitBreaker(pub Publisher, config CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		publisher:       pub,
		config:          config,
		state:           StateClosed,
		lastStateChange: time.Now(),
	}
}

// SetOnStateChange sets a callback for state changes
func (cb *CircuitBreaker) SetOnStateChange(fn func(from, to CircuitState)) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.onStateChange = fn
}

// State returns the current state of the circuit breaker
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Stats returns current circuit breaker statistics
func (cb *CircuitBreaker) Stats() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return map[string]interface{}{
		"state":                 cb.state.String(),
		"consecutive_failures":  cb.consecutiveFailures,
		"consecutive_successes": cb.consecutiveSuccesses,
		"last_failure_time":     cb.lastFailureTime,
		"last_state_change":     cb.lastStateChange,
	}
}

// Publish publishes a message through the circuit breaker
func (cb *CircuitBreaker) Publish(ctx context.Context, data interface{}, attributes map[string]string) (string, error) {
	// Check if circuit allows the request
	if err := cb.beforeRequest(); err != nil {
		return "", err
	}

	// Attempt to publish
	msgID, err := cb.publisher.Publish(ctx, data, attributes)

	// Record the result
	cb.afterRequest(err)

	return msgID, err
}

// Close closes the underlying publisher
func (cb *CircuitBreaker) Close() error {
	return cb.publisher.Close()
}

// beforeRequest checks if the request should be allowed
func (cb *CircuitBreaker) beforeRequest() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()

	switch cb.state {
	case StateClosed:
		// Allow all requests
		return nil

	case StateOpen:
		// Check if timeout has elapsed
		if now.Sub(cb.lastFailureTime) >= cb.config.Timeout {
			// Transition to half-open
			cb.transitionTo(StateHalfOpen)
			cb.halfOpenRequests = 1
			return nil
		}
		// Circuit is still open - fail fast
		return errors.NewConnectionError("circuit breaker is open")

	case StateHalfOpen:
		// Allow limited requests to test if service is healthy
		if cb.halfOpenRequests >= cb.config.MaxHalfOpenRequests {
			return errors.NewConnectionError("circuit breaker: too many requests in half-open state")
		}
		cb.halfOpenRequests++
		return nil

	default:
		return nil
	}
}

// afterRequest records the result of the request
func (cb *CircuitBreaker) afterRequest(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.recordFailure()
	} else {
		cb.recordSuccess()
	}
}

// recordFailure handles a failed request
func (cb *CircuitBreaker) recordFailure() {
	cb.consecutiveFailures++
	cb.consecutiveSuccesses = 0
	cb.lastFailureTime = time.Now()

	switch cb.state {
	case StateClosed:
		// Check if we should open the circuit
		if cb.consecutiveFailures >= cb.config.FailureThreshold {
			cb.transitionTo(StateOpen)
		}

	case StateHalfOpen:
		// Any failure in half-open state trips the circuit again
		cb.transitionTo(StateOpen)
	}
}

// recordSuccess handles a successful request
func (cb *CircuitBreaker) recordSuccess() {
	cb.consecutiveSuccesses++
	cb.consecutiveFailures = 0

	switch cb.state {
	case StateHalfOpen:
		// Check if we should close the circuit
		if cb.consecutiveSuccesses >= cb.config.SuccessThreshold {
			cb.transitionTo(StateClosed)
		}
	}
}

// transitionTo changes the circuit breaker state
func (cb *CircuitBreaker) transitionTo(newState CircuitState) {
	oldState := cb.state
	if oldState == newState {
		return
	}

	cb.state = newState
	cb.lastStateChange = time.Now()
	cb.halfOpenRequests = 0

	// Call state change callback if set
	if cb.onStateChange != nil {
		// Call in goroutine to avoid blocking
		go cb.onStateChange(oldState, newState)
	}
}

// Reset manually resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.transitionTo(StateClosed)
	cb.consecutiveFailures = 0
	cb.consecutiveSuccesses = 0
	cb.halfOpenRequests = 0
}

// CircuitBreakerPublisher is a type alias for the circuit breaker that implements Publisher
type CircuitBreakerPublisher = CircuitBreaker
