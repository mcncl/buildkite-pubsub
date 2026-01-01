package publisher

import (
	"context"
	"errors"
	"testing"
)

func TestMockPublisher_NewMockPublisher(t *testing.T) {
	pub := NewMockPublisher()
	if pub == nil {
		t.Fatal("NewMockPublisher() returned nil")
	}

	mock, ok := pub.(*MockPublisher)
	if !ok {
		t.Fatal("NewMockPublisher() did not return *MockPublisher")
	}

	if mock.TopicID() != "mock-topic" {
		t.Errorf("TopicID() = %v, want %v", mock.TopicID(), "mock-topic")
	}
}

func TestMockPublisher_Publish(t *testing.T) {
	pub := NewMockPublisher()
	mock := pub.(*MockPublisher)

	ctx := context.Background()
	testData := map[string]string{"key": "value"}
	attrs := map[string]string{"attr": "val"}

	msgID, err := pub.Publish(ctx, testData, attrs)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if msgID != "mock-message-id" {
		t.Errorf("Publish() msgID = %v, want %v", msgID, "mock-message-id")
	}

	// Verify message was recorded
	published := mock.GetPublished()
	if len(published) != 1 {
		t.Fatalf("GetPublished() len = %d, want 1", len(published))
	}

	// Verify data was stored correctly
	if published[0].Data == nil {
		t.Error("Published data is nil")
	}
	if published[0].Attributes["attr"] != "val" {
		t.Errorf("Published attributes = %v, want attr=val", published[0].Attributes)
	}
}

func TestMockPublisher_Publish_WithError(t *testing.T) {
	pub := NewMockPublisher()
	mock := pub.(*MockPublisher)

	expectedErr := errors.New("publish failed")
	mock.SetError(expectedErr)

	ctx := context.Background()
	testData := map[string]string{"key": "value"}

	_, err := pub.Publish(ctx, testData, nil)
	if err == nil {
		t.Fatal("Publish() expected error, got nil")
	}
	if err != expectedErr {
		t.Errorf("Publish() error = %v, want %v", err, expectedErr)
	}

	// Verify no message was recorded when error occurred
	if len(mock.GetPublished()) != 0 {
		t.Error("GetPublished() should be empty when error occurs")
	}
}

func TestMockPublisher_Publish_Multiple(t *testing.T) {
	pub := NewMockPublisher()
	mock := pub.(*MockPublisher)

	ctx := context.Background()

	// Publish multiple messages
	for i := 0; i < 5; i++ {
		_, err := pub.Publish(ctx, map[string]int{"index": i}, nil)
		if err != nil {
			t.Fatalf("Publish() error = %v", err)
		}
	}

	published := mock.GetPublished()
	if len(published) != 5 {
		t.Errorf("GetPublished() len = %d, want 5", len(published))
	}
}

func TestMockPublisher_LastPublished(t *testing.T) {
	pub := NewMockPublisher()
	mock := pub.(*MockPublisher)

	// Test empty case
	if mock.LastPublished() != nil {
		t.Error("LastPublished() should be nil when no messages published")
	}

	ctx := context.Background()

	// Publish some messages
	for i := 0; i < 3; i++ {
		_, err := pub.Publish(ctx, map[string]int{"index": i}, nil)
		if err != nil {
			t.Fatalf("Publish() error = %v", err)
		}
	}

	last := mock.LastPublished()
	if last == nil {
		t.Fatal("LastPublished() returned nil after publishing")
	}

	// Verify it's the last message
	data, ok := last.Data.(map[string]int)
	if !ok {
		t.Fatalf("LastPublished().Data type = %T, want map[string]int", last.Data)
	}
	if data["index"] != 2 {
		t.Errorf("LastPublished().Data[index] = %v, want 2", data["index"])
	}
}

