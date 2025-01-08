package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

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

func TestInitMetricsRegistration(t *testing.T) {
	// Test that metrics can be registered multiple times with different registries
	reg1 := prometheus.NewRegistry()
	InitMetrics(reg1)

	reg2 := prometheus.NewRegistry()
	InitMetrics(reg2)

	// Record some metrics with the second registry
	RecordBuildStatus("passed", "test-pipeline")

	// Verify metrics were recorded in reg2
	families, err := reg2.Gather()
	if err != nil {
		t.Fatalf("Error gathering metrics: %v", err)
	}

	var found bool
	for _, family := range families {
		if family.GetName() == "buildkite_build_status_total" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected to find buildkite_build_status_total metric in second registry")
	}
}