package webhook

import (
	"context"
	"sync"
	"testing"

	"github.com/mcncl/buildkite-pubsub/internal/errors"
	"github.com/mcncl/buildkite-pubsub/internal/metrics"
	"github.com/mcncl/buildkite-pubsub/internal/publisher"
	"github.com/prometheus/client_golang/prometheus"
)

// MockDLQPublisher tracks messages sent to the DLQ
type MockDLQPublisher struct {
	mu       sync.Mutex
	messages []dlqMessage
	err      error
}

type dlqMessage struct {
	data       interface{}
	attributes map[string]string
}

func NewMockDLQPublisher() *MockDLQPublisher {
	return &MockDLQPublisher{
		messages: make([]dlqMessage, 0),
	}
}

func (m *MockDLQPublisher) Publish(ctx context.Context, data interface{}, attributes map[string]string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.err != nil {
		return "", m.err
	}

	m.messages = append(m.messages, dlqMessage{
		data:       data,
		attributes: attributes,
	})

	return "dlq-message-id", nil
}

func (m *MockDLQPublisher) Close() error {
	return nil
}

func (m *MockDLQPublisher) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
}

func (m *MockDLQPublisher) MessageCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.messages)
}

func (m *MockDLQPublisher) LastMessage() *dlqMessage {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.messages) == 0 {
		return nil
	}
	return &m.messages[len(m.messages)-1]
}

func (m *MockDLQPublisher) Messages() []dlqMessage {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]dlqMessage, len(m.messages))
	copy(result, m.messages)
	return result
}

func TestPublishWithRetry_DLQ_EnabledAndTriggered(t *testing.T) {
	// Initialize metrics
	reg := prometheus.NewRegistry()
	if err := metrics.InitMetrics(reg); err != nil {
		t.Fatalf("failed to initialize metrics: %v", err)
	}

	// Main publisher that always fails
	mainPub := NewMockPublisherWithRetryableErrors(100) // Always fails
	dlqPub := NewMockDLQPublisher()

	handler := &Handler{
		publisher:    mainPub,
		dlqPublisher: dlqPub,
		enableDLQ:    true,
	}

	ctx := context.Background()
	testData := map[string]string{"test": "data", "build_id": "123"}
	testAttrs := map[string]string{"event_type": "build.started", "pipeline": "test-pipeline"}

	// Should fail after max retries
	_, err := handler.publishWithRetry(ctx, testData, testAttrs, 2)
	if err == nil {
		t.Error("Expected error after retries exhausted")
	}

	// DLQ should have received the message
	if dlqPub.MessageCount() != 1 {
		t.Errorf("DLQ message count = %d, want 1", dlqPub.MessageCount())
	}

	// Verify DLQ message attributes
	dlqMsg := dlqPub.LastMessage()
	if dlqMsg == nil {
		t.Fatal("DLQ message is nil")
	}

	// Check DLQ-specific attributes are present
	if dlqMsg.attributes["dlq_reason"] == "" {
		t.Error("DLQ message missing dlq_reason attribute")
	}
	if dlqMsg.attributes["dlq_retry_count"] == "" {
		t.Error("DLQ message missing dlq_retry_count attribute")
	}
	if dlqMsg.attributes["dlq_original_timestamp"] == "" {
		t.Error("DLQ message missing dlq_original_timestamp attribute")
	}
	if dlqMsg.attributes["dlq_error_message"] == "" {
		t.Error("DLQ message missing dlq_error_message attribute")
	}

	// Check original attributes are preserved
	if dlqMsg.attributes["event_type"] != "build.started" {
		t.Errorf("DLQ message event_type = %s, want build.started", dlqMsg.attributes["event_type"])
	}
	if dlqMsg.attributes["pipeline"] != "test-pipeline" {
		t.Errorf("DLQ message pipeline = %s, want test-pipeline", dlqMsg.attributes["pipeline"])
	}
}

