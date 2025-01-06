package main

import (
	"context"
	"crypto/tls"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mcncl/buildkite-pubsub/internal/middleware"
	"github.com/mcncl/buildkite-pubsub/internal/publisher"
	"github.com/mcncl/buildkite-pubsub/pkg/webhook"
)

func main() {
	ctx := context.Background()

	// Initialize health checker
	healthCheck := webhook.NewHealthCheck()

	// Create publisher
	pub, err := publisher.NewPubSubPublisher(ctx,
		os.Getenv("PROJECT_ID"),
		os.Getenv("TOPIC_ID"),
	)
	if err != nil {
		log.Fatalf("Failed to create publisher: %v", err)
	}
	defer pub.Close()

	// Create webhook handler
	webhookHandler := webhook.NewHandler(webhook.Config{
		BuildkiteToken: os.Getenv("BUILDKITE_WEBHOOK_TOKEN"),
		Publisher:      pub,
	})

	// Create router
	mux := http.NewServeMux()

	// Add health check routes
	mux.HandleFunc("/health", healthCheck.HealthHandler)
	mux.HandleFunc("/ready", healthCheck.ReadyHandler)

	securityConfig := middleware.DefaultSecurityConfig()

	// Add webhook route with middleware
	mux.Handle("/webhook", chainMiddleware(
		webhookHandler,
		middleware.WithRequestID,
		middleware.WithRequestTimeout(30*time.Second),
		middleware.WithSecurity(securityConfig),
		middleware.WithRateLimit(60),
		middleware.WithPerIPRateLimit(30),
		middleware.WithLogging,
	))

	// Configure server
	srv := &http.Server{
		Addr:         ":" + getPort(),
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
		TLSConfig: &tls.Config{
			MinVersion:               tls.VersionTLS12,
			PreferServerCipherSuites: true,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			},
		},
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

// Middleware chain helper
func chainMiddleware(handler http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	// Apply middleware in reverse order so they execute in the order they're passed
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
}
