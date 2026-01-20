package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cloud.google.com/go/pubsub/v2"
	"github.com/mcncl/buildkite-pubsub/internal/config"
	"github.com/mcncl/buildkite-pubsub/internal/errors"
	"github.com/mcncl/buildkite-pubsub/internal/logging"
	"github.com/mcncl/buildkite-pubsub/internal/metrics"
	loggingMiddleware "github.com/mcncl/buildkite-pubsub/internal/middleware/logging"
	"github.com/mcncl/buildkite-pubsub/internal/middleware/request"
	"github.com/mcncl/buildkite-pubsub/internal/middleware/security"
	"github.com/mcncl/buildkite-pubsub/internal/publisher"
	"github.com/mcncl/buildkite-pubsub/internal/telemetry"
	"github.com/mcncl/buildkite-pubsub/pkg/webhook"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	// Parse command line flags
	configFile := flag.String("config", "", "Path to configuration file (JSON or YAML)")
	logLevel := flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	logFormat := flag.String("log-format", "json", "Log format (json, text, dev)")
	flag.Parse()

	// Initialize structured logger
	logger := initLogger(*logLevel, *logFormat)

	// Load configuration
	cfg, err := config.Load(*configFile, nil)
	if err != nil {
		logger.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Log the configuration (with sensitive values masked)
	logger.Info("Configuration loaded", "config", cfg.String())

	ctx := context.Background()

	// Initialize health checker
	healthCheck := webhook.NewHealthCheck()

	// Initialize telemetry if ENABLE_TRACING=true
	var telemetryProvider *telemetry.Provider
	if os.Getenv("ENABLE_TRACING") == "true" {
		telemetryConfig := telemetry.ConfigFromEnv()
		if telemetryConfig.ServiceName == "" {
			telemetryConfig.ServiceName = "buildkite-webhook"
		}

		telemetryProvider, err = telemetry.NewProvider(telemetryConfig)
		if err != nil {
			logger.Warn("Failed to create telemetry provider", "error", err)
		} else {
			startCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			if err := telemetryProvider.Start(startCtx); err != nil {
				logger.Warn("Failed to start telemetry", "error", err)
				telemetryProvider = nil
			} else {
				logger.Info("Tracing enabled", "endpoint", telemetryConfig.OTLPEndpoint)
			}
			cancel()
		}
	}

	// Add metrics initialization
	reg := prometheus.NewRegistry()
	if err := metrics.InitMetrics(reg); err != nil {
		logger.Error("Failed to initialize metrics", "error", err)
		os.Exit(1)
	}

	// Create publisher with optimized settings from config
	pubSettings := &pubsub.PublishSettings{
		CountThreshold: cfg.GCP.PubSubBatchSize,
		ByteThreshold:  1e6,  // 1MB
		DelayThreshold: 10e6, // 10ms
		NumGoroutines:  4,
		FlowControlSettings: pubsub.FlowControlSettings{
			MaxOutstandingMessages: 1000,
			MaxOutstandingBytes:    1e9,
			LimitExceededBehavior:  pubsub.FlowControlBlock,
		},
		EnableCompression:         true,
		CompressionBytesThreshold: 1000,
	}

	pub, err := publisher.NewPubSubPublisherWithSettings(ctx, cfg.GCP.ProjectID, cfg.GCP.TopicID, pubSettings)
	if err != nil {
		// Wrap the error with additional context
		if errors.IsConnectionError(err) {
			err = errors.Wrap(err, "failed to connect to Google Cloud Pub/Sub")
		} else {
			err = errors.Wrap(err, "failed to create publisher")
		}

		logger.Error("Publisher initialization error", "error", err, "project_id", cfg.GCP.ProjectID, "topic_id", cfg.GCP.TopicID)
		os.Exit(1)
	}
	defer func() {
		if err := pub.Close(); err != nil {
			logger.Error("Failed to close publisher", "error", err)
		}
	}()

	// Create webhook handler
	webhookHandler := webhook.NewHandler(webhook.Config{
		BuildkiteToken: cfg.Webhook.Token,
		HMACSecret:     cfg.Webhook.HMACSecret,
		Publisher:      pub,
	})

	// Create router
	mux := http.NewServeMux()

	// Add metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())

	// Add health check routes
	mux.HandleFunc("/health", healthCheck.HealthHandler)
	mux.HandleFunc("/ready", healthCheck.ReadyHandler)

	// Add webhook route with middleware
	var middlewares []func(http.Handler) http.Handler

	if telemetryProvider != nil {
		middlewares = append(middlewares, telemetryProvider.TracingMiddleware)
	}

	middlewares = append(middlewares,
		request.WithRequestID,
		loggingMiddleware.WithStructuredLogging(logger),
		security.WithRateLimit(cfg.Security.RateLimit),
		request.WithTimeout(cfg.Server.RequestTimeout),
	)

	mux.Handle(cfg.Webhook.Path, chainMiddleware(webhookHandler, middlewares...))

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
		logger.Info("Server starting", "port", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			logger.Error("HTTP server error", "error", err)
			os.Exit(1)
		}
	}()

	// Mark as ready to receive traffic
	healthCheck.SetReady(true)

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigChan
	logger.Info("Shutting down server", "signal", sig.String())

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.RequestTimeout)
	defer cancel()

	healthCheck.SetReady(false)
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server shutdown error", "error", err)
	}

	// Shutdown telemetry
	if telemetryProvider != nil {
		if err := telemetryProvider.Shutdown(shutdownCtx); err != nil {
			logger.Error("Telemetry shutdown error", "error", err)
		}
	}

	logger.Info("Server shutdown complete")
}

// initLogger creates and configures the structured logger
func initLogger(level, format string) *slog.Logger {
	return logging.NewLogger(level, format)
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
