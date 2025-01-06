package publisher

import (
	"context"
	"encoding/json"
	"fmt"

	"cloud.google.com/go/pubsub"
)

// Publisher defines the interface for publishing messages
type Publisher interface {
	Publish(ctx context.Context, data interface{}, attributes map[string]string) (string, error)
	Close() error
}

// PubSubPublisher implements the Publisher interface for Google Cloud Pub/Sub
type PubSubPublisher struct {
	client  *pubsub.Client
	topic   *pubsub.Topic
	topicID string
}

// NewPubSubPublisher creates a new Google Cloud Pub/Sub publisher
func NewPubSubPublisher(ctx context.Context, projectID, topicID string) (*PubSubPublisher, error) {
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to create pubsub client: %w", err)
	}

	topic := client.Topic(topicID)

	// Check if topic exists
	exists, err := topic.Exists(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check topic existence: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("topic %s does not exist", topicID)
	}

	return &PubSubPublisher{
		client:  client,
		topic:   topic,
		topicID: topicID,
	}, nil
}

func (p *PubSubPublisher) TopicID() string {
	return p.topicID
}

// Publish publishes a message to Pub/Sub
func (p *PubSubPublisher) Publish(ctx context.Context, data interface{}, attributes map[string]string) (string, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal data: %w", err)
	}

	msg := &pubsub.Message{
		Data:       jsonData,
		Attributes: attributes,
	}

	result := p.topic.Publish(ctx, msg)
	msgID, err := result.Get(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to publish message: %w", err)
	}

	return msgID, nil
}

// Close closes the publisher and its connections
func (p *PubSubPublisher) Close() error {
	p.topic.Stop()
	return p.client.Close()
}
