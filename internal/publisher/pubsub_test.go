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

func TestPubSubPublisher(t *testing.T) {
	ctx := context.Background()

	// Create a pstest server
	srv := pstest.NewServer()
	defer func() { _ = srv.Close() }()

	// Connect to the server
	conn, err := grpc.NewClient(srv.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc.Dial: %v", err)
	}
	defer func() { _ = conn.Close() }()

	// Create client with v2 API
	client, err := pubsub.NewClient(ctx, "project",
		option.WithGRPCConn(conn),
		option.WithoutAuthentication())
	if err != nil {
		t.Fatalf("pubsub.NewClient: %v", err)
	}
	defer func() { _ = client.Close() }()

	// Create topic using admin client from client
	topicID := "test-topic"
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
			t.Fatalf("CreateTopic: %v", err)
		}
	}

	// Create publisher with v2 API
	publisher := client.Publisher(topicID)
	publisher.PublishSettings = pubsub.PublishSettings{
		CountThreshold: 100,
		ByteThreshold:  1e6,
		DelayThreshold: 10e6,
	}

	// Create test publisher
	pub := &PubSubPublisher{
		client:    client,
		publisher: publisher,
		topicID:   topicID,
		projectID: "project",
	}

	// Test data
	testData := struct {
		Message string `json:"message"`
	}{
		Message: "test message",
	}
	attrs := map[string]string{
		"key": "value",
	}

	// Test Publish
	msgID, err := pub.Publish(ctx, testData, attrs)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if msgID == "" {
		t.Error("Publish() returned empty message ID")
	}

	// Test Close
	if err := pub.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}
