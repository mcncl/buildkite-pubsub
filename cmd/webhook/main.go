package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/mcncl/buildkite-pubsub/internal/config"
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
	// Parse command line flags
	configFile := flag.String("config", "", "Path to configuration file (JSON or YAML)")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configFile, nil)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Log the configuration (with sensitive values masked)
	log.Printf("Configuration loaded: %s", cfg)

	ctx := context.Background()

	// Initialize health checker
	healthCheck := webhook.NewHealthCheck()

	// Add metrics initialization
	reg := prometheus.NewRegistry()
	if err := metrics.InitMetrics(reg); err != nil {
		log.Fatalf("Failed to initialize metrics: %v", err)
	}

	// Create publisher
	pub, err := publisher.NewPubSubPublisher(ctx, cfg.GCP.ProjectID, cfg.GCP.TopicID)
	if err != nil {
		// Wrap the error with additional context
		if errors.IsConnectionError(err) {
			err = errors.Wrap(err, "failed to connect to Google Cloud Pub/Sub")
		} else {
			err = errors.Wrap(err, "failed to create publisher")
		}

		err = errors.WithDetails(err, map[string]interface{}{
			"project_id": cfg.GCP.ProjectID,
			"topic_id":   cfg.GCP.TopicID,
		})

		log.Fatalf("Publisher initialization error: %s", errors.Format(err))
	}
	defer pub.Close()

	// Create webhook handler
	webhookHandler := webhook.NewHandler(webhook.Config{
		BuildkiteToken: cfg.Webhook.Token,
		Publisher:      pub,
	})

	// Create router
	mux := http.NewServeMux()

	// Add metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())

	// Add health check routes
	mux.HandleFunc("/health", healthCheck.HealthHandler)
	mux.HandleFunc("/ready", healthCheck.ReadyHandler)

	// Create security configuration
	securityConfig := security.SecurityConfig{
		AllowedOrigins: cfg.Security.AllowedOrigins,
		AllowedMethods: cfg.Security.AllowedMethods,
		AllowedHeaders: cfg.Security.AllowedHeaders,
		MaxAge:         3600,
	}

	// Add webhook route with middleware
	// Note: The order of middleware is important!
	mux.Handle(cfg.Webhook.Path, chainMiddleware(
		webhookHandler,
		request.WithRequestID, // Generate request ID first
		logging.WithLogging,   // Add logging early for all requests
		security.WithSecurityHeaders(securityConfig),
		security.WithRateLimit(cfg.Security.RateLimit),     // Rate limiting before timeout
		security.WithIPRateLimit(cfg.Security.IPRateLimit), // IP-based rate limiting
		request.WithTimeout(cfg.Server.RequestTimeout),     // Timeout last
	))

	// Configure server
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      mux,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Server starting on port %d", cfg.Server.Port)
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
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.RequestTimeout)
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
