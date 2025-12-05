package webhook

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/mcncl/buildkite-pubsub/internal/errors"
	"github.com/mcncl/buildkite-pubsub/internal/metrics"
	"github.com/mcncl/buildkite-pubsub/internal/publisher"
	"github.com/prometheus/client_golang/prometheus"
)

// MockPublisherWithRetryableErrors simulates failures that should retry
type MockPublisherWithRetryableErrors struct {
	publisher.MockPublisher
	mu                    sync.Mutex
	failuresBeforeSuccess int
	attemptCount          int
	attemptTimes          []time.Time
	publishCalls          []publishCall
}

type publishCall struct {
	timestamp  time.Time
	data       interface{}
	attributes map[string]string
}

func NewMockPublisherWithRetryableErrors(failuresBeforeSuccess int) *MockPublisherWithRetryableErrors {
	return &MockPublisherWithRetryableErrors{
		failuresBeforeSuccess: failuresBeforeSuccess,
		attemptTimes:          make([]time.Time, 0),
		publishCalls:          make([]publishCall, 0),
	}
}

func (m *MockPublisherWithRetryableErrors) Publish(ctx context.Context, data interface{}, attributes map[string]string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.attemptCount++
	now := time.Now()
	m.attemptTimes = append(m.attemptTimes, now)
	m.publishCalls = append(m.publishCalls, publishCall{
		timestamp:  now,
		data:       data,
		attributes: attributes,
	})

	// Fail N times before succeeding
	if m.attemptCount <= m.failuresBeforeSuccess {
		return "", errors.NewConnectionError("temporary connection failure")
	}

	return "mock-message-id", nil
}

func (m *MockPublisherWithRetryableErrors) AttemptCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.attemptCount
}

func (m *MockPublisherWithRetryableErrors) BackoffDurations() []time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()

	durations := make([]time.Duration, 0)
	for i := 1; i < len(m.attemptTimes); i++ {
		durations = append(durations, m.attemptTimes[i].Sub(m.attemptTimes[i-1]))
	}
	return durations
}

func (m *MockPublisherWithRetryableErrors) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.attemptCount = 0
	m.attemptTimes = make([]time.Time, 0)
	m.publishCalls = make([]publishCall, 0)
}

// MockPublisherWithErrorType simulates specific error types
type MockPublisherWithErrorType struct {
	publisher.MockPublisher
	mu           sync.Mutex
	errorType    string
	attemptCount int
}

func NewMockPublisherWithErrorType(errorType string) *MockPublisherWithErrorType {
	return &MockPublisherWithErrorType{
		errorType: errorType,
	}
}

func (m *MockPublisherWithErrorType) Publish(ctx context.Context, data interface{}, attributes map[string]string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.attemptCount++

	switch m.errorType {
	case "connection":
		return "", errors.NewConnectionError("connection refused")
	case "publish":
		return "", errors.NewPublishError("failed to publish message", fmt.Errorf("publish error"))
	case "rate_limit":
		return "", errors.NewRateLimitError("too many requests")
	case "auth":
		return "", errors.NewAuthError("authentication failed")
	case "validation":
		return "", errors.NewValidationError("invalid payload")
	default:
		return "", fmt.Errorf("unknown error")
	}
}

func (m *MockPublisherWithErrorType) AttemptCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.attemptCount
}

func (m *MockPublisherWithErrorType) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.attemptCount = 0
}

