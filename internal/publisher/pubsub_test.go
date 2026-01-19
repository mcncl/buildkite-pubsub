package publisher

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"cloud.google.com/go/pubsub/pstest"
	"cloud.google.com/go/pubsub/v2"
	"cloud.google.com/go/pubsub/v2/apiv1/pubsubpb"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// testSetup creates a pstest server and client for testing
func testSetup(t *testing.T) (*pstest.Server, *pubsub.Client, func()) {
	t.Helper()
	ctx := context.Background()

	srv := pstest.NewServer()

	conn, err := grpc.NewClient(srv.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		srv.Close()
		t.Fatalf("grpc.NewClient: %v", err)
	}

	client, err := pubsub.NewClient(ctx, "test-project",
		option.WithGRPCConn(conn),
		option.WithoutAuthentication())
	if err != nil {
		conn.Close()
		srv.Close()
		t.Fatalf("pubsub.NewClient: %v", err)
	}

	cleanup := func() {
		client.Close()
		conn.Close()
		srv.Close()
	}

	return srv, client, cleanup
}

// createTopic creates a topic in the test server
func createTopic(t *testing.T, client *pubsub.Client, topicID string) {
	t.Helper()
	ctx := context.Background()

	topicPath := "projects/test-project/topics/" + topicID
	_, err := client.TopicAdminClient.CreateTopic(ctx, &pubsubpb.Topic{
		Name: topicPath,
	})
	if err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}
}

// createTestPublisher creates a PubSubPublisher for testing
func createTestPublisher(t *testing.T, client *pubsub.Client, topicID string) *PubSubPublisher {
	t.Helper()

	publisher := client.Publisher(topicID)
	publisher.PublishSettings = pubsub.PublishSettings{
		CountThreshold: 1, // Publish immediately for testing
		ByteThreshold:  1e6,
		DelayThreshold: 10e6,
	}

	return &PubSubPublisher{
		client:    client,
		publisher: publisher,
		topicID:   topicID,
		projectID: "test-project",
	}
}

func TestPubSubPublisher_Publish(t *testing.T) {
	_, client, cleanup := testSetup(t)
	defer cleanup()

	topicID := "test-topic"
	createTopic(t, client, topicID)

	pub := createTestPublisher(t, client, topicID)
	defer pub.Close()

	ctx := context.Background()
	testData := map[string]string{"message": "test"}
	attrs := map[string]string{"key": "value"}

	msgID, err := pub.Publish(ctx, testData, attrs)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if msgID == "" {
		t.Error("Publish() returned empty message ID")
	}
}

func TestPubSubPublisher_Publish_WithNilAttributes(t *testing.T) {
	_, client, cleanup := testSetup(t)
	defer cleanup()

	topicID := "test-topic-nil-attrs"
	createTopic(t, client, topicID)

	pub := createTestPublisher(t, client, topicID)
	defer pub.Close()

	ctx := context.Background()
	testData := map[string]string{"message": "test"}

	msgID, err := pub.Publish(ctx, testData, nil)
	if err != nil {
		t.Fatalf("Publish() with nil attributes error = %v", err)
	}
	if msgID == "" {
		t.Error("Publish() returned empty message ID")
	}
}

func TestPubSubPublisher_Publish_MarshalError(t *testing.T) {
	_, client, cleanup := testSetup(t)
	defer cleanup()

	topicID := "test-topic-marshal"
	createTopic(t, client, topicID)

	pub := createTestPublisher(t, client, topicID)
	defer pub.Close()

	ctx := context.Background()

	// Create an unmarshalable type (channel cannot be JSON marshaled)
	unmarshalable := make(chan int)

	_, err := pub.Publish(ctx, unmarshalable, nil)
	if err == nil {
		t.Error("Publish() expected error for unmarshalable data, got nil")
	}

	// Verify error message mentions marshaling
	if err != nil {
		expectedSubstring := "failed to marshal"
		if !containsSubstring(err.Error(), expectedSubstring) {
			t.Errorf("Publish() error = %v, want error containing %q", err, expectedSubstring)
		}
	}
}

func TestPubSubPublisher_Publish_ContextCancelled(t *testing.T) {
	_, client, cleanup := testSetup(t)
	defer cleanup()

	topicID := "test-topic-ctx"
	createTopic(t, client, topicID)

	pub := createTestPublisher(t, client, topicID)
	defer pub.Close()

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	testData := map[string]string{"message": "test"}

	_, err := pub.Publish(ctx, testData, nil)
	if err == nil {
		t.Error("Publish() expected error for cancelled context, got nil")
	}
}

