package webhook

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/mcncl/buildkite-pubsub/internal/buildkite"
	"github.com/mcncl/buildkite-pubsub/internal/errors"
	"github.com/mcncl/buildkite-pubsub/internal/metrics"
	"github.com/mcncl/buildkite-pubsub/internal/publisher"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// ErrorResponse represents a standardized error response
type ErrorResponse struct {
	Status     string      `json:"status"`
	Message    string      `json:"message"`
	ErrorType  string      `json:"error_type"`
	RetryAfter int         `json:"retry_after,omitempty"`
	Details    interface{} `json:"details,omitempty"`
}

// Config holds the configuration for the webhook handler
type Config struct {
	BuildkiteToken string
	HMACSecret     string
	Publisher      publisher.Publisher
}

// Handler handles incoming Buildkite webhooks
type Handler struct {
	validator *buildkite.Validator
	publisher publisher.Publisher
}

// NewHandler creates a new webhook handler
func NewHandler(cfg Config) *Handler {
	var validator *buildkite.Validator
	if cfg.HMACSecret != "" {
		validator = buildkite.NewValidatorWithHMAC(cfg.BuildkiteToken, cfg.HMACSecret)
	} else {
		validator = buildkite.NewValidator(cfg.BuildkiteToken)
	}

	return &Handler{
		validator: validator,
		publisher: cfg.Publisher,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	eventType := "unknown"

	// Track the request in metrics
	defer func() {
		metrics.WebhookRequestDuration.WithLabelValues(eventType).Observe(time.Since(start).Seconds())
	}()

	if r.Method != http.MethodPost {
		// Special case for method not allowed - use specific HTTP status code
		metrics.ErrorsTotal.WithLabelValues("method_not_allowed").Inc()
		metrics.WebhookRequestsTotal.WithLabelValues("405", eventType).Inc()

		response := ErrorResponse{
			Status:    "error",
			Message:   "Method not allowed, only POST is supported",
			ErrorType: "validation",
			Details: map[string]interface{}{
				"method": r.Method,
				"path":   r.URL.Path,
			},
		}

		h.sendJSONResponse(w, http.StatusMethodNotAllowed, response)
		return
	}

	// Validate token first
	if !h.validator.ValidateToken(r) {
		err := errors.NewAuthError("invalid token")
		metrics.AuthFailures.Inc()
		metrics.ErrorsTotal.WithLabelValues("auth_failure").Inc()
		h.handleError(w, r, err, eventType)
		return
	}

	// Read and measure the body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		err = errors.Wrap(err, "failed to read request body")
		metrics.ErrorsTotal.WithLabelValues("body_read_error").Inc()
		h.handleError(w, r, err, eventType)
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
		err = errors.NewValidationError("failed to decode payload")
		err = errors.WithDetails(err, map[string]interface{}{
			"error":       err.Error(),
			"body_length": len(body),
		})
		metrics.ErrorsTotal.WithLabelValues("json_decode_error").Inc()
		h.handleError(w, r, err, eventType)
		return
	}

	eventType = payload.Event

	// Record payload processing duration
	metrics.PayloadProcessingDuration.WithLabelValues(eventType).Observe(time.Since(processStart).Seconds())

	// Handle ping event specially
	if eventType == "ping" {
		metrics.WebhookRequestsTotal.WithLabelValues("200", eventType).Inc()
		h.sendJSONResponse(w, http.StatusOK, map[string]string{
			"status":  "success",
			"message": "Pong! Webhook received successfully",
		})
		return
	}

	// Transform payload
	tracer := otel.Tracer("buildkite-webhook")
	ctx, transformSpan := tracer.Start(r.Context(), "transform_payload",
		trace.WithAttributes(
			attribute.String("event_type", eventType),
			attribute.String("build_id", payload.Build.ID),
		))
	transformed, err := buildkite.Transform(payload)
	transformSpan.End()

	if err != nil {
		transformSpan.RecordError(err)
		err = errors.Wrap(err, "failed to transform payload")
		metrics.ErrorsTotal.WithLabelValues("transform_error").Inc()
		h.handleError(w, r, err, eventType)
		return
	}

	// Record build metrics if this is a build event
	if build := transformed.Build; build.ID != "" {
		metrics.RecordBuildStatus(build.State, build.Pipeline)
		metrics.RecordPipelineBuild(build.Pipeline, build.Organization)

		// Calculate and record queue time for started builds
		if !build.StartedAt.IsZero() && build.StartedAt.After(build.CreatedAt) {
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
	ctx, publishSpan := tracer.Start(ctx, "pubsub_publish",
		trace.WithAttributes(
			attribute.String("event_type", eventType),
			attribute.String("pipeline", transformed.Pipeline.Name),
		))
	defer publishSpan.End()

	// Build comprehensive attributes for Pub/Sub filtering
	pubsubAttributes := map[string]string{
		"origin":      "buildkite-webhook",
		"event_type":  eventType,
		"pipeline":    transformed.Pipeline.Name,
		"build_state": transformed.Build.State,
		"branch":      transformed.Build.Branch,
	}

	msgID, err := h.publisher.Publish(ctx, transformed, pubsubAttributes)

	pubDuration := time.Since(pubStart).Seconds()
	metrics.PubsubPublishDuration.Observe(pubDuration)

	if err != nil {
		publishSpan.RecordError(err)
		publishSpan.SetStatus(codes.Error, "publish failed")

		// Classify and handle the publish error
		var publishErr error
		if errors.IsConnectionError(err) {
			publishErr = errors.NewConnectionError("failed to connect to Pub/Sub")
			metrics.PubsubPublishRequestsTotal.WithLabelValues("error", eventType).Inc()
		} else if errors.IsRateLimitError(err) {
			publishErr = err // Already a rate limit error
			metrics.PubsubPublishRequestsTotal.WithLabelValues("rate_limit", eventType).Inc()
		} else {
			publishErr = errors.NewPublishError("failed to publish message", err)
			metrics.PubsubPublishRequestsTotal.WithLabelValues("error", eventType).Inc()
		}

		// Add context information to the error
		publishErr = errors.WithDetails(publishErr, map[string]interface{}{
			"event_type":   eventType,
			"payload_size": len(transformedJSON),
			"build_id":     transformed.Build.ID,
			"pipeline":     transformed.Build.Pipeline,
		})

		metrics.ErrorsTotal.WithLabelValues("publish_error").Inc()
		h.handleError(w, r, publishErr, eventType)
		return
	}

	// Record successful publish
	publishSpan.SetAttributes(attribute.String("message_id", msgID))
	publishSpan.SetStatus(codes.Ok, "published successfully")

	metrics.WebhookRequestsTotal.WithLabelValues("200", eventType).Inc()
	metrics.PubsubPublishRequestsTotal.WithLabelValues("success", eventType).Inc()

	// Return success response
	h.sendJSONResponse(w, http.StatusOK, map[string]interface{}{
		"status":     "success",
		"message":    "Event published successfully",
		"message_id": msgID,
		"event_type": eventType,
	})
}

// handleError processes errors and returns appropriate HTTP responses
func (h *Handler) handleError(w http.ResponseWriter, r *http.Request, err error, eventType string) {
	// Always record error in metrics
	metrics.WebhookRequestsTotal.WithLabelValues(h.getStatusCodeForError(err), eventType).Inc()

	var errorType string

	// Extract error details if available
	details := errors.GetDetails(err)

	// Create error response based on error type
	response := ErrorResponse{
		Status:  "error",
		Message: errors.Format(err),
	}

	// Set error type and specific handling based on error type
	switch {
	case errors.IsAuthError(err):
		errorType = "auth"
		response.ErrorType = errorType
		h.sendJSONResponse(w, http.StatusUnauthorized, response)

	case errors.IsValidationError(err):
		errorType = "validation"
		response.ErrorType = errorType
		response.Details = details
		h.sendJSONResponse(w, http.StatusBadRequest, response)

	case errors.IsRateLimitError(err):
		errorType = "rate_limit"
		response.ErrorType = errorType
		response.RetryAfter = 60 // Suggest retry after 60 seconds
		h.sendJSONResponse(w, http.StatusTooManyRequests, response)

	case errors.IsConnectionError(err):
		errorType = "connection"
		response.ErrorType = errorType
		response.RetryAfter = 30 // Suggest retry after 30 seconds
		h.sendJSONResponse(w, http.StatusServiceUnavailable, response)

	case errors.IsPublishError(err):
		errorType = "publish"
		response.ErrorType = errorType
		response.Details = details
		h.sendJSONResponse(w, http.StatusInternalServerError, response)

	default:
		// Handle any other errors as internal errors
		errorType = "internal"
		response.ErrorType = errorType
		h.sendJSONResponse(w, http.StatusInternalServerError, response)
	}
}

// getStatusCodeForError returns an appropriate HTTP status code for an error
func (h *Handler) getStatusCodeForError(err error) string {
	switch {
	case errors.IsAuthError(err):
		return "401"
	case errors.IsValidationError(err):
		// Check for method not allowed
		details := errors.GetDetails(err)
		if details != nil {
			if method, ok := details["method"]; ok {
				if method != "POST" {
					return "405"
				}
			}
		}
		return "400"
	case errors.IsRateLimitError(err):
		return "429"
	case errors.IsConnectionError(err):
		return "503"
	case errors.IsPublishError(err):
		return "500"
	default:
		return "500"
	}
}

// sendJSONResponse sends a JSON response with the given status code
func (h *Handler) sendJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		// If we can't encode the response, log it but there's not much we can do at this point
		metrics.ErrorsTotal.WithLabelValues("json_encode_error").Inc()
	}
}