func TestMockPublisher_Reset(t *testing.T) {
	pub := NewMockPublisher()
	mock := pub.(*MockPublisher)

	ctx := context.Background()

	// Set error and publish messages
	mock.SetError(errors.New("test error"))
	pub.Publish(ctx, "test1", nil) // This will fail
	mock.Error = nil               // Clear error manually
	pub.Publish(ctx, "test2", nil) // This will succeed

	if len(mock.GetPublished()) != 1 {
		t.Fatalf("Expected 1 published message, got %d", len(mock.GetPublished()))
	}

	// Reset
	mock.Reset()

	// Verify everything is cleared
	if len(mock.GetPublished()) != 0 {
		t.Error("Reset() should clear published messages")
	}
	if mock.Error != nil {
		t.Error("Reset() should clear error")
	}

	// Verify can publish again
	msgID, err := pub.Publish(ctx, "after reset", nil)
	if err != nil {
		t.Fatalf("Publish() after Reset() error = %v", err)
	}
	if msgID != "mock-message-id" {
		t.Errorf("Publish() msgID = %v, want mock-message-id", msgID)
	}
}

func TestMockPublisher_Close(t *testing.T) {
	pub := NewMockPublisher()

	err := pub.Close()
	if err != nil {
		t.Errorf("Close() error = %v, want nil", err)
	}
}

func TestMockPublisher_TopicID(t *testing.T) {
	pub := NewMockPublisher()
	mock := pub.(*MockPublisher)

	if mock.TopicID() != "mock-topic" {
		t.Errorf("TopicID() = %v, want mock-topic", mock.TopicID())
	}
}

func TestMockPublisher_SetError(t *testing.T) {
	pub := NewMockPublisher()
	mock := pub.(*MockPublisher)

	// Initially no error
	if mock.Error != nil {
		t.Error("Initial error should be nil")
	}

	// Set error
	testErr := errors.New("test error")
	mock.SetError(testErr)

	if mock.Error != testErr {
		t.Errorf("Error = %v, want %v", mock.Error, testErr)
	}

	// Set nil error
	mock.SetError(nil)
	if mock.Error != nil {
		t.Error("SetError(nil) should set error to nil")
	}
}

func TestMockPublisher_Publish_NilAttributes(t *testing.T) {
	pub := NewMockPublisher()
	mock := pub.(*MockPublisher)

	ctx := context.Background()
	testData := "test data"

	_, err := pub.Publish(ctx, testData, nil)
	if err != nil {
		t.Fatalf("Publish() with nil attributes error = %v", err)
	}

	last := mock.LastPublished()
	if last == nil {
		t.Fatal("LastPublished() returned nil")
	}
	if last.Attributes != nil {
		t.Errorf("Attributes = %v, want nil", last.Attributes)
	}
}

func TestMockPublisher_Publish_EmptyAttributes(t *testing.T) {
	pub := NewMockPublisher()
	mock := pub.(*MockPublisher)

	ctx := context.Background()
	testData := "test data"
	emptyAttrs := map[string]string{}

	_, err := pub.Publish(ctx, testData, emptyAttrs)
	if err != nil {
		t.Fatalf("Publish() with empty attributes error = %v", err)
	}

	last := mock.LastPublished()
	if last == nil {
		t.Fatal("LastPublished() returned nil")
	}
	if len(last.Attributes) != 0 {
		t.Errorf("Attributes len = %d, want 0", len(last.Attributes))
	}
}

func TestMockPublisher_Publish_VariousDataTypes(t *testing.T) {
	pub := NewMockPublisher()

	ctx := context.Background()

	tests := []struct {
		name string
		data interface{}
	}{
		{"string", "hello"},
		{"int", 42},
		{"float", 3.14},
		{"bool", true},
		{"slice", []string{"a", "b", "c"}},
		{"map", map[string]int{"one": 1, "two": 2}},
		{"struct", struct{ Name string }{"test"}},
		{"nil", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := pub.Publish(ctx, tt.data, nil)
			if err != nil {
				t.Errorf("Publish(%v) error = %v", tt.name, err)
			}
		})
	}
}

func TestMockPublisher_ImplementsPublisherInterface(t *testing.T) {
	// This test verifies at compile time that MockPublisher implements Publisher
	var _ Publisher = NewMockPublisher()
	var _ Publisher = &MockPublisher{}
}