func TestPubSubPublisher_Publish_ContextTimeout(t *testing.T) {
	_, client, cleanup := testSetup(t)
	defer cleanup()

	topicID := "test-topic-timeout"
	createTopic(t, client, topicID)

	pub := createTestPublisher(t, client, topicID)
	defer pub.Close()

	// Create a context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Give time for timeout to expire
	time.Sleep(1 * time.Millisecond)

	testData := map[string]string{"message": "test"}

	_, err := pub.Publish(ctx, testData, nil)
	if err == nil {
		t.Error("Publish() expected error for timed out context, got nil")
	}
}

func TestPubSubPublisher_Publish_ComplexData(t *testing.T) {
	_, client, cleanup := testSetup(t)
	defer cleanup()

	topicID := "test-topic-complex"
	createTopic(t, client, topicID)

	pub := createTestPublisher(t, client, topicID)
	defer pub.Close()

	ctx := context.Background()

	// Complex nested structure
	testData := struct {
		Event   string            `json:"event"`
		Build   map[string]any    `json:"build"`
		Tags    []string          `json:"tags"`
		Count   int               `json:"count"`
		Enabled bool              `json:"enabled"`
		Attrs   map[string]string `json:"attrs"`
	}{
		Event: "build.finished",
		Build: map[string]any{
			"id":     "123",
			"number": 42,
			"nested": map[string]string{"key": "value"},
		},
		Tags:    []string{"ci", "main", "deploy"},
		Count:   100,
		Enabled: true,
		Attrs:   map[string]string{"env": "prod"},
	}

	attrs := map[string]string{
		"event_type": "build.finished",
		"pipeline":   "main-pipeline",
	}

	msgID, err := pub.Publish(ctx, testData, attrs)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if msgID == "" {
		t.Error("Publish() returned empty message ID")
	}
}

func TestPubSubPublisher_PublishAsync(t *testing.T) {
	_, client, cleanup := testSetup(t)
	defer cleanup()

	topicID := "test-topic-async"
	createTopic(t, client, topicID)

	pub := createTestPublisher(t, client, topicID)
	defer pub.Close()

	ctx := context.Background()
	testData := map[string]string{"message": "async test"}
	attrs := map[string]string{"async": "true"}

	result := pub.PublishAsync(ctx, testData, attrs)
	if result == nil {
		t.Fatal("PublishAsync() returned nil result")
	}

	// Wait for the result
	msgID, err := result.Get(ctx)
	if err != nil {
		t.Fatalf("PublishAsync().Get() error = %v", err)
	}
	if msgID == "" {
		t.Error("PublishAsync() returned empty message ID")
	}
}

func TestPubSubPublisher_PublishAsync_Multiple(t *testing.T) {
	_, client, cleanup := testSetup(t)
	defer cleanup()

	topicID := "test-topic-async-multi"
	createTopic(t, client, topicID)

	pub := createTestPublisher(t, client, topicID)
	defer pub.Close()

	ctx := context.Background()
	numMessages := 10
	results := make([]*pubsub.PublishResult, numMessages)

	// Publish multiple messages asynchronously
	for i := 0; i < numMessages; i++ {
		testData := map[string]int{"index": i}
		results[i] = pub.PublishAsync(ctx, testData, nil)
	}

	// Verify all results
	messageIDs := make(map[string]bool)
	for i, result := range results {
		msgID, err := result.Get(ctx)
		if err != nil {
			t.Errorf("PublishAsync()[%d].Get() error = %v", i, err)
			continue
		}
		if msgID == "" {
			t.Errorf("PublishAsync()[%d] returned empty message ID", i)
		}
		messageIDs[msgID] = true
	}

	// Verify we got unique message IDs
	if len(messageIDs) != numMessages {
		t.Errorf("Expected %d unique message IDs, got %d", numMessages, len(messageIDs))
	}
}

func TestPubSubPublisher_TopicID(t *testing.T) {
	_, client, cleanup := testSetup(t)
	defer cleanup()

	topicID := "my-custom-topic"
	createTopic(t, client, topicID)

	pub := createTestPublisher(t, client, topicID)
	defer pub.Close()

	if got := pub.TopicID(); got != topicID {
		t.Errorf("TopicID() = %v, want %v", got, topicID)
	}
}

