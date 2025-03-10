package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/mcncl/buildkite-pubsub/internal/config"
	"github.com/mcncl/buildkite-pubsub/internal/errors"
	"github.com/mcncl/buildkite-pubsub/internal/logging"
	"github.com/mcncl/buildkite-pubsub/internal/metrics"
	loggingMiddleware "github.com/mcncl/buildkite-pubsub/internal/middleware/logging"
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
	logLevel := flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	logFormat := flag.String("log-format", "json", "Log format (json, text, dev)")
	flag.Parse()

	// Initialize structured logger
	logger := initLogger(*logLevel, *logFormat)

	// Load configuration
	cfg, err := config.Load(*configFile, nil)
	if err != nil {
		logger.WithError(err).Error("Failed to load configuration")
		os.Exit(1)
	}

	// Log the configuration (with sensitive values masked)
	logger.WithField("config", cfg.String()).Info("Configuration loaded")

	ctx := context.Background()

	// Initialize health checker
	healthCheck := webhook.NewHealthCheck()

	// Add metrics initialization
	reg := prometheus.NewRegistry()
	if err := metrics.InitMetrics(reg); err != nil {
		logger.WithError(err).Error("Failed to initialize metrics")
		os.Exit(1)
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

		logger.WithError(err).Error("Publisher initialization error")
		os.Exit(1)
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

	// Create rate limiters
	globalRateLimiter := security.NewGlobalRateLimiter(cfg.Security.RateLimit)
	ipRateLimiter := security.NewIPRateLimiter(cfg.Security.IPRateLimit)

	// Add webhook route with middleware
	// Note: The order of middleware is important!
	mux.Handle(cfg.Webhook.Path, chainMiddleware(
		webhookHandler,
		request.WithRequestID, // Generate request ID first
		loggingMiddleware.WithStructuredLogging(logger), // Add structured logging early for all requests
		security.WithSecurityHeaders(securityConfig),
		security.WithRateLimiter(globalRateLimiter),    // Global rate limiting
		security.WithRateLimiter(ipRateLimiter),        // IP-based rate limiting
		request.WithTimeout(cfg.Server.RequestTimeout), // Timeout last
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
		logger.WithField("port", cfg.Server.Port).Info("Server starting")
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			logger.WithError(err).Error("HTTP server error")
			os.Exit(1)
		}
	}()

	// Mark as ready to receive traffic
	healthCheck.SetReady(true)

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigChan
	logger.WithField("signal", sig.String()).Info("Shutting down server")

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.RequestTimeout)
	defer cancel()

	healthCheck.SetReady(false)
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.WithError(err).Error("HTTP server shutdown error")
	}

	logger.Info("Server shutdown complete")
}

// initLogger creates and configures the structured logger
func initLogger(level, format string) logging.Logger {
	// Parse log level
	var logLevel logging.Level
	switch level {
	case "debug":
		logLevel = logging.LevelDebug
	case "info":
		logLevel = logging.LevelInfo
	case "warn":
		logLevel = logging.LevelWarn
	case "error":
		logLevel = logging.LevelError
	default:
		logLevel = logging.LevelInfo
	}

	// Parse log format
	var logFormat logging.Format
	switch format {
	case "json":
		logFormat = logging.FormatJSON
	case "text":
		logFormat = logging.FormatText
	case "dev":
		logFormat = logging.FormatDevelopment
	default:
		logFormat = logging.FormatJSON
	}

	// Get hostname
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	// Create and return logger
	return logging.NewLogger(logging.Config{
		Output:   os.Stderr,
		Level:    logLevel,
		Format:   logFormat,
		AppName:  "buildkite-webhook",
		Hostname: hostname,
	})
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
