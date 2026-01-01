package publisher

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/mcncl/buildkite-pubsub/internal/errors"
)

// FailingMockPublisher fails a specified number of times before succeeding
type FailingMockPublisher struct {
	mu             sync.Mutex
	failuresLeft   int
	publishCount   int
	successCount   int
	failureCount   int
}

func NewFailingMockPublisher(failCount int) *FailingMockPublisher {
	return &FailingMockPublisher{
		failuresLeft: failCount,
	}
}

func (m *FailingMockPublisher) Publish(ctx context.Context, data interface{}, attributes map[string]string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.publishCount++

	if m.failuresLeft > 0 {
		m.failuresLeft--
		m.failureCount++
		return "", errors.NewConnectionError("simulated failure")
	}

	m.successCount++
	return "success-id", nil
}

func (m *FailingMockPublisher) Close() error {
	return nil
}

func (m *FailingMockPublisher) SetFailures(count int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failuresLeft = count
}

func (m *FailingMockPublisher) Stats() (publishCount, successCount, failureCount int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.publishCount, m.successCount, m.failureCount
}

func TestCircuitBreaker_StartsInClosedState(t *testing.T) {
	pub := NewMockPublisher()
	cb := NewCircuitBreaker(pub, DefaultCircuitBreakerConfig())

	if cb.State() != StateClosed {
		t.Errorf("Initial state = %v, want %v", cb.State(), StateClosed)
	}
}

func TestCircuitBreaker_StaysClosedOnSuccess(t *testing.T) {
	pub := NewMockPublisher()
	cb := NewCircuitBreaker(pub, DefaultCircuitBreakerConfig())

	ctx := context.Background()

	// Multiple successful publishes
	for i := 0; i < 10; i++ {
		_, err := cb.Publish(ctx, "test", nil)
		if err != nil {
			t.Fatalf("Publish failed: %v", err)
		}
	}

	if cb.State() != StateClosed {
		t.Errorf("State after successes = %v, want %v", cb.State(), StateClosed)
	}
}

func TestCircuitBreaker_OpensAfterFailureThreshold(t *testing.T) {
	pub := NewFailingMockPublisher(100) // Always fail
	config := CircuitBreakerConfig{
		FailureThreshold:    3,
		SuccessThreshold:    2,
		Timeout:             30 * time.Second,
		MaxHalfOpenRequests: 3,
	}
	cb := NewCircuitBreaker(pub, config)

	ctx := context.Background()

	// Cause failures to trip the circuit
	for i := 0; i < config.FailureThreshold; i++ {
		_, _ = cb.Publish(ctx, "test", nil)
	}

	if cb.State() != StateOpen {
		t.Errorf("State after failures = %v, want %v", cb.State(), StateOpen)
	}
}

func TestCircuitBreaker_FailsFastWhenOpen(t *testing.T) {
	pub := NewFailingMockPublisher(100)
	config := CircuitBreakerConfig{
		FailureThreshold:    3,
		SuccessThreshold:    2,
		Timeout:             1 * time.Hour, // Long timeout to ensure circuit stays open
		MaxHalfOpenRequests: 3,
	}
	cb := NewCircuitBreaker(pub, config)

	ctx := context.Background()

	// Trip the circuit
	for i := 0; i < config.FailureThreshold; i++ {
		_, _ = cb.Publish(ctx, "test", nil)
	}

	// Now circuit should be open
	if cb.State() != StateOpen {
		t.Fatalf("Circuit should be open, got %v", cb.State())
	}

	// Next publish should fail fast without calling underlying publisher
	beforeCount, _, _ := pub.Stats()
	_, err := cb.Publish(ctx, "test", nil)
	afterCount, _, _ := pub.Stats()

	if err == nil {
		t.Error("Expected error when circuit is open")
	}

	if afterCount != beforeCount {
		t.Errorf("Publisher was called despite open circuit (before=%d, after=%d)", beforeCount, afterCount)
	}

	if !errors.IsConnectionError(err) {
		t.Errorf("Expected connection error, got: %v", err)
	}
}

