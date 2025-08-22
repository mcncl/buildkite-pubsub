package publisher

import (
	"context"
	"testing"

	"cloud.google.com/go/pubsub/pstest"
	"cloud.google.com/go/pubsub/v2"
	"cloud.google.com/go/pubsub/v2/apiv1/pubsubpb"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func BenchmarkPublish(b *testing.B) {
	ctx := context.Background()

	// Create a pstest server
	srv := pstest.NewServer()
	defer func() { _ = srv.Close() }()

	// Connect to the server
	conn, err := grpc.NewClient(srv.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		b.Fatalf("grpc.Dial: %v", err)
	}
	defer func() { _ = conn.Close() }()

	// Create client
	client, err := pubsub.NewClient(ctx, "project",
		option.WithGRPCConn(conn),
		option.WithoutAuthentication())
	if err != nil {
		b.Fatalf("pubsub.NewClient: %v", err)
	}
	defer func() { _ = client.Close() }()

	// Create topic using admin client from client
	topicID := "benchmark-topic"
	topicPath := "projects/project/topics/" + topicID
	_, err = client.TopicAdminClient.GetTopic(ctx, &pubsubpb.GetTopicRequest{
		Topic: topicPath,
	})
	if err != nil {
		// Topic doesn't exist, create it
		_, err = client.TopicAdminClient.CreateTopic(ctx, &pubsubpb.Topic{
			Name: topicPath,
		})
		if err != nil {
			b.Fatalf("CreateTopic: %v", err)
		}
	}

	// Test data
	testData := struct {
		EventType string `json:"event_type"`
		BuildID   string `json:"build_id"`
		Pipeline  string `json:"pipeline"`
		Branch    string `json:"branch"`
		Commit    string `json:"commit"`
		State     string `json:"state"`
	}{
		EventType: "build.finished",
		BuildID:   "01234567-89ab-cdef-0123-456789abcdef",
		Pipeline:  "my-pipeline",
		Branch:    "main",
		Commit:    "abc123def456",
		State:     "passed",
	}

	attrs := map[string]string{
		"event_type": "build.finished",
		"pipeline":   "my-pipeline",
		"branch":     "main",
	}

	b.Run("Default Settings", func(b *testing.B) {
		// Create publisher with default settings
		publisher := client.Publisher(topicID)
		pub := &PubSubPublisher{
			client:    client,
			publisher: publisher,
			topicID:   topicID,
			projectID: "project",
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := pub.Publish(ctx, testData, attrs)
			if err != nil {
				b.Fatalf("Publish() error = %v", err)
			}
		}
	})

	b.Run("Optimized Settings", func(b *testing.B) {
		// Create publisher with optimized settings
		optimizedPublisher := client.Publisher(topicID)
		optimizedPublisher.PublishSettings = pubsub.PublishSettings{
			CountThreshold:            100,
			ByteThreshold:             1e6,
			DelayThreshold:            10e6,
			NumGoroutines:             4,
			EnableCompression:         true,
			CompressionBytesThreshold: 1000,
		}

		pub := &PubSubPublisher{
			client:    client,
			publisher: optimizedPublisher,
			topicID:   topicID,
			projectID: "project",
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := pub.Publish(ctx, testData, attrs)
			if err != nil {
				b.Fatalf("Publish() error = %v", err)
			}
		}
		// Flush at the end to ensure all messages are sent
		pub.Flush()
	})

	b.Run("Async Publish", func(b *testing.B) {
		// Create publisher with optimized settings for async
		asyncPublisher := client.Publisher(topicID)
		asyncPublisher.PublishSettings = pubsub.PublishSettings{
			CountThreshold:    100,
			ByteThreshold:     1e6,
			DelayThreshold:    10e6,
			NumGoroutines:     4,
			EnableCompression: true,
			FlowControlSettings: pubsub.FlowControlSettings{
				MaxOutstandingMessages: 10000,
				MaxOutstandingBytes:    1e9,
				LimitExceededBehavior:  pubsub.FlowControlBlock,
			},
		}

		pub := &PubSubPublisher{
			client:    client,
			publisher: asyncPublisher,
			topicID:   topicID,
			projectID: "project",
		}

		b.ResetTimer()
		results := make([]*pubsub.PublishResult, b.N)
		for i := 0; i < b.N; i++ {
			results[i] = pub.PublishAsync(ctx, testData, attrs)
		}

		// Wait for all results
		for _, result := range results {
			_, err := result.Get(ctx)
			if err != nil {
				b.Fatalf("PublishAsync() error = %v", err)
			}
		}
	})
}