func TestPublishWithRetry_SuccessScenarios(t *testing.T) {
	// Initialize metrics once for all subtests
	reg := prometheus.NewRegistry()
	if err := metrics.InitMetrics(reg); err != nil {
		t.Fatalf("failed to initialize metrics: %v", err)
	}

	tests := []struct {
		name                  string
		failuresBeforeSuccess int
		maxRetries            int
		wantAttempts          int
		wantSuccess           bool
		wantMessageID         string
	}{
		{
			name:                  "succeeds on first attempt",
			failuresBeforeSuccess: 0,
			maxRetries:            3,
			wantAttempts:          1,
			wantSuccess:           true,
			wantMessageID:         "mock-message-id",
		},
		{
			name:                  "succeeds after 1 retry",
			failuresBeforeSuccess: 1,
			maxRetries:            3,
			wantAttempts:          2,
			wantSuccess:           true,
			wantMessageID:         "mock-message-id",
		},
		{
			name:                  "succeeds after 2 retries",
			failuresBeforeSuccess: 2,
			maxRetries:            3,
			wantAttempts:          3,
			wantSuccess:           true,
			wantMessageID:         "mock-message-id",
		},
		{
			name:                  "succeeds on last possible retry",
			failuresBeforeSuccess: 3,
			maxRetries:            3,
			wantAttempts:          4, // initial + 3 retries
			wantSuccess:           true,
			wantMessageID:         "mock-message-id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockPub := NewMockPublisherWithRetryableErrors(tt.failuresBeforeSuccess)

			handler := &Handler{
				publisher: mockPub,
			}

			ctx := context.Background()
			testData := map[string]string{"test": "data"}
			testAttrs := map[string]string{"event_type": "build.started"}

			msgID, err := handler.publishWithRetry(ctx, testData, testAttrs, tt.maxRetries)

			// Check attempt count
			if mockPub.AttemptCount() != tt.wantAttempts {
				t.Errorf("AttemptCount() = %d, want %d", mockPub.AttemptCount(), tt.wantAttempts)
			}

			// Check success/failure
			if tt.wantSuccess {
				if err != nil {
					t.Errorf("Expected success but got error: %v", err)
				}
				if msgID != tt.wantMessageID {
					t.Errorf("MessageID = %s, want %s", msgID, tt.wantMessageID)
				}
			} else {
				if err == nil {
					t.Error("Expected error but got success")
				}
			}
		})
	}
}

func TestPublishWithRetry_FailureScenarios(t *testing.T) {
	// Initialize metrics once for all subtests
	reg := prometheus.NewRegistry()
	if err := metrics.InitMetrics(reg); err != nil {
		t.Fatalf("failed to initialize metrics: %v", err)
	}

	tests := []struct {
		name                  string
		failuresBeforeSuccess int
		maxRetries            int
		wantAttempts          int
		wantError             bool
	}{
		{
			name:                  "fails after max retries - 5 failures with 3 retries",
			failuresBeforeSuccess: 5,
			maxRetries:            3,
			wantAttempts:          4, // initial + 3 retries
			wantError:             true,
		},
		{
			name:                  "fails after max retries - 10 failures with 5 retries",
			failuresBeforeSuccess: 10,
			maxRetries:            5,
			wantAttempts:          6, // initial + 5 retries
			wantError:             true,
		},
		{
			name:                  "fails with zero retries",
			failuresBeforeSuccess: 1,
			maxRetries:            0,
			wantAttempts:          1, // only initial attempt
			wantError:             true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockPub := NewMockPublisherWithRetryableErrors(tt.failuresBeforeSuccess)

			handler := &Handler{
				publisher: mockPub,
			}

			ctx := context.Background()
			testData := map[string]string{"test": "data"}
			testAttrs := map[string]string{"event_type": "build.started"}

			msgID, err := handler.publishWithRetry(ctx, testData, testAttrs, tt.maxRetries)

			// Check attempt count
			if mockPub.AttemptCount() != tt.wantAttempts {
				t.Errorf("AttemptCount() = %d, want %d", mockPub.AttemptCount(), tt.wantAttempts)
			}

			// Should return error
			if !tt.wantError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
			if tt.wantError && err == nil {
				t.Error("Expected error but got success")
			}

			// Message ID should be empty on failure
			if tt.wantError && msgID != "" {
				t.Errorf("Expected empty message ID on failure, got: %s", msgID)
			}
		})
	}
}

