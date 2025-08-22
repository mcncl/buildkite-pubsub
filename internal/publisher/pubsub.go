package publisher

import (
	"context"
	"encoding/json"
	"fmt"

	"cloud.google.com/go/pubsub/v2"
	"cloud.google.com/go/pubsub/v2/apiv1/pubsubpb"
)

// Publisher defines the interface for publishing messages
type Publisher interface {
	Publish(ctx context.Context, data interface{}, attributes map[string]string) (string, error)
	Close() error
}

// PubSubPublisher implements the Publisher interface for Google Cloud Pub/Sub
type PubSubPublisher struct {
	client    *pubsub.Client
	publisher *pubsub.Publisher
	topicID   string
	projectID string
}

// NewPubSubPublisher creates a new Google Cloud Pub/Sub publisher
func NewPubSubPublisher(ctx context.Context, projectID, topicID string) (*PubSubPublisher, error) {
	return NewPubSubPublisherWithSettings(ctx, projectID, topicID, nil)
}

// NewPubSubPublisherWithSettings creates a new Google Cloud Pub/Sub publisher with custom settings
func NewPubSubPublisherWithSettings(ctx context.Context, projectID, topicID string, settings *pubsub.PublishSettings) (*PubSubPublisher, error) {
	// Create the client
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to create pubsub client: %w", err)
	}

	// Check if topic exists using admin client from the client
	topicPath := fmt.Sprintf("projects/%s/topics/%s", projectID, topicID)
	_, err = client.TopicAdminClient.GetTopic(ctx, &pubsubpb.GetTopicRequest{
		Topic: topicPath,
	})
	if err != nil {
		return nil, fmt.Errorf("topic %s does not exist or cannot be accessed: %w", topicID, err)
	}

	// Create publisher with settings
	publisher := client.Publisher(topicID)

	// Apply optimized settings if not provided
	if settings == nil {
		settings = &pubsub.PublishSettings{
			// Batch settings for better throughput
			CountThreshold: 100,  // Send batch when 100 messages accumulated
			ByteThreshold:  1e6,  // Send batch when 1MB accumulated
			DelayThreshold: 10e6, // Send batch after 10ms

			// Connection pool for better concurrency
			NumGoroutines: 4, // Use 4 goroutines for publishing

			// Flow control to prevent overwhelming the service
			FlowControlSettings: pubsub.FlowControlSettings{
				MaxOutstandingMessages: 1000, // Max 1000 messages in flight
				MaxOutstandingBytes:    1e9,  // Max 1GB in flight
				LimitExceededBehavior:  pubsub.FlowControlBlock,
			},

			// Enable compression for better network utilization
			EnableCompression:         true,
			CompressionBytesThreshold: 1000, // Compress messages > 1KB
		}
	}

	publisher.PublishSettings = *settings

	return &PubSubPublisher{
		client:    client,
		publisher: publisher,
		topicID:   topicID,
		projectID: projectID,
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

	// Use non-blocking publish for better performance
	result := p.publisher.Publish(ctx, msg)

	// Get will block until the message is sent or ctx is cancelled
	msgID, err := result.Get(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to publish message: %w", err)
	}

	return msgID, nil
}

// PublishAsync publishes a message asynchronously without waiting for confirmation
func (p *PubSubPublisher) PublishAsync(ctx context.Context, data interface{}, attributes map[string]string) *pubsub.PublishResult {
	jsonData, _ := json.Marshal(data)

	msg := &pubsub.Message{
		Data:       jsonData,
		Attributes: attributes,
	}

	return p.publisher.Publish(ctx, msg)
}

// Close closes the publisher and its connections
func (p *PubSubPublisher) Close() error {
	// Stop accepting new messages and flush pending ones
	p.publisher.Stop()
	return p.client.Close()
}

// Flush waits for all pending messages to be sent
func (p *PubSubPublisher) Flush() {
	p.publisher.Flush()
}