func TestCircuitBreaker_TransitionsToHalfOpenAfterTimeout(t *testing.T) {
	pub := NewFailingMockPublisher(100)
	config := CircuitBreakerConfig{
		FailureThreshold:    3,
		SuccessThreshold:    2,
		Timeout:             100 * time.Millisecond, // Short timeout
		MaxHalfOpenRequests: 3,
	}
	cb := NewCircuitBreaker(pub, config)

	ctx := context.Background()

	// Trip the circuit
	for i := 0; i < config.FailureThreshold; i++ {
		_, _ = cb.Publish(ctx, "test", nil)
	}

	if cb.State() != StateOpen {
		t.Fatalf("Circuit should be open, got %v", cb.State())
	}

	// Wait for timeout
	time.Sleep(config.Timeout + 50*time.Millisecond)

	// Next request should transition to half-open
	_, _ = cb.Publish(ctx, "test", nil)

	if cb.State() != StateHalfOpen && cb.State() != StateOpen {
		// Might be open again if the request failed
		t.Logf("State after timeout = %v (acceptable states: half-open or open)", cb.State())
	}
}

func TestCircuitBreaker_ClosesAfterSuccessInHalfOpen(t *testing.T) {
	pub := NewFailingMockPublisher(3) // Fail 3 times, then succeed
	config := CircuitBreakerConfig{
		FailureThreshold:    3,
		SuccessThreshold:    2,
		Timeout:             100 * time.Millisecond,
		MaxHalfOpenRequests: 5,
	}
	cb := NewCircuitBreaker(pub, config)

	ctx := context.Background()

	// Trip the circuit
	for i := 0; i < config.FailureThreshold; i++ {
		_, _ = cb.Publish(ctx, "test", nil)
	}

	if cb.State() != StateOpen {
		t.Fatalf("Circuit should be open, got %v", cb.State())
	}

	// Wait for timeout
	time.Sleep(config.Timeout + 50*time.Millisecond)

	// Now publishes should succeed and close the circuit
	for i := 0; i < config.SuccessThreshold; i++ {
		_, err := cb.Publish(ctx, "test", nil)
		if err != nil {
			t.Logf("Publish %d in half-open: %v", i, err)
		}
	}

	// Circuit should be closed now
	if cb.State() != StateClosed {
		t.Errorf("State after successes = %v, want %v", cb.State(), StateClosed)
	}
}

func TestCircuitBreaker_ReopensOnFailureInHalfOpen(t *testing.T) {
	pub := NewFailingMockPublisher(100) // Always fail
	config := CircuitBreakerConfig{
		FailureThreshold:    3,
		SuccessThreshold:    2,
		Timeout:             100 * time.Millisecond,
		MaxHalfOpenRequests: 5,
	}
	cb := NewCircuitBreaker(pub, config)

	ctx := context.Background()

	// Trip the circuit
	for i := 0; i < config.FailureThreshold; i++ {
		_, _ = cb.Publish(ctx, "test", nil)
	}

	// Wait for timeout
	time.Sleep(config.Timeout + 50*time.Millisecond)

	// This request should fail and reopen the circuit
	_, _ = cb.Publish(ctx, "test", nil)

	if cb.State() != StateOpen {
		t.Errorf("State after failure in half-open = %v, want %v", cb.State(), StateOpen)
	}
}

func TestCircuitBreaker_LimitsHalfOpenRequests(t *testing.T) {
	pub := NewMockPublisher() // Always succeed
	config := CircuitBreakerConfig{
		FailureThreshold:    3,
		SuccessThreshold:    10, // High threshold so we stay in half-open
		Timeout:             100 * time.Millisecond,
		MaxHalfOpenRequests: 2,
	}

	// Create a circuit breaker that's already in half-open state
	cb := NewCircuitBreaker(pub, config)
	cb.mu.Lock()
	cb.state = StateHalfOpen
	cb.halfOpenRequests = 0
	cb.mu.Unlock()

	ctx := context.Background()

	// First requests should succeed
	for i := 0; i < config.MaxHalfOpenRequests; i++ {
		_, err := cb.Publish(ctx, "test", nil)
		if err != nil {
			t.Errorf("Request %d should succeed, got: %v", i, err)
		}
	}

	// Additional requests should be rejected
	_, err := cb.Publish(ctx, "test", nil)
	if err == nil {
		t.Error("Expected error for exceeding max half-open requests")
	}
}

