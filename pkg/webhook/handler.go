package webhook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mcncl/buildkite-pubsub/internal/buildkite"
	"github.com/mcncl/buildkite-pubsub/internal/metrics"
	"github.com/mcncl/buildkite-pubsub/internal/publisher"
)

// Config holds the configuration for the webhook handler
type Config struct {
	BuildkiteToken string
	Publisher      publisher.Publisher
}

// Handler handles incoming Buildkite webhooks
type Handler struct {
	validator *buildkite.Validator
	publisher publisher.Publisher
}

// NewHandler creates a new webhook handler
func NewHandler(cfg Config) *Handler {
	return &Handler{
		validator: buildkite.NewValidator(cfg.BuildkiteToken),
		publisher: cfg.Publisher,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	var eventType string = "unknown"

	if r.Method != http.MethodPost {
		metrics.WebhookRequestsTotal.WithLabelValues("405", eventType).Inc()
		metrics.ErrorsTotal.WithLabelValues("method_not_allowed").Inc()
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Validate token first
	if !h.validator.ValidateToken(r) {
		metrics.AuthFailures.Inc()
		metrics.WebhookRequestsTotal.WithLabelValues("401", eventType).Inc()
		metrics.ErrorsTotal.WithLabelValues("auth_failure").Inc()
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	// Read and measure the body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		metrics.WebhookRequestsTotal.WithLabelValues("400", eventType).Inc()
		metrics.ErrorsTotal.WithLabelValues("body_read_error").Inc()
		http.Error(w, fmt.Sprintf("Failed to read body: %v", err), http.StatusBadRequest)
		return
	}
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	// Record initial message size
	metrics.RecordMessageSize("raw", len(body))

	// Start payload processing timer
	processStart := time.Now()

	// Parse payload
	var payload buildkite.Payload
	if err := json.Unmarshal(body, &payload); err != nil {
		metrics.WebhookRequestsTotal.WithLabelValues("400", eventType).Inc()
		metrics.ErrorsTotal.WithLabelValues("json_decode_error").Inc()
		http.Error(w, fmt.Sprintf("Failed to decode payload: %v", err), http.StatusBadRequest)
		return
	}

	eventType = payload.Event

	// Record payload processing duration
	metrics.PayloadProcessingDuration.WithLabelValues(eventType).Observe(time.Since(processStart).Seconds())

	// Handle ping event specially
	if eventType == "ping" {
		metrics.WebhookRequestsTotal.WithLabelValues("200", eventType).Inc()
		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(map[string]string{
			"message": "Pong! Webhook received successfully",
		})
		if err != nil {
			metrics.ErrorsTotal.WithLabelValues("json_encode_error").Inc()
			http.Error(w, fmt.Sprintf("Failed to encode response: %v", err), http.StatusInternalServerError)
		}
		return
	}

	// Transform payload
	transformed, err := buildkite.Transform(payload)
	if err != nil {
		metrics.WebhookRequestsTotal.WithLabelValues("500", eventType).Inc()
		metrics.ErrorsTotal.WithLabelValues("transform_error").Inc()
		http.Error(w, fmt.Sprintf("Failed to transform payload: %v", err), http.StatusInternalServerError)
		return
	}

	// Record build metrics if this is a build event
	if build := transformed.Build; build.ID != "" {
		metrics.RecordBuildStatus(build.State, build.Pipeline)
		metrics.RecordPipelineBuild(build.Pipeline, build.Organization)

		// Calculate and record queue time for started builds
		if build.StartedAt.After(build.CreatedAt) {
			queueTime := build.StartedAt.Sub(build.CreatedAt).Seconds()
			metrics.RecordQueueTime(build.Pipeline, queueTime)
		}
	}

	// Track pub/sub publish time
	pubStart := time.Now()

	// Prepare for publishing
	transformedJSON, _ := json.Marshal(transformed)
	metrics.RecordPubsubMessageSize(eventType, len(transformedJSON))

	// Publish to Pub/Sub
	msgID, err := h.publisher.Publish(r.Context(), transformed, map[string]string{
		"origin":     "buildkite-webhook",
		"event_type": eventType,
	})

	pubDuration := time.Since(pubStart).Seconds()
	metrics.PubsubPublishDuration.Observe(pubDuration)

	if err != nil {
		metrics.WebhookRequestsTotal.WithLabelValues("500", eventType).Inc()
		metrics.PubsubPublishRequestsTotal.WithLabelValues("error", eventType).Inc()
		metrics.ErrorsTotal.WithLabelValues("publish_error").Inc()
		http.Error(w, fmt.Sprintf("Failed to publish message: %v", err), http.StatusInternalServerError)
		return
	}

	metrics.WebhookRequestsTotal.WithLabelValues("200", eventType).Inc()
	metrics.PubsubPublishRequestsTotal.WithLabelValues("success", eventType).Inc()
	metrics.WebhookRequestDuration.WithLabelValues(eventType).Observe(time.Since(start).Seconds())

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(map[string]string{
		"message":    "Event published successfully",
		"message_id": msgID,
	})
	if err != nil {
		metrics.ErrorsTotal.WithLabelValues("json_encode_error").Inc()
		http.Error(w, fmt.Sprintf("Failed to encode response: %v", err), http.StatusInternalServerError)
	}
}