func TestPublishWithRetry_DLQ_Disabled(t *testing.T) {
	// Initialize metrics
	reg := prometheus.NewRegistry()
	if err := metrics.InitMetrics(reg); err != nil {
		t.Fatalf("failed to initialize metrics: %v", err)
	}

	// Main publisher that always fails
	mainPub := NewMockPublisherWithRetryableErrors(100)
	dlqPub := NewMockDLQPublisher()

	handler := &Handler{
		publisher:    mainPub,
		dlqPublisher: dlqPub,
		enableDLQ:    false, // DLQ disabled
	}

	ctx := context.Background()
	testData := map[string]string{"test": "data"}
	testAttrs := map[string]string{"event_type": "build.started"}

	_, err := handler.publishWithRetry(ctx, testData, testAttrs, 2)
	if err == nil {
		t.Error("Expected error after retries exhausted")
	}

	// DLQ should NOT have received any messages
	if dlqPub.MessageCount() != 0 {
		t.Errorf("DLQ message count = %d, want 0 (DLQ disabled)", dlqPub.MessageCount())
	}
}

func TestPublishWithRetry_DLQ_NilPublisher(t *testing.T) {
	// Initialize metrics
	reg := prometheus.NewRegistry()
	if err := metrics.InitMetrics(reg); err != nil {
		t.Fatalf("failed to initialize metrics: %v", err)
	}

	// Main publisher that always fails
	mainPub := NewMockPublisherWithRetryableErrors(100)

	handler := &Handler{
		publisher:    mainPub,
		dlqPublisher: nil, // No DLQ publisher
		enableDLQ:    true,
	}

	ctx := context.Background()
	testData := map[string]string{"test": "data"}
	testAttrs := map[string]string{"event_type": "build.started"}

	// Should not panic even with nil DLQ publisher
	_, err := handler.publishWithRetry(ctx, testData, testAttrs, 2)
	if err == nil {
		t.Error("Expected error after retries exhausted")
	}
}

func TestPublishWithRetry_DLQ_PublishError(t *testing.T) {
	// Initialize metrics
	reg := prometheus.NewRegistry()
	if err := metrics.InitMetrics(reg); err != nil {
		t.Fatalf("failed to initialize metrics: %v", err)
	}

	// Main publisher that always fails
	mainPub := NewMockPublisherWithRetryableErrors(100)
	dlqPub := NewMockDLQPublisher()
	dlqPub.SetError(errors.NewConnectionError("DLQ connection failed"))

	handler := &Handler{
		publisher:    mainPub,
		dlqPublisher: dlqPub,
		enableDLQ:    true,
	}

	ctx := context.Background()
	testData := map[string]string{"test": "data"}
	testAttrs := map[string]string{"event_type": "build.started"}

	// Should not fail due to DLQ error (DLQ is best-effort)
	_, err := handler.publishWithRetry(ctx, testData, testAttrs, 2)
	if err == nil {
		t.Error("Expected original error (not DLQ error)")
	}

	// Original error should be returned
	if !errors.IsConnectionError(err) {
		t.Errorf("Expected connection error from main publisher, got: %v", err)
	}
}

func TestPublishWithRetry_DLQ_SuccessNoTrigger(t *testing.T) {
	// Initialize metrics
	reg := prometheus.NewRegistry()
	if err := metrics.InitMetrics(reg); err != nil {
		t.Fatalf("failed to initialize metrics: %v", err)
	}

	// Main publisher that succeeds
	mainPub := publisher.NewMockPublisher()
	dlqPub := NewMockDLQPublisher()

	handler := &Handler{
		publisher:    mainPub,
		dlqPublisher: dlqPub,
		enableDLQ:    true,
	}

	ctx := context.Background()
	testData := map[string]string{"test": "data"}
	testAttrs := map[string]string{"event_type": "build.started"}

	// Should succeed
	_, err := handler.publishWithRetry(ctx, testData, testAttrs, 3)
	if err != nil {
		t.Errorf("Expected success, got error: %v", err)
	}

	// DLQ should NOT have received any messages (main publish succeeded)
	if dlqPub.MessageCount() != 0 {
		t.Errorf("DLQ message count = %d, want 0 (publish succeeded)", dlqPub.MessageCount())
	}
}