func TestPubSubPublisher_Flush(t *testing.T) {
	_, client, cleanup := testSetup(t)
	defer cleanup()

	topicID := "test-topic-flush"
	createTopic(t, client, topicID)

	// Create publisher with batching settings
	publisher := client.Publisher(topicID)
	publisher.PublishSettings = pubsub.PublishSettings{
		CountThreshold: 100, // High threshold to trigger batching
		ByteThreshold:  1e6,
		DelayThreshold: 1e9, // 1 second delay
	}

	pub := &PubSubPublisher{
		client:    client,
		publisher: publisher,
		topicID:   topicID,
		projectID: "test-project",
	}
	defer pub.Close()

	ctx := context.Background()

	// Publish some messages (will be batched)
	for i := 0; i < 5; i++ {
		testData := map[string]int{"index": i}
		pub.PublishAsync(ctx, testData, nil)
	}

	// Flush should not panic and should complete quickly
	done := make(chan bool)
	go func() {
		pub.Flush()
		done <- true
	}()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Error("Flush() timed out")
	}
}

func TestPubSubPublisher_Close(t *testing.T) {
	_, client, cleanup := testSetup(t)
	defer cleanup()

	topicID := "test-topic-close"
	createTopic(t, client, topicID)

	pub := createTestPublisher(t, client, topicID)

	// Close should not return an error
	err := pub.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestPubSubPublisher_Close_AfterPublish(t *testing.T) {
	_, client, cleanup := testSetup(t)
	defer cleanup()

	topicID := "test-topic-close-after"
	createTopic(t, client, topicID)

	pub := createTestPublisher(t, client, topicID)

	ctx := context.Background()

	// Publish some messages
	for i := 0; i < 3; i++ {
		_, err := pub.Publish(ctx, map[string]int{"i": i}, nil)
		if err != nil {
			t.Fatalf("Publish() error = %v", err)
		}
	}

	// Close should flush and complete successfully
	err := pub.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestPubSubPublisher_Publish_EmptyData(t *testing.T) {
	_, client, cleanup := testSetup(t)
	defer cleanup()

	topicID := "test-topic-empty"
	createTopic(t, client, topicID)

	pub := createTestPublisher(t, client, topicID)
	defer pub.Close()

	ctx := context.Background()

	// Empty struct
	testData := struct{}{}

	msgID, err := pub.Publish(ctx, testData, nil)
	if err != nil {
		t.Fatalf("Publish() with empty data error = %v", err)
	}
	if msgID == "" {
		t.Error("Publish() returned empty message ID")
	}
}

func TestPubSubPublisher_Publish_LargePayload(t *testing.T) {
	_, client, cleanup := testSetup(t)
	defer cleanup()

	topicID := "test-topic-large"
	createTopic(t, client, topicID)

	pub := createTestPublisher(t, client, topicID)
	defer pub.Close()

	ctx := context.Background()

	// Create a large payload (100KB)
	largeString := make([]byte, 100*1024)
	for i := range largeString {
		largeString[i] = 'a'
	}

	testData := map[string]string{
		"large_field": string(largeString),
	}

	msgID, err := pub.Publish(ctx, testData, nil)
	if err != nil {
		t.Fatalf("Publish() with large payload error = %v", err)
	}
	if msgID == "" {
		t.Error("Publish() returned empty message ID")
	}
}

func TestPubSubPublisher_Publish_SpecialCharacters(t *testing.T) {
	_, client, cleanup := testSetup(t)
	defer cleanup()

	topicID := "test-topic-special"
	createTopic(t, client, topicID)

	pub := createTestPublisher(t, client, topicID)
	defer pub.Close()

	ctx := context.Background()

	testData := map[string]string{
		"unicode":     "こんにちは世界 🎉 émojis",
		"escapes":     "line1\nline2\ttab",
		"quotes":      `"quoted" and 'single'`,
		"special":     "<>&",
		"backslashes": `path\to\file`,
	}

	attrs := map[string]string{
		"type": "special-chars",
	}

	msgID, err := pub.Publish(ctx, testData, attrs)
	if err != nil {
		t.Fatalf("Publish() with special characters error = %v", err)
	}
	if msgID == "" {
		t.Error("Publish() returned empty message ID")
	}
}

func TestPubSubPublisher_Publish_NullValues(t *testing.T) {
	_, client, cleanup := testSetup(t)
	defer cleanup()

	topicID := "test-topic-null"
	createTopic(t, client, topicID)

	pub := createTestPublisher(t, client, topicID)
	defer pub.Close()

	ctx := context.Background()

	// Struct with nil/zero values
	testData := struct {
		Name    *string `json:"name"`
		Count   int     `json:"count"`
		Enabled bool    `json:"enabled"`
	}{
		Name:    nil,
		Count:   0,
		Enabled: false,
	}

	msgID, err := pub.Publish(ctx, testData, nil)
	if err != nil {
		t.Fatalf("Publish() with null values error = %v", err)
	}
	if msgID == "" {
		t.Error("Publish() returned empty message ID")
	}
}

func TestPubSubPublisher_Publish_RawJSON(t *testing.T) {
	_, client, cleanup := testSetup(t)
	defer cleanup()

	topicID := "test-topic-rawjson"
	createTopic(t, client, topicID)

	pub := createTestPublisher(t, client, topicID)
	defer pub.Close()

	ctx := context.Background()

	// json.RawMessage is valid JSON that should be preserved
	testData := struct {
		Event string          `json:"event"`
		Raw   json.RawMessage `json:"raw"`
	}{
		Event: "test",
		Raw:   json.RawMessage(`{"nested": true, "count": 42}`),
	}

	msgID, err := pub.Publish(ctx, testData, nil)
	if err != nil {
		t.Fatalf("Publish() with RawJSON error = %v", err)
	}
	if msgID == "" {
		t.Error("Publish() returned empty message ID")
	}
}

// Tests for factory functions using the emulator

func TestNewPubSubPublisher_WithEmulator(t *testing.T) {
	// Start a pstest server and set up the emulator environment
	srv := pstest.NewServer()
	defer srv.Close()

	// Set the emulator host environment variable
	t.Setenv("PUBSUB_EMULATOR_HOST", srv.Addr)

	ctx := context.Background()
	projectID := "test-project"
	topicID := "test-topic-factory"

	// Create the topic first using a separate client
	setupClient, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		t.Fatalf("Failed to create setup client: %v", err)
	}

	topicPath := fmt.Sprintf("projects/%s/topics/%s", projectID, topicID)
	_, err = setupClient.TopicAdminClient.CreateTopic(ctx, &pubsubpb.Topic{
		Name: topicPath,
	})
	if err != nil {
		setupClient.Close()
		t.Fatalf("Failed to create topic: %v", err)
	}
	setupClient.Close()

	// Now test NewPubSubPublisher
	pub, err := NewPubSubPublisher(ctx, projectID, topicID)
	if err != nil {
		t.Fatalf("NewPubSubPublisher() error = %v", err)
	}
	defer pub.Close()

	// Verify it works
	msgID, err := pub.Publish(ctx, map[string]string{"test": "factory"}, nil)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if msgID == "" {
		t.Error("Publish() returned empty message ID")
	}
	if pub.TopicID() != topicID {
		t.Errorf("TopicID() = %v, want %v", pub.TopicID(), topicID)
	}
}

func TestNewPubSubPublisher_TopicNotExists(t *testing.T) {
	srv := pstest.NewServer()
	defer srv.Close()

	t.Setenv("PUBSUB_EMULATOR_HOST", srv.Addr)

	ctx := context.Background()
	projectID := "test-project"
	topicID := "non-existent-topic"

	// Try to create publisher for non-existent topic
	_, err := NewPubSubPublisher(ctx, projectID, topicID)
	if err == nil {
		t.Error("NewPubSubPublisher() expected error for non-existent topic, got nil")
	}

	// Verify error message mentions topic
	expectedSubstring := "does not exist"
	if !containsSubstring(err.Error(), expectedSubstring) {
		t.Errorf("NewPubSubPublisher() error = %v, want error containing %q", err, expectedSubstring)
	}
}

func TestNewPubSubPublisherWithSettings_CustomSettings(t *testing.T) {
	srv := pstest.NewServer()
	defer srv.Close()

	t.Setenv("PUBSUB_EMULATOR_HOST", srv.Addr)

	ctx := context.Background()
	projectID := "test-project"
	topicID := "test-topic-custom-settings"

	// Create the topic first
	setupClient, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		t.Fatalf("Failed to create setup client: %v", err)
	}

	topicPath := fmt.Sprintf("projects/%s/topics/%s", projectID, topicID)
	_, err = setupClient.TopicAdminClient.CreateTopic(ctx, &pubsubpb.Topic{
		Name: topicPath,
	})
	if err != nil {
		setupClient.Close()
		t.Fatalf("Failed to create topic: %v", err)
	}
	setupClient.Close()

	// Custom settings
	customSettings := &pubsub.PublishSettings{
		CountThreshold:            50,
		ByteThreshold:             500000,
		DelayThreshold:            5e6,
		NumGoroutines:             2,
		EnableCompression:         true,
		CompressionBytesThreshold: 500,
	}

	pub, err := NewPubSubPublisherWithSettings(ctx, projectID, topicID, customSettings)
	if err != nil {
		t.Fatalf("NewPubSubPublisherWithSettings() error = %v", err)
	}
	defer pub.Close()

	// Verify it works
	msgID, err := pub.Publish(ctx, map[string]string{"test": "custom"}, nil)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if msgID == "" {
		t.Error("Publish() returned empty message ID")
	}
}