func TestPublishWithRetry_ExponentialBackoff(t *testing.T) {
	// Initialize metrics
	reg := prometheus.NewRegistry()
	if err := metrics.InitMetrics(reg); err != nil {
		t.Fatalf("failed to initialize metrics: %v", err)
	}

	mockPub := NewMockPublisherWithRetryableErrors(3) // Will retry 3 times

	handler := &Handler{
		publisher: mockPub,
	}

	ctx := context.Background()
	testData := map[string]string{"test": "data"}
	testAttrs := map[string]string{"event_type": "build.started"}

	_, err := handler.publishWithRetry(ctx, testData, testAttrs, 5)
	if err != nil {
		t.Fatalf("Expected success after retries: %v", err)
	}

	// Check that backoff increases exponentially
	durations := mockPub.BackoffDurations()

	// Should have 3 backoff durations (between 4 attempts)
	if len(durations) != 3 {
		t.Fatalf("Expected 3 backoff periods, got %d", len(durations))
	}

	// Verify exponential increase with tolerance for timing jitter
	// Expected: 100ms, 500ms, 1s
	expectedBackoffs := []time.Duration{
		100 * time.Millisecond,
		500 * time.Millisecond,
		1 * time.Second,
	}
	tolerance := 100 * time.Millisecond // More generous tolerance for CI environments

	for i, expected := range expectedBackoffs {
		actual := durations[i]
		minDuration := expected - tolerance
		maxDuration := expected + tolerance

		if actual < minDuration || actual > maxDuration {
			t.Errorf("Backoff[%d] = %v, want ~%v (range: %v-%v)", i, actual, expected, minDuration, maxDuration)
		}
	}

	// Verify each backoff is greater than the previous one
	for i := 1; i < len(durations); i++ {
		if durations[i] <= durations[i-1] {
			t.Errorf("Backoff should increase: backoff[%d]=%v is not > backoff[%d]=%v",
				i, durations[i], i-1, durations[i-1])
		}
	}
}

func TestPublishWithRetry_NonRetryableErrors(t *testing.T) {
	// Initialize metrics once for all subtests
	reg := prometheus.NewRegistry()
	if err := metrics.InitMetrics(reg); err != nil {
		t.Fatalf("failed to initialize metrics: %v", err)
	}

	tests := []struct {
		name          string
		errorType     string
		wantRetryable bool
		maxRetries    int
		wantAttempts  int
	}{
		{
			name:          "connection error is retryable",
			errorType:     "connection",
			wantRetryable: true,
			maxRetries:    3,
			wantAttempts:  4, // Will keep retrying until max
		},
		{
			name:          "publish error is retryable",
			errorType:     "publish",
			wantRetryable: true,
			maxRetries:    3,
			wantAttempts:  4, // Will keep retrying until max
		},
		{
			name:          "rate limit error is retryable",
			errorType:     "rate_limit",
			wantRetryable: true,
			maxRetries:    3,
			wantAttempts:  4, // Will keep retrying until max
		},
		{
			name:          "auth error is not retryable",
			errorType:     "auth",
			wantRetryable: false,
			maxRetries:    3,
			wantAttempts:  1, // Should fail immediately without retry
		},
		{
			name:          "validation error is not retryable",
			errorType:     "validation",
			wantRetryable: false,
			maxRetries:    3,
			wantAttempts:  1, // Should fail immediately without retry
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockPub := NewMockPublisherWithErrorType(tt.errorType)

			handler := &Handler{
				publisher: mockPub,
			}

			ctx := context.Background()
			testData := map[string]string{"test": "data"}
			testAttrs := map[string]string{"event_type": "build.started"}

			_, err := handler.publishWithRetry(ctx, testData, testAttrs, tt.maxRetries)

			if err == nil {
				t.Fatal("Expected error but got success")
			}

			// Check attempt count matches expectation
			actualAttempts := mockPub.AttemptCount()
			if actualAttempts != tt.wantAttempts {
				t.Errorf("AttemptCount() = %d, want %d (retryable=%v)", actualAttempts, tt.wantAttempts, tt.wantRetryable)
			}

			// Verify error is of correct type
			switch tt.errorType {
			case "auth":
				if !errors.IsAuthError(err) {
					t.Errorf("Expected auth error, got: %v", err)
				}
			case "validation":
				if !errors.IsValidationError(err) {
					t.Errorf("Expected validation error, got: %v", err)
				}
			case "connection":
				if !errors.IsConnectionError(err) {
					t.Errorf("Expected connection error, got: %v", err)
				}
			case "publish":
				if !errors.IsPublishError(err) {
					t.Errorf("Expected publish error, got: %v", err)
				}
			case "rate_limit":
				if !errors.IsRateLimitError(err) {
					t.Errorf("Expected rate limit error, got: %v", err)
				}
			}
		})
	}
}

