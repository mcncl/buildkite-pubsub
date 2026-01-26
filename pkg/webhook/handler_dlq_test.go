package webhook

import (
	"context"
	"sync"
	"testing"

	"github.com/mcncl/buildkite-pubsub/internal/errors"
	"github.com/mcncl/buildkite-pubsub/internal/metrics"
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

func TestSendToDLQ_Enabled(t *testing.T) {
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

	handler.sendToDLQ(ctx, testData, attrs, testErr)

	if dlqPub.MessageCount() != 1 {
		t.Fatalf("DLQ message count = %d, want 1", dlqPub.MessageCount())
	}

	msg := dlqPub.LastMessage()

	if msg.attributes["dlq_reason"] != "connection_error" {
		t.Errorf("dlq_reason = %s, want connection_error", msg.attributes["dlq_reason"])
	}
	if msg.attributes["event_type"] != "build.finished" {
		t.Errorf("event_type = %s, want build.finished", msg.attributes["event_type"])
	}

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

func TestSendToDLQ_Disabled(t *testing.T) {
	dlqPub := NewMockDLQPublisher()

	handler := &Handler{
		dlqPublisher: dlqPub,
		enableDLQ:    false,
	}

	ctx := context.Background()
	testData := map[string]string{"test": "data"}
	attrs := map[string]string{"event_type": "build.started"}
	testErr := errors.NewConnectionError("test failure")

	handler.sendToDLQ(ctx, testData, attrs, testErr)

	if dlqPub.MessageCount() != 0 {
		t.Errorf("DLQ message count = %d, want 0 (DLQ disabled)", dlqPub.MessageCount())
	}
}

func TestSendToDLQ_NilPublisher(t *testing.T) {
	handler := &Handler{
		dlqPublisher: nil,
		enableDLQ:    true,
	}

	ctx := context.Background()
	testData := map[string]string{"test": "data"}
	attrs := map[string]string{"event_type": "build.started"}
	testErr := errors.NewConnectionError("test failure")

	// Should not panic
	handler.sendToDLQ(ctx, testData, attrs, testErr)
}

func TestSendToDLQ_PublishError(t *testing.T) {
	reg := prometheus.NewRegistry()
	if err := metrics.InitMetrics(reg); err != nil {
		t.Fatalf("failed to initialize metrics: %v", err)
	}

	dlqPub := NewMockDLQPublisher()
	dlqPub.SetError(errors.NewConnectionError("DLQ connection failed"))

	handler := &Handler{
		dlqPublisher: dlqPub,
		enableDLQ:    true,
	}

	ctx := context.Background()
	testData := map[string]string{"test": "data"}
	attrs := map[string]string{"event_type": "build.started"}
	testErr := errors.NewConnectionError("original failure")

	// Should not panic even when DLQ publish fails
	handler.sendToDLQ(ctx, testData, attrs, testErr)
}