func TestNewPubSubPublisherWithSettings_NilSettings(t *testing.T) {
	srv := pstest.NewServer()
	defer srv.Close()

	t.Setenv("PUBSUB_EMULATOR_HOST", srv.Addr)

	ctx := context.Background()
	projectID := "test-project"
	topicID := "test-topic-nil-settings"

	// Create the topic first
	setupClient, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		t.Fatalf("Failed to create setup client: %v", err)
	}

	topicPath := fmt.Sprintf("projects/%s/topics/%s", projectID, topicID)
	_, err = setupClient.TopicAdminClient.CreateTopic(ctx, &pubsubpb.Topic{
		Name: topicPath,
	})
	if err != nil {
		setupClient.Close()
		t.Fatalf("Failed to create topic: %v", err)
	}
	setupClient.Close()

	// Nil settings should use defaults
	pub, err := NewPubSubPublisherWithSettings(ctx, projectID, topicID, nil)
	if err != nil {
		t.Fatalf("NewPubSubPublisherWithSettings() with nil settings error = %v", err)
	}
	defer pub.Close()

	// Verify it works
	msgID, err := pub.Publish(ctx, map[string]string{"test": "nil-settings"}, nil)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if msgID == "" {
		t.Error("Publish() returned empty message ID")
	}
}

