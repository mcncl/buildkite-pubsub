package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mcncl/buildkite-pubsub/internal/errors"
	"github.com/mcncl/buildkite-pubsub/internal/metrics"
	"github.com/mcncl/buildkite-pubsub/internal/middleware/logging"
	"github.com/mcncl/buildkite-pubsub/internal/middleware/request"
	"github.com/mcncl/buildkite-pubsub/internal/middleware/security"
	"github.com/mcncl/buildkite-pubsub/internal/publisher"
	"github.com/mcncl/buildkite-pubsub/pkg/webhook"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	ctx := context.Background()

	// Initialize health checker
	healthCheck := webhook.NewHealthCheck()

	// Add metrics initialization
	reg := prometheus.NewRegistry()
	if err := metrics.InitMetrics(reg); err != nil {
		log.Fatalf("Failed to initialize metrics: %v", err)
	}

	// Get required environment variables
	projectID := os.Getenv("PROJECT_ID")
	topicID := os.Getenv("TOPIC_ID")
	webhookToken := os.Getenv("BUILDKITE_WEBHOOK_TOKEN")

	// Validate required environment variables
	if projectID == "" || topicID == "" || webhookToken == "" {
		err := errors.NewValidationError("missing required environment variables")
		err = errors.WithDetails(err, map[string]interface{}{
			"project_id_set":        projectID != "",
			"topic_id_set":          topicID != "",
			"buildkite_webhook_set": webhookToken != "",
		})
		log.Fatalf("Configuration error: %s", errors.Format(err))
	}

	// Create publisher
	pub, err := publisher.NewPubSubPublisher(ctx, projectID, topicID)
	if err != nil {
		// Wrap the error with additional context
		if errors.IsConnectionError(err) {
			err = errors.Wrap(err, "failed to connect to Google Cloud Pub/Sub")
		} else {
			err = errors.Wrap(err, "failed to create publisher")
		}

		err = errors.WithDetails(err, map[string]interface{}{
			"project_id": projectID,
			"topic_id":   topicID,
		})

		log.Fatalf("Publisher initialization error: %s", errors.Format(err))
	}
	defer pub.Close()

	// Create webhook handler
	webhookHandler := webhook.NewHandler(webhook.Config{
		BuildkiteToken: webhookToken,
		Publisher:      pub,
	})

	// Create router
	mux := http.NewServeMux()

	// Add metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())

	// Add health check routes
	mux.HandleFunc("/health", healthCheck.HealthHandler)
	mux.HandleFunc("/ready", healthCheck.ReadyHandler)

	securityConfig := security.DefaultConfig()

	// Add webhook route with middleware
	// Note: The order of middleware is important!
	mux.Handle("/webhook", chainMiddleware(
		webhookHandler,
		request.WithRequestID, // Generate request ID first
		logging.WithLogging,   // Add logging early for all requests
		security.WithSecurityHeaders(securityConfig),
		security.WithRateLimit(60),          // Rate limiting before timeout
		security.WithIPRateLimit(30),        // IP-based rate limiting
		request.WithTimeout(30*time.Second), // Timeout last
	))

	// Configure server
	srv := &http.Server{
		Addr:         ":" + getPort(),
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Server starting on port %s", getPort())
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Mark as ready to receive traffic
	healthCheck.SetReady(true)

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	// Graceful shutdown
	log.Println("Shutting down server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	healthCheck.SetReady(false)
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server Shutdown: %v", err)
	}
}

func getPort() string {
	if port := os.Getenv("PORT"); port != "" {
		return port
	}
	return "8080"
}

// Middleware chain helper - applies middleware in reverse order
// so they execute in the order they're passed
func chainMiddleware(handler http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
}
