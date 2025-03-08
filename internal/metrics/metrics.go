package metrics

import (
	"fmt"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Webhook request metrics
	WebhookRequestsTotal       *prometheus.CounterVec    // Total number of webhook requests
	WebhookRequestDuration     *prometheus.HistogramVec  // Duration of webhook requests
	RequestSizeBytes           *prometheus.HistogramVec  // Size of incoming requests
	ResponseSizeBytes          *prometheus.HistogramVec  // Size of outgoing responses
	AuthFailures               prometheus.Counter        // Authentication failures
	RateLimitExceeded          *prometheus.CounterVec    // Rate limit exceeded events
	RateLimitTotal             *prometheus.CounterVec    // Total rate limit hits by type and endpoint
	ConcurrentRequests         *prometheus.GaugeVec      // Current number of concurrent requests
	ErrorsTotal                *prometheus.CounterVec    // Total errors by type
	
	// Message size metrics
	MessageSizeBytes           *prometheus.HistogramVec  // Size of webhook payload messages
	
	// Payload processing metrics
	PayloadProcessingDuration  *prometheus.HistogramVec  // Processing time for payloads
	
	// Build status metrics
	BuildStatusTotal           *prometheus.CounterVec    // Build status counts
	PipelineBuildsTotal        *prometheus.CounterVec    // Total builds per pipeline
	QueueTimeSeconds           *prometheus.HistogramVec  // Build queue time
	
	// Pub/Sub metrics
	PubsubPublishRequestsTotal *prometheus.CounterVec    // Pub/Sub publish attempts
	PubsubPublishDuration      prometheus.Histogram      // Pub/Sub publish latency
	PubsubMessageSizeBytes     *prometheus.HistogramVec  // Size of Pub/Sub messages
	PubsubRetries              *prometheus.CounterVec    // Pub/Sub retries
	PubsubBacklogSize          *prometheus.GaugeVec      // Current Pub/Sub backlog size
	PubsubConnectionPoolSize   *prometheus.GaugeVec      // Connection pool size
	PubsubBatchSize            prometheus.Histogram      // Size of batched messages
	
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

	// Webhook request metrics
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

	RequestSizeBytes = factory.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "buildkite_request_size_bytes",
			Help: "Size of incoming HTTP requests in bytes",
			Buckets: []float64{
				100, 500, 1000, 5000, 10000, 50000, 100000, 500000, 1000000,
			},
		},
		[]string{"method", "path"},
	)

	ResponseSizeBytes = factory.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "buildkite_response_size_bytes",
			Help: "Size of outgoing HTTP responses in bytes",
			Buckets: []float64{
				100, 500, 1000, 5000, 10000, 50000, 100000,
			},
		},
		[]string{"method", "path", "status"},
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

	RateLimitTotal = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "buildkite_rate_limit_total",
			Help: "Total rate limit hits by type and endpoint",
		},
		[]string{"limiter_type", "endpoint"},
	)

	ConcurrentRequests = factory.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "buildkite_concurrent_requests",
			Help: "Current number of concurrent requests by endpoint",
		},
		[]string{"endpoint"},
	)

	ErrorsTotal = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "buildkite_errors_total",
			Help: "Total number of errors by type",
		},
		[]string{"type"},
	)

	// Message size metrics
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

	// Payload processing metrics
	PayloadProcessingDuration = factory.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "buildkite_payload_processing_duration_seconds",
			Help:    "Time spent processing and transforming payloads",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"event_type"},
	)

	// Build status metrics
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

	// Pub/Sub metrics
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

	PubsubBacklogSize = factory.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "buildkite_pubsub_backlog_size",
			Help: "Current size of messages in the Pub/Sub publishing queue",
		},
		[]string{"topic"},
	)

	PubsubConnectionPoolSize = factory.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "buildkite_pubsub_connection_pool_size",
			Help: "Size of the Pub/Sub connection pool",
		},
		[]string{"type"}, // "max" or "active"
	)

	PubsubBatchSize = factory.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "buildkite_pubsub_batch_size",
			Help:    "Number of messages in each Pub/Sub batch",
			Buckets: []float64{1, 5, 10, 20, 50, 100, 200, 500},
		},
	)

	return nil
}

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

// New helper functions for enhanced metrics

// RecordRequestSize records the size of an incoming HTTP request
func RecordRequestSize(method, path string, sizeBytes int) {
	RequestSizeBytes.WithLabelValues(method, path).Observe(float64(sizeBytes))
}

// RecordResponseSize records the size of an outgoing HTTP response
func RecordResponseSize(method, path string, statusCode int, sizeBytes int) {
	ResponseSizeBytes.WithLabelValues(method, path, fmt.Sprintf("%d", statusCode)).Observe(float64(sizeBytes))
}

// RecordPubsubBacklogSize records the current Pub/Sub backlog size
func RecordPubsubBacklogSize(topic string, size int) {
	PubsubBacklogSize.WithLabelValues(topic).Set(float64(size))
}

// RecordPubsubConnectionPoolSize records the connection pool sizes
func RecordPubsubConnectionPoolSize(max, active int) {
	PubsubConnectionPoolSize.WithLabelValues("max").Set(float64(max))
	PubsubConnectionPoolSize.WithLabelValues("active").Set(float64(active))
}

// RecordRateLimit records rate limit hits
func RecordRateLimit(limiterType, endpoint string) {
	RateLimitTotal.WithLabelValues(limiterType, endpoint).Inc()
}

// IncrementConcurrentRequests increments the concurrent requests gauge
func IncrementConcurrentRequests(endpoint string) {
	ConcurrentRequests.WithLabelValues(endpoint).Inc()
}

// DecrementConcurrentRequests decrements the concurrent requests gauge
func DecrementConcurrentRequests(endpoint string) {
	ConcurrentRequests.WithLabelValues(endpoint).Dec()
}

// RecordPubsubBatchSize records the size of a Pub/Sub batch
func RecordPubsubBatchSize(batchSize int) {
	PubsubBatchSize.Observe(float64(batchSize))
}