func TestPublishWithRetry_DLQ_NonRetryableError(t *testing.T) {
	// Initialize metrics
	reg := prometheus.NewRegistry()
	if err := metrics.InitMetrics(reg); err != nil {
		t.Fatalf("failed to initialize metrics: %v", err)
	}

	// Main publisher that returns non-retryable error
	mainPub := NewMockPublisherWithErrorType("auth") // Auth errors are non-retryable
	dlqPub := NewMockDLQPublisher()

	handler := &Handler{
		publisher:    mainPub,
		dlqPublisher: dlqPub,
		enableDLQ:    true,
	}

	ctx := context.Background()
	testData := map[string]string{"test": "data"}
	testAttrs := map[string]string{"event_type": "build.started"}

	// Should fail immediately (no retries for auth errors)
	_, err := handler.publishWithRetry(ctx, testData, testAttrs, 3)
	if err == nil {
		t.Error("Expected auth error")
	}

	// DLQ should NOT have received messages (non-retryable errors don't go to DLQ)
	if dlqPub.MessageCount() != 0 {
		t.Errorf("DLQ message count = %d, want 0 (non-retryable error)", dlqPub.MessageCount())
	}
}

func TestClassifyFailureReason(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "connection error",
			err:      errors.NewConnectionError("connection failed"),
			expected: "connection_error",
		},
		{
			name:     "rate limit error",
			err:      errors.NewRateLimitError("rate limited"),
			expected: "rate_limit",
		},
		{
			name:     "publish error",
			err:      errors.NewPublishError("publish failed", nil),
			expected: "publish_error",
		},
		{
			name:     "unknown error",
			err:      errors.NewInternalError("internal error"),
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyFailureReason(tt.err)
			if result != tt.expected {
				t.Errorf("classifyFailureReason() = %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestSendToDLQ_DirectCall(t *testing.T) {
	// Initialize metrics
	reg := prometheus.NewRegistry()
	if err := metrics.InitMetrics(reg); err != nil {
		t.Fatalf("failed to initialize metrics: %v", err)
	}

	dlqPub := NewMockDLQPublisher()

	handler := &Handler{
		dlqPublisher: dlqPub,
		enableDLQ:    true,
	}

	ctx := context.Background()
	testData := map[string]string{"message": "test payload"}
	attrs := map[string]string{
		"event_type": "build.finished",
		"pipeline":   "my-pipeline",
		"branch":     "main",
	}
	testErr := errors.NewConnectionError("test failure")

	handler.sendToDLQ(ctx, testData, attrs, testErr, 3)

	// Verify message was sent
	if dlqPub.MessageCount() != 1 {
		t.Fatalf("DLQ message count = %d, want 1", dlqPub.MessageCount())
	}

	msg := dlqPub.LastMessage()

	// Verify attributes
	if msg.attributes["dlq_reason"] != "connection_error" {
		t.Errorf("dlq_reason = %s, want connection_error", msg.attributes["dlq_reason"])
	}
	if msg.attributes["dlq_retry_count"] != "3" {
		t.Errorf("dlq_retry_count = %s, want 3", msg.attributes["dlq_retry_count"])
	}
	if msg.attributes["event_type"] != "build.finished" {
		t.Errorf("event_type = %s, want build.finished", msg.attributes["event_type"])
	}

	// Verify DLQ message wraps original payload
	msgData, ok := msg.data.(map[string]interface{})
	if !ok {
		t.Fatal("DLQ message data is not a map")
	}
	if _, exists := msgData["original_payload"]; !exists {
		t.Error("DLQ message missing original_payload")
	}
	if _, exists := msgData["dlq_metadata"]; !exists {
		t.Error("DLQ message missing dlq_metadata")
	}
}
