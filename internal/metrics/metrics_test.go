package metrics

import (
	"fmt"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

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

func TestConcurrentMetricRecording(t *testing.T) {
	reg := prometheus.NewRegistry()
	InitMetrics(reg)

	const numGoroutines = 100
	done := make(chan bool)

	// Record metrics concurrently
	for i := 0; i < numGoroutines; i++ {
		go func(i int) {
			defer func() {
				done <- true
			}()

			// Record different types of metrics concurrently
			RecordBuildStatus("passed", fmt.Sprintf("pipeline-%d", i))
			RecordPipelineBuild(fmt.Sprintf("pipeline-%d", i), "test-org")
			RecordQueueTime(fmt.Sprintf("pipeline-%d", i), float64(i))
			RecordMessageSize("build.started", i*1000)
			RecordPubsubMessageSize("build.started", i*1000)
			RecordPubsubRetry("build.started")
		}(i)
	}

	// Wait for all goroutines to finish
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify metrics were recorded correctly
	metrics, err := reg.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	// Verify metric counts
	for _, mf := range metrics {
		switch mf.GetName() {
		case "buildkite_build_status_total":
			if len(mf.GetMetric()) != numGoroutines {
				t.Errorf("Expected %d build status metrics, got %d", numGoroutines, len(mf.GetMetric()))
			}
		case "buildkite_pipeline_builds_total":
			if len(mf.GetMetric()) != numGoroutines {
				t.Errorf("Expected %d pipeline build metrics, got %d", numGoroutines, len(mf.GetMetric()))
			}
		}
	}
}

func TestMetricsBeforeInitialization(t *testing.T) {
	// Reset all metrics to nil
	WebhookRequestsTotal = nil
	WebhookRequestDuration = nil
	AuthFailures = nil
	// ... reset other metrics ...

	// Attempt to record metrics before initialization
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when recording metrics before initialization")
		}
	}()

	RecordBuildStatus("passed", "test-pipeline")
}

func TestMetricsRecording(t *testing.T) {
	// Initialize test registry
	reg := prometheus.NewRegistry()
	InitMetrics(reg)

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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset registry for each test
			reg = prometheus.NewRegistry()
			InitMetrics(reg)

			// Record metric
			tt.recordFunc()

			// Check metric value
			tt.checkFunc(t)
		})
	}
}

func TestMetricsInitialization(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func() prometheus.Registerer
		wantError bool
	}{
		{
			name: "fresh registry initializes successfully",
			setupFunc: func() prometheus.Registerer {
				return prometheus.NewRegistry()
			},
			wantError: false,
		},
		{
			name: "double initialization fails",
			setupFunc: func() prometheus.Registerer {
				reg := prometheus.NewRegistry()
				InitMetrics(reg)
				return reg
			},
			wantError: true,
		},
		{
			name: "nil registry fails",
			setupFunc: func() prometheus.Registerer {
				return nil
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					if !tt.wantError {
						t.Errorf("InitMetrics panicked unexpectedly: %v", r)
					}
				}
			}()

			reg := tt.setupFunc()
			InitMetrics(reg)

			if tt.wantError {
				t.Error("Expected error but got none")
			}
		})
	}
}