func TestNewPubSubPublisherWithSettings(t *testing.T) {
	srv, client, cleanup := testSetup(t)
	defer cleanup()
	_ = srv // unused but needed for server lifetime

	topicID := "factory-test-topic"
	createTopic(t, client, topicID)

	// We can't directly test NewPubSubPublisherWithSettings because it creates its own client,
	// but we can test the behavior by using the pstest server with environment variables
	// For now, we test that the publisher created with custom settings works correctly

	customSettings := &pubsub.PublishSettings{
		CountThreshold:            50,
		ByteThreshold:             500000,
		DelayThreshold:            5e6,
		NumGoroutines:             2,
		EnableCompression:         true,
		CompressionBytesThreshold: 500,
	}

	publisher := client.Publisher(topicID)
	publisher.PublishSettings = *customSettings

	pub := &PubSubPublisher{
		client:    client,
		publisher: publisher,
		topicID:   topicID,
		projectID: "test-project",
	}
	defer pub.Close()

	// Verify it works with custom settings
	ctx := context.Background()
	msgID, err := pub.Publish(ctx, map[string]string{"test": "data"}, nil)
	if err != nil {
		t.Fatalf("Publish() with custom settings error = %v", err)
	}
	if msgID == "" {
		t.Error("Publish() returned empty message ID")
	}
}

func TestNewPubSubPublisherWithSettings_DefaultSettings(t *testing.T) {
	_, client, cleanup := testSetup(t)
	defer cleanup()

	topicID := "default-settings-topic"
	createTopic(t, client, topicID)

	// Test with nil settings (should use defaults)
	publisher := client.Publisher(topicID)
	// When settings is nil, the code applies default settings
	defaultSettings := pubsub.PublishSettings{
		CountThreshold: 100,
		ByteThreshold:  1e6,
		DelayThreshold: 10e6,
		NumGoroutines:  4,
		FlowControlSettings: pubsub.FlowControlSettings{
			MaxOutstandingMessages: 1000,
			MaxOutstandingBytes:    1e9,
			LimitExceededBehavior:  pubsub.FlowControlBlock,
		},
		EnableCompression:         true,
		CompressionBytesThreshold: 1000,
	}
	publisher.PublishSettings = defaultSettings

	pub := &PubSubPublisher{
		client:    client,
		publisher: publisher,
		topicID:   topicID,
		projectID: "test-project",
	}
	defer pub.Close()

	// Verify default settings work
	ctx := context.Background()
	msgID, err := pub.Publish(ctx, map[string]string{"test": "default"}, nil)
	if err != nil {
		t.Fatalf("Publish() with default settings error = %v", err)
	}
	if msgID == "" {
		t.Error("Publish() returned empty message ID")
	}
}