func TestPublishWithRetry_RetryMetricsRecorded(t *testing.T) {
	// Setup metrics registry
	reg := prometheus.NewRegistry()
	if err := metrics.InitMetrics(reg); err != nil {
		t.Fatalf("failed to initialize metrics: %v", err)
	}

	mockPub := NewMockPublisherWithRetryableErrors(2) // Will fail twice, then succeed

	handler := &Handler{
		publisher: mockPub,
	}

	ctx := context.Background()
	testData := map[string]string{"test": "data"}
	testAttrs := map[string]string{"event_type": "build.started"}

	_, err := handler.publishWithRetry(ctx, testData, testAttrs, 3)
	if err != nil {
		t.Fatalf("Expected success after retries: %v", err)
	}

	// Verify retry metric was incremented 2 times (for 2 retries before success)
	metricFamilies, err := reg.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	var retryCount float64
	found := false
	for _, mf := range metricFamilies {
		if mf.GetName() == "buildkite_pubsub_retries_total" {
			for _, m := range mf.GetMetric() {
				// Find the metric with event_type="build.started"
				for _, label := range m.GetLabel() {
					if label.GetName() == "event_type" && label.GetValue() == "build.started" {
						retryCount = m.GetCounter().GetValue()
						found = true
						break
					}
				}
			}
		}
	}

	if !found {
		t.Error("Retry metric not found")
	}

	if retryCount != 2 {
		t.Errorf("Retry metric count = %f, want 2", retryCount)
	}
}

func TestPublishWithRetry_ContextCancellation(t *testing.T) {
	// Initialize metrics
	reg := prometheus.NewRegistry()
	if err := metrics.InitMetrics(reg); err != nil {
		t.Fatalf("failed to initialize metrics: %v", err)
	}

	mockPub := NewMockPublisherWithRetryableErrors(10) // Will keep failing

	handler := &Handler{
		publisher: mockPub,
	}

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	testData := map[string]string{"test": "data"}
	testAttrs := map[string]string{"event_type": "build.started"}

	start := time.Now()
	_, err := handler.publishWithRetry(ctx, testData, testAttrs, 5)
	elapsed := time.Since(start)

	// Should fail due to context cancellation
	if err == nil {
		t.Error("Expected error due to context cancellation")
	}

	// Should not wait for all retries
	if elapsed > 500*time.Millisecond {
		t.Errorf("Took too long: %v (context should have cancelled earlier)", elapsed)
	}

	// Should have attempted less than max retries due to context cancellation
	attemptCount := mockPub.AttemptCount()
	if attemptCount > 3 {
		t.Errorf("Too many attempts despite context cancellation: %d", attemptCount)
	}
}

