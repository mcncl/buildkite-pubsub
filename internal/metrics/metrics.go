package metrics

import (
	"fmt"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Webhook request metrics
	WebhookRequestsTotal   *prometheus.CounterVec
	WebhookRequestDuration *prometheus.HistogramVec
	AuthFailures           prometheus.Counter
	RateLimitExceeded      *prometheus.CounterVec
	ErrorsTotal            *prometheus.CounterVec

	// Payload processing metrics
	PayloadProcessingDuration *prometheus.HistogramVec

	// Pub/Sub metrics
	PubsubPublishRequestsTotal *prometheus.CounterVec
	PubsubPublishDuration      prometheus.Histogram
	PubsubRetries              *prometheus.CounterVec

	// Dead Letter Queue metrics
	DLQMessagesTotal *prometheus.CounterVec

	// Mutex to protect metric initialization
	initMutex sync.Mutex
)

// InitMetrics initializes metrics with a specific registry
func InitMetrics(reg prometheus.Registerer) error {
	initMutex.Lock()
	defer initMutex.Unlock()

	if reg == nil {
		return fmt.Errorf("registry cannot be nil")
	}

	factory := promauto.With(reg)

	WebhookRequestsTotal = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "buildkite_webhook_requests_total",
			Help: "Total number of webhook requests received",
		},
		[]string{"status", "event_type"},
	)

	WebhookRequestDuration = factory.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "buildkite_webhook_request_duration_seconds",
			Help:    "Duration of webhook requests in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"event_type"},
	)

	AuthFailures = factory.NewCounter(
		prometheus.CounterOpts{
			Name: "buildkite_webhook_auth_failures_total",
			Help: "Total number of authentication failures",
		},
	)

	RateLimitExceeded = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "buildkite_rate_limit_exceeded_total",
			Help: "Total number of requests that exceeded rate limits",
		},
		[]string{"type"},
	)

	ErrorsTotal = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "buildkite_errors_total",
			Help: "Total number of errors by type",
		},
		[]string{"type"},
	)

	PayloadProcessingDuration = factory.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "buildkite_payload_processing_duration_seconds",
			Help:    "Time spent processing and transforming payloads",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"event_type"},
	)

	PubsubPublishRequestsTotal = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "buildkite_pubsub_publish_requests_total",
			Help: "Total number of Pub/Sub publish requests",
		},
		[]string{"status", "event_type"},
	)

	PubsubPublishDuration = factory.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "buildkite_pubsub_publish_duration_seconds",
			Help:    "Duration of Pub/Sub publish operations in seconds",
			Buckets: prometheus.DefBuckets,
		},
	)

	PubsubRetries = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "buildkite_pubsub_retries_total",
			Help: "Number of Pub/Sub publish retries",
		},
		[]string{"event_type"},
	)

	DLQMessagesTotal = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "buildkite_dlq_messages_total",
			Help: "Total number of messages sent to the Dead Letter Queue",
		},
		[]string{"event_type", "failure_reason"},
	)

	return nil
}

// RecordMessageSize records the size of a message (kept for handler.go compatibility)
func RecordMessageSize(eventType string, sizeBytes int) {
	// No-op: metric removed but function kept for compatibility
}

// RecordPubsubMessageSize records the size of a published Pub/Sub message
func RecordPubsubMessageSize(eventType string, sizeBytes int) {
	// No-op: metric removed but function kept for compatibility
}

// RecordPubsubRetry records a Pub/Sub publish retry attempt
func RecordPubsubRetry(eventType string) {
	PubsubRetries.WithLabelValues(eventType).Inc()
}

// RecordDLQMessage records a message sent to the Dead Letter Queue
func RecordDLQMessage(eventType, failureReason string) {
	DLQMessagesTotal.WithLabelValues(eventType, failureReason).Inc()
}

// RecordBuildStatus is a no-op (metric removed)
func RecordBuildStatus(status, pipeline string) {}

// RecordPipelineBuild is a no-op (metric removed)
func RecordPipelineBuild(pipeline, organization string) {}

// RecordQueueTime is a no-op (metric removed)
func RecordQueueTime(pipeline string, queueSeconds float64) {}