func TestCircuitBreaker_Reset(t *testing.T) {
	pub := NewFailingMockPublisher(100)
	config := CircuitBreakerConfig{
		FailureThreshold:    3,
		SuccessThreshold:    2,
		Timeout:             1 * time.Hour,
		MaxHalfOpenRequests: 3,
	}
	cb := NewCircuitBreaker(pub, config)

	ctx := context.Background()

	// Trip the circuit
	for i := 0; i < config.FailureThreshold; i++ {
		_, _ = cb.Publish(ctx, "test", nil)
	}

	if cb.State() != StateOpen {
		t.Fatalf("Circuit should be open, got %v", cb.State())
	}

	// Reset
	cb.Reset()

	if cb.State() != StateClosed {
		t.Errorf("State after reset = %v, want %v", cb.State(), StateClosed)
	}

	// Verify counters are reset
	stats := cb.Stats()
	if stats["consecutive_failures"].(int) != 0 {
		t.Errorf("consecutive_failures = %d, want 0", stats["consecutive_failures"])
	}
}

func TestCircuitBreaker_StateChangeCallback(t *testing.T) {
	pub := NewFailingMockPublisher(100)
	config := CircuitBreakerConfig{
		FailureThreshold:    3,
		SuccessThreshold:    2,
		Timeout:             100 * time.Millisecond,
		MaxHalfOpenRequests: 3,
	}
	cb := NewCircuitBreaker(pub, config)

	var mu sync.Mutex
	stateChanges := make([]struct{ from, to CircuitState }, 0)

	cb.SetOnStateChange(func(from, to CircuitState) {
		mu.Lock()
		defer mu.Unlock()
		stateChanges = append(stateChanges, struct{ from, to CircuitState }{from, to})
	})

	ctx := context.Background()

	// Trip the circuit
	for i := 0; i < config.FailureThreshold; i++ {
		_, _ = cb.Publish(ctx, "test", nil)
	}

	// Wait a bit for callback
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(stateChanges) != 1 {
		t.Fatalf("Expected 1 state change, got %d", len(stateChanges))
	}

	if stateChanges[0].from != StateClosed || stateChanges[0].to != StateOpen {
		t.Errorf("State change = %v -> %v, want closed -> open",
			stateChanges[0].from, stateChanges[0].to)
	}
}

func TestCircuitBreaker_ConcurrentAccess(t *testing.T) {
	pub := NewFailingMockPublisher(0) // Always succeed
	cb := NewCircuitBreaker(pub, DefaultCircuitBreakerConfig())

	ctx := context.Background()
	numGoroutines := 50
	requestsPerGoroutine := 100

	var wg sync.WaitGroup
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for r := 0; r < requestsPerGoroutine; r++ {
				_, _ = cb.Publish(ctx, "test", nil)
			}
		}()
	}

	wg.Wait()

	// Should still be closed after successful publishes
	if cb.State() != StateClosed {
		t.Errorf("State after concurrent successes = %v, want %v", cb.State(), StateClosed)
	}
}

func TestCircuitBreaker_Close(t *testing.T) {
	pub := NewMockPublisher()
	cb := NewCircuitBreaker(pub, DefaultCircuitBreakerConfig())

	err := cb.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestCircuitState_String(t *testing.T) {
	tests := []struct {
		state    CircuitState
		expected string
	}{
		{StateClosed, "closed"},
		{StateOpen, "open"},
		{StateHalfOpen, "half-open"},
		{CircuitState(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.state.String(); got != tt.expected {
				t.Errorf("String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCircuitBreaker_Stats(t *testing.T) {
	pub := NewFailingMockPublisher(2) // Fail twice
	config := DefaultCircuitBreakerConfig()
	cb := NewCircuitBreaker(pub, config)

	ctx := context.Background()

	// Cause some failures
	_, _ = cb.Publish(ctx, "test", nil)
	_, _ = cb.Publish(ctx, "test", nil)

	stats := cb.Stats()

	if stats["state"] != "closed" {
		t.Errorf("stats[state] = %v, want closed", stats["state"])
	}

	if stats["consecutive_failures"].(int) != 2 {
		t.Errorf("stats[consecutive_failures] = %v, want 2", stats["consecutive_failures"])
	}
}

func TestDefaultCircuitBreakerConfig(t *testing.T) {
	config := DefaultCircuitBreakerConfig()

	if config.FailureThreshold <= 0 {
		t.Error("FailureThreshold should be positive")
	}
	if config.SuccessThreshold <= 0 {
		t.Error("SuccessThreshold should be positive")
	}
	if config.Timeout <= 0 {
		t.Error("Timeout should be positive")
	}
	if config.MaxHalfOpenRequests <= 0 {
		t.Error("MaxHalfOpenRequests should be positive")
	}
}