func TestPubSubPublisher_ConcurrentPublish(t *testing.T) {
	_, client, cleanup := testSetup(t)
	defer cleanup()

	topicID := "concurrent-topic"
	createTopic(t, client, topicID)

	pub := createTestPublisher(t, client, topicID)
	defer pub.Close()

	ctx := context.Background()
	numGoroutines := 10
	messagesPerGoroutine := 5

	// Use channels to collect results
	results := make(chan error, numGoroutines*messagesPerGoroutine)

	// Publish concurrently
	for g := 0; g < numGoroutines; g++ {
		go func(goroutineID int) {
			for m := 0; m < messagesPerGoroutine; m++ {
				data := map[string]int{
					"goroutine": goroutineID,
					"message":   m,
				}
				_, err := pub.Publish(ctx, data, nil)
				results <- err
			}
		}(g)
	}

	// Collect results
	totalMessages := numGoroutines * messagesPerGoroutine
	errorCount := 0
	for i := 0; i < totalMessages; i++ {
		if err := <-results; err != nil {
			errorCount++
			t.Logf("Concurrent publish error: %v", err)
		}
	}

	if errorCount > 0 {
		t.Errorf("Concurrent publish had %d errors out of %d messages", errorCount, totalMessages)
	}
}

func TestPubSubPublisher_PublishAfterClose(t *testing.T) {
	_, client, cleanup := testSetup(t)
	defer cleanup()

	topicID := "close-then-publish-topic"
	createTopic(t, client, topicID)

	pub := createTestPublisher(t, client, topicID)

	// Close the publisher
	err := pub.Close()
	if err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	// Attempt to publish after close - behavior may vary but shouldn't panic
	ctx := context.Background()
	// This may return an error, but should not panic
	_, _ = pub.Publish(ctx, map[string]string{"test": "after close"}, nil)
	// We don't assert on the error because behavior after close is implementation-dependent
}

func TestPubSubPublisher_FlushEmpty(t *testing.T) {
	_, client, cleanup := testSetup(t)
	defer cleanup()

	topicID := "flush-empty-topic"
	createTopic(t, client, topicID)

	pub := createTestPublisher(t, client, topicID)
	defer pub.Close()

	// Flush when nothing has been published should not panic
	done := make(chan bool)
	go func() {
		pub.Flush()
		done <- true
	}()

	select {
	case <-done:
		// Success - didn't panic
	case <-time.After(2 * time.Second):
		t.Error("Flush() on empty publisher timed out")
	}
}

func TestPubSubPublisher_PublishWithAllAttributes(t *testing.T) {
	_, client, cleanup := testSetup(t)
	defer cleanup()

	topicID := "all-attrs-topic"
	createTopic(t, client, topicID)

	pub := createTestPublisher(t, client, topicID)
	defer pub.Close()

	ctx := context.Background()

	// Comprehensive attributes like the real webhook handler uses
	attrs := map[string]string{
		"origin":      "buildkite-webhook",
		"event_type":  "build.finished",
		"pipeline":    "my-pipeline",
		"build_state": "passed",
		"branch":      "main",
		"commit":      "abc123",
		"build_id":    "12345",
	}

	testData := map[string]interface{}{
		"event": "build.finished",
		"build": map[string]interface{}{
			"id":     "12345",
			"state":  "passed",
			"branch": "main",
		},
	}

	msgID, err := pub.Publish(ctx, testData, attrs)
	if err != nil {
		t.Fatalf("Publish() with all attributes error = %v", err)
	}
	if msgID == "" {
		t.Error("Publish() returned empty message ID")
	}
}

// Helper function to check if a string contains a substring
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstringHelper(s, substr))
}

func containsSubstringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
