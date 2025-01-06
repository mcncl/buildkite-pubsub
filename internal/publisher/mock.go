package publisher

import (
	"context"
)

// MockPublisher provides a mock implementation of the Publisher interface for testing
type MockPublisher struct {
	published []publishedMessage
	Error     error
	topicID   string
}

type publishedMessage struct {
	Data       interface{}
	Attributes map[string]string
}

// NewMockPublisher creates a new MockPublisher
func NewMockPublisher() Publisher {
	return &MockPublisher{
		published: make([]publishedMessage, 0),
		topicID:   "mock-topic",
	}
}

func (m *MockPublisher) TopicID() string {
	return m.topicID
}

// Publish records the published message and returns a mock message ID
func (m *MockPublisher) Publish(ctx context.Context, data interface{}, attributes map[string]string) (string, error) {
	if m.Error != nil {
		return "", m.Error
	}

	m.published = append(m.published, publishedMessage{
		Data:       data,
		Attributes: attributes,
	})

	return "mock-message-id", nil
}

// Close implements the Publisher interface
func (m *MockPublisher) Close() error {
	return nil
}

// GetPublished returns all published messages
func (m *MockPublisher) GetPublished() []publishedMessage {
	return m.published
}

// LastPublished returns the last published message or nil if none exists
func (m *MockPublisher) LastPublished() *publishedMessage {
	if len(m.published) == 0 {
		return nil
	}
	return &m.published[len(m.published)-1]
}

// Reset clears all published messages and errors
func (m *MockPublisher) Reset() {
	m.published = m.published[:0]
	m.Error = nil
}

// SetError sets an error to be returned by the next Publish call
func (m *MockPublisher) SetError(err error) {
	m.Error = err
}
