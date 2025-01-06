package webhook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/mcncl/buildkite-pubsub/internal/buildkite"
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
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Validate token first
	if !h.validator.ValidateToken(r) {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	// Read the body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read body: %v", err), http.StatusBadRequest)
		return
	}
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	// Parse payload
	var payload buildkite.Payload
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, fmt.Sprintf("Failed to decode payload: %v", err), http.StatusBadRequest)
		return
	}

	// Handle ping event specially
	if payload.Event == "ping" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Pong! Webhook received successfully",
		})
		return
	}

	// Transform payload
	transformed, err := buildkite.Transform(payload)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to transform payload: %v", err), http.StatusInternalServerError)
		return
	}

	// Publish to Pub/Sub
	msgID, err := h.publisher.Publish(r.Context(), transformed, map[string]string{
		"origin":     "buildkite-webhook",
		"event_type": payload.Event,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to publish message: %v", err), http.StatusInternalServerError)
		return
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message":    "Event published successfully",
		"message_id": msgID,
	})
}
