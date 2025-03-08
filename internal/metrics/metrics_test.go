package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestMetricsRecording(t *testing.T) {
	// Initialize test registry
	reg := prometheus.NewRegistry()
	err := InitMetrics(reg)
	if err != nil {
		t.Fatalf("failed to initialize metrics: %v", err)
	}

	tests := []struct {
		name       string
		recordFunc func()
		checkFunc  func(t *testing.T)
	}{
		{
			name: "BuildStatusTotal increments correctly",
			recordFunc: func() {
				RecordBuildStatus("passed", "test-pipeline")
			},
			checkFunc: func(t *testing.T) {
				value := getCounterValue(t, BuildStatusTotal.WithLabelValues("passed", "test-pipeline"))
				if value != 1 {
					t.Errorf("expected BuildStatusTotal to be 1, got %v", value)
				}
			},
		},
		{
			name: "PipelineBuildsTotal increments correctly",
			recordFunc: func() {
				RecordPipelineBuild("test-pipeline", "test-org")
			},
			checkFunc: func(t *testing.T) {
				value := getCounterValue(t, PipelineBuildsTotal.WithLabelValues("test-pipeline", "test-org"))
				if value != 1 {
					t.Errorf("expected PipelineBuildsTotal to be 1, got %v", value)
				}
			},
		},
		{
			name: "QueueTimeSeconds observes correctly",
			recordFunc: func() {
				RecordQueueTime("test-pipeline", 10.5)
			},
			checkFunc: func(t *testing.T) {
				histogram := getHistogramValue(t, QueueTimeSeconds.WithLabelValues("test-pipeline"))
				if histogram.GetSampleCount() != 1 {
					t.Errorf("expected QueueTimeSeconds sample count to be 1, got %v", histogram.GetSampleCount())
				}
				if histogram.GetSampleSum() != 10.5 {
					t.Errorf("expected QueueTimeSeconds sample sum to be 10.5, got %v", histogram.GetSampleSum())
				}
			},
		},
		{
			name: "MessageSizeBytes observes correctly",
			recordFunc: func() {
				RecordMessageSize("build.started", 1000)
			},
			checkFunc: func(t *testing.T) {
				histogram := getHistogramValue(t, MessageSizeBytes.WithLabelValues("build.started"))
				if histogram.GetSampleCount() != 1 {
					t.Errorf("expected MessageSizeBytes sample count to be 1, got %v", histogram.GetSampleCount())
				}
				if histogram.GetSampleSum() != 1000 {
					t.Errorf("expected MessageSizeBytes sample sum to be 1000, got %v", histogram.GetSampleSum())
				}
			},
		},
		{
			name: "PubsubMessageSizeBytes observes correctly",
			recordFunc: func() {
				RecordPubsubMessageSize("build.started", 2000)
			},
			checkFunc: func(t *testing.T) {
				histogram := getHistogramValue(t, PubsubMessageSizeBytes.WithLabelValues("build.started"))
				if histogram.GetSampleCount() != 1 {
					t.Errorf("expected PubsubMessageSizeBytes sample count to be 1, got %v", histogram.GetSampleCount())
				}
				if histogram.GetSampleSum() != 2000 {
					t.Errorf("expected PubsubMessageSizeBytes sample sum to be 2000, got %v", histogram.GetSampleSum())
				}
			},
		},
		{
			name: "PubsubRetries increments correctly",
			recordFunc: func() {
				RecordPubsubRetry("build.started")
			},
			checkFunc: func(t *testing.T) {
				value := getCounterValue(t, PubsubRetries.WithLabelValues("build.started"))
				if value != 1 {
					t.Errorf("expected PubsubRetries to be 1, got %v", value)
				}
			},
		},
		// New tests for enhanced metrics
		{
			name: "RequestSizeBytes observes correctly",
			recordFunc: func() {
				RecordRequestSize("POST", "/webhook", 5000)
			},
			checkFunc: func(t *testing.T) {
				histogram := getHistogramValue(t, RequestSizeBytes.WithLabelValues("POST", "/webhook"))
				if histogram.GetSampleCount() != 1 {
					t.Errorf("expected RequestSizeBytes sample count to be 1, got %v", histogram.GetSampleCount())
				}
				if histogram.GetSampleSum() != 5000 {
					t.Errorf("expected RequestSizeBytes sample sum to be 5000, got %v", histogram.GetSampleSum())
				}
			},
		},
		{
			name: "ResponseSizeBytes observes correctly",
			recordFunc: func() {
				RecordResponseSize("POST", "/webhook", 200, 1024)
			},
			checkFunc: func(t *testing.T) {
				histogram := getHistogramValue(t, ResponseSizeBytes.WithLabelValues("POST", "/webhook", "200"))
				if histogram.GetSampleCount() != 1 {
					t.Errorf("expected ResponseSizeBytes sample count to be 1, got %v", histogram.GetSampleCount())
				}
				if histogram.GetSampleSum() != 1024 {
					t.Errorf("expected ResponseSizeBytes sample sum to be 1024, got %v", histogram.GetSampleSum())
				}
			},
		},
		{
			name: "PubsubBacklogSize records correctly",
			recordFunc: func() {
				RecordPubsubBacklogSize("buildkite-events", 5)
			},
			checkFunc: func(t *testing.T) {
				gauge := getGaugeValue(t, PubsubBacklogSize.WithLabelValues("buildkite-events"))
				if gauge != 5 {
					t.Errorf("expected PubsubBacklogSize to be 5, got %v", gauge)
				}
			},
		},
		{
			name: "PubsubConnectionPoolSize records correctly",
			recordFunc: func() {
				RecordPubsubConnectionPoolSize(10, 5)
			},
			checkFunc: func(t *testing.T) {
				maxConn := getGaugeValue(t, PubsubConnectionPoolSize.WithLabelValues("max"))
				activeConn := getGaugeValue(t, PubsubConnectionPoolSize.WithLabelValues("active"))
				if maxConn != 10 {
					t.Errorf("expected max PubsubConnectionPoolSize to be 10, got %v", maxConn)
				}
				if activeConn != 5 {
					t.Errorf("expected active PubsubConnectionPoolSize to be 5, got %v", activeConn)
				}
			},
		},
		{
			name: "RateLimitTotal increments correctly",
			recordFunc: func() {
				RecordRateLimit("global", "webhook")
			},
			checkFunc: func(t *testing.T) {
				value := getCounterValue(t, RateLimitTotal.WithLabelValues("global", "webhook"))
				if value != 1 {
					t.Errorf("expected RateLimitTotal to be 1, got %v", value)
				}
			},
		},
		{
			name: "ConcurrentRequests gauge works correctly",
			recordFunc: func() {
				IncrementConcurrentRequests("webhook")
				IncrementConcurrentRequests("webhook")
				DecrementConcurrentRequests("webhook")
			},
			checkFunc: func(t *testing.T) {
				gauge := getGaugeValue(t, ConcurrentRequests.WithLabelValues("webhook"))
				if gauge != 1 {
					t.Errorf("expected ConcurrentRequests to be 1, got %v", gauge)
				}
			},
		},
		{
			name: "PubsubBatchSize records correctly",
			recordFunc: func() {
				RecordPubsubBatchSize(15)
			},
			checkFunc: func(t *testing.T) {
				histogram := getHistogramValue(t, PubsubBatchSize)
				if histogram.GetSampleCount() != 1 {
					t.Errorf("expected PubsubBatchSize sample count to be 1, got %v", histogram.GetSampleCount())
				}
				if histogram.GetSampleSum() != 15 {
					t.Errorf("expected PubsubBatchSize sample sum to be 15, got %v", histogram.GetSampleSum())
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset registry for each test
			reg = prometheus.NewRegistry()
			err := InitMetrics(reg)
			if err != nil {
				t.Fatalf("failed to initialize metrics: %v", err)
			}

			// Record metric
			tt.recordFunc()

			// Check metric value
			tt.checkFunc(t)
		})
	}
}

// Helper function to get counter value
func getCounterValue(t *testing.T, c prometheus.Counter) float64 {
	t.Helper()
	var metric dto.Metric
	if err := c.Write(&metric); err != nil {
		t.Fatalf("Error getting counter value: %v", err)
	}
	return metric.GetCounter().GetValue()
}

// Helper function to get histogram value
func getHistogramValue(t *testing.T, h prometheus.Observer) *dto.Histogram {
	t.Helper()
	var metric dto.Metric
	if err := h.(prometheus.Metric).Write(&metric); err != nil {
		t.Fatalf("Error getting histogram value: %v", err)
	}
	return metric.GetHistogram()
}

// Helper function to get gauge value
func getGaugeValue(t *testing.T, g prometheus.Gauge) float64 {
	t.Helper()
	var metric dto.Metric
	if err := g.(prometheus.Metric).Write(&metric); err != nil {
		t.Fatalf("Error getting gauge value: %v", err)
	}
	return metric.GetGauge().GetValue()
}

func TestMetricsInitialization(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func() (prometheus.Registerer, error)
		wantError bool
	}{
		{
			name: "fresh registry initializes successfully",
			setupFunc: func() (prometheus.Registerer, error) {
				return prometheus.NewRegistry(), nil
			},
			wantError: false,
		},
		{
			name: "nil registry fails",
			setupFunc: func() (prometheus.Registerer, error) {
				return nil, nil
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg, _ := tt.setupFunc()
			err := InitMetrics(reg)
			if (err != nil) != tt.wantError {
				t.Errorf("InitMetrics() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestMetricsLabels(t *testing.T) {
	// Initialize test registry
	reg := prometheus.NewRegistry()
	err := InitMetrics(reg)
	if err != nil {
		t.Fatalf("failed to initialize metrics: %v", err)
	}

	// Test that metrics with multiple label sets get recorded correctly
	RecordBuildStatus("passed", "pipeline1")
	RecordBuildStatus("failed", "pipeline1")
	RecordBuildStatus("passed", "pipeline2")

	// Get metrics from registry
	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	// Find BuildStatusTotal metrics
	var buildStatusMetric *dto.MetricFamily
	for _, family := range families {
		if family.GetName() == "buildkite_build_status_total" {
			buildStatusMetric = family
			break
		}
	}

	if buildStatusMetric == nil {
		t.Fatal("buildkite_build_status_total metric not found")
	}

	// Check we have 3 different label combinations
	if len(buildStatusMetric.Metric) != 3 {
		t.Errorf("Expected 3 different label sets for BuildStatusTotal, got %d", 
			len(buildStatusMetric.Metric))
	}
}