func TestPublishWithRetry_ContextCancelledBetweenRetries(t *testing.T) {
	// Initialize metrics
	reg := prometheus.NewRegistry()
	if err := metrics.InitMetrics(reg); err != nil {
		t.Fatalf("failed to initialize metrics: %v", err)
	}

	mockPub := NewMockPublisherWithRetryableErrors(5) // Will keep failing

	handler := &Handler{
		publisher: mockPub,
	}

	// Create context that we'll cancel manually
	ctx, cancel := context.WithCancel(context.Background())

	testData := map[string]string{"test": "data"}
	testAttrs := map[string]string{"event_type": "build.started"}

	// Cancel context after first retry attempt
	go func() {
		time.Sleep(150 * time.Millisecond) // After first backoff
		cancel()
	}()

	_, err := handler.publishWithRetry(ctx, testData, testAttrs, 5)

	// Should fail due to context cancellation
	if err == nil {
		t.Error("Expected error due to context cancellation")
	}

	// Should have attempted 2-3 times max (initial + maybe 1-2 retries before cancel)
	attemptCount := mockPub.AttemptCount()
	if attemptCount > 3 {
		t.Errorf("Too many attempts despite context cancellation: %d", attemptCount)
	}
}

func TestPublishWithRetry_ConcurrentCalls(t *testing.T) {
	// Initialize metrics
	reg := prometheus.NewRegistry()
	if err := metrics.InitMetrics(reg); err != nil {
		t.Fatalf("failed to initialize metrics: %v", err)
	}

	// Verify that concurrent calls to publishWithRetry are safe
	mockPub := NewMockPublisherWithRetryableErrors(1) // Fail once, then succeed

	handler := &Handler{
		publisher: mockPub,
	}

	ctx := context.Background()
	numGoroutines := 10

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			testData := map[string]string{"test": fmt.Sprintf("data-%d", id)}
			testAttrs := map[string]string{"event_type": "build.started"}
			_, err := handler.publishWithRetry(ctx, testData, testAttrs, 3)
			errors <- err
		}(i)
	}

	wg.Wait()
	close(errors)

	// All should eventually succeed
	errorCount := 0
	for err := range errors {
		if err != nil {
			errorCount++
			t.Errorf("Concurrent call failed: %v", err)
		}
	}

	if errorCount > 0 {
		t.Errorf("Expected all concurrent calls to succeed, but %d failed", errorCount)
	}
}

func TestPublishWithRetry_ZeroMaxRetries(t *testing.T) {
	// Initialize metrics
	reg := prometheus.NewRegistry()
	if err := metrics.InitMetrics(reg); err != nil {
		t.Fatalf("failed to initialize metrics: %v", err)
	}

	mockPub := NewMockPublisherWithRetryableErrors(1) // Will fail once

	handler := &Handler{
		publisher: mockPub,
	}

	ctx := context.Background()
	testData := map[string]string{"test": "data"}
	testAttrs := map[string]string{"event_type": "build.started"}

	_, err := handler.publishWithRetry(ctx, testData, testAttrs, 0)

	// Should fail immediately with no retries
	if err == nil {
		t.Error("Expected error with zero retries")
	}

	// Should only attempt once
	if mockPub.AttemptCount() != 1 {
		t.Errorf("AttemptCount() = %d, want 1", mockPub.AttemptCount())
	}
}

func TestPublishWithRetry_LargeMaxRetries(t *testing.T) {
	// Initialize metrics
	reg := prometheus.NewRegistry()
	if err := metrics.InitMetrics(reg); err != nil {
		t.Fatalf("failed to initialize metrics: %v", err)
	}

	mockPub := NewMockPublisherWithRetryableErrors(2) // Fail twice

	handler := &Handler{
		publisher: mockPub,
	}

	ctx := context.Background()
	testData := map[string]string{"test": "data"}
	testAttrs := map[string]string{"event_type": "build.started"}

	// Large max retries, but should succeed on 3rd attempt
	_, err := handler.publishWithRetry(ctx, testData, testAttrs, 100)
	if err != nil {
		t.Errorf("Expected success with large max retries: %v", err)
	}

	// Should only attempt 3 times (not all 100 retries)
	if mockPub.AttemptCount() != 3 {
		t.Errorf("AttemptCount() = %d, want 3", mockPub.AttemptCount())
	}
}
