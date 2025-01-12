package publisher

import (
	"context"
	"testing"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/pubsub/pstest"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestPubSubPublisher(t *testing.T) {
	ctx := context.Background()

	// Create a pstest server
	srv := pstest.NewServer()
	defer srv.Close()

	// Connect to the server
	conn, err := grpc.NewClient(srv.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc.Dial: %v", err)
	}
	defer conn.Close()

	// Create client
	client, err := pubsub.NewClient(ctx, "project",
		option.WithGRPCConn(conn),
		option.WithoutAuthentication())
	if err != nil {
		t.Fatalf("pubsub.NewClient: %v", err)
	}
	defer client.Close()

	// Create topic
	topicID := "test-topic"
	topic := client.Topic(topicID)
	ok, err := topic.Exists(ctx)
	if err != nil {
		t.Fatalf("topic.Exists: %v", err)
	}
	if !ok {
		topic, err = client.CreateTopic(ctx, topicID)
		if err != nil {
			t.Fatalf("CreateTopic: %v", err)
		}
	}

	// Create test publisher
	pub := &PubSubPublisher{
		client: client,
		topic:  topic,
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
