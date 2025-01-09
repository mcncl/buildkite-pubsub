package metrics

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Metrics variables - these will be initialized by InitMetrics
	WebhookRequestsTotal       *prometheus.CounterVec
	WebhookRequestDuration     *prometheus.HistogramVec
	AuthFailures               prometheus.Counter
	PubsubPublishRequestsTotal *prometheus.CounterVec
	PubsubPublishDuration      prometheus.Histogram
	RateLimitExceeded          *prometheus.CounterVec
	ErrorsTotal                *prometheus.CounterVec
	MessageSizeBytes           *prometheus.HistogramVec
	PayloadProcessingDuration  *prometheus.HistogramVec
	BuildStatusTotal           *prometheus.CounterVec
	PipelineBuildsTotal        *prometheus.CounterVec
	QueueTimeSeconds           *prometheus.HistogramVec
	PubsubMessageSizeBytes     *prometheus.HistogramVec
	PubsubRetries              *prometheus.CounterVec
)

// InitMetrics initializes metrics with a specific registry
func InitMetrics(reg prometheus.Registerer) error {
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

	MessageSizeBytes = factory.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "buildkite_message_size_bytes",
			Help: "Size of webhook payload messages in bytes",
			Buckets: []float64{
				100, 500, 1000, 5000, 10000, 50000, 100000,
			},
		},
		[]string{"event_type"},
	)

	PayloadProcessingDuration = factory.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "buildkite_payload_processing_duration_seconds",
			Help:    "Time spent processing and transforming payloads",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"event_type"},
	)

	BuildStatusTotal = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "buildkite_build_status_total",
			Help: "Total number of builds by status",
		},
		[]string{"status", "pipeline"},
	)

	PipelineBuildsTotal = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "buildkite_pipeline_builds_total",
			Help: "Total number of builds per pipeline",
		},
		[]string{"pipeline", "organization"},
	)

	QueueTimeSeconds = factory.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "buildkite_queue_time_seconds",
			Help:    "Time builds spend in queue before starting",
			Buckets: prometheus.ExponentialBuckets(1, 2, 10),
		},
		[]string{"pipeline"},
	)

	PubsubMessageSizeBytes = factory.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "buildkite_pubsub_message_size_bytes",
			Help: "Size of messages published to Pub/Sub in bytes",
			Buckets: []float64{
				100, 500, 1000, 5000, 10000, 50000, 100000,
			},
		},
		[]string{"event_type"},
	)

	PubsubRetries = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "buildkite_pubsub_retries_total",
			Help: "Number of Pub/Sub publish retries",
		},
		[]string{"event_type"},
	)

	return nil
}

// Helper functions for recording metrics

// RecordBuildStatus records build status metrics
func RecordBuildStatus(status, pipeline string) {
	BuildStatusTotal.WithLabelValues(status, pipeline).Inc()
}

// RecordPipelineBuild records pipeline build metrics
func RecordPipelineBuild(pipeline, organization string) {
	PipelineBuildsTotal.WithLabelValues(pipeline, organization).Inc()
}

// RecordQueueTime records build queue time metrics
func RecordQueueTime(pipeline string, queueSeconds float64) {
	QueueTimeSeconds.WithLabelValues(pipeline).Observe(queueSeconds)
}

// RecordMessageSize records the size of a message
func RecordMessageSize(eventType string, sizeBytes int) {
	MessageSizeBytes.WithLabelValues(eventType).Observe(float64(sizeBytes))
}

// RecordPubsubMessageSize records the size of a published Pub/Sub message
func RecordPubsubMessageSize(eventType string, sizeBytes int) {
	PubsubMessageSizeBytes.WithLabelValues(eventType).Observe(float64(sizeBytes))
}

// RecordPubsubRetry records a Pub/Sub publish retry attempt
func RecordPubsubRetry(eventType string) {
	PubsubRetries.WithLabelValues(eventType).Inc()
}
