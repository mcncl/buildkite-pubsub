package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Webhook request metrics
	WebhookRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "buildkite_webhook_requests_total",
			Help: "Total number of webhook requests received",
		},
		[]string{"status", "event_type"},
	)

	WebhookRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "buildkite_webhook_request_duration_seconds",
			Help:    "Duration of webhook requests in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"event_type"},
	)

	// Authentication metrics
	AuthFailures = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "buildkite_webhook_auth_failures_total",
			Help: "Total number of authentication failures",
		},
	)

	// PubSub metrics
	PubsubPublishRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "buildkite_pubsub_publish_requests_total",
			Help: "Total number of Pub/Sub publish requests",
		},
		[]string{"status"},
	)

	PubsubPublishDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "buildkite_pubsub_publish_duration_seconds",
			Help:    "Duration of Pub/Sub publish operations in seconds",
			Buckets: prometheus.DefBuckets,
		},
	)

	// Rate limiting metrics
	RateLimitExceeded = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "buildkite_rate_limit_exceeded_total",
			Help: "Total number of requests that exceeded rate limits",
		},
		[]string{"type"}, // "global" or "per_ip"
	)

	// Error metrics
	ErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "buildkite_errors_total",
			Help: "Total number of errors by type",
		},
		[]string{"type"},
	)
)
