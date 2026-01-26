package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestInitMetrics(t *testing.T) {
	tests := []struct {
		name      string
		registry  prometheus.Registerer
		wantError bool
	}{
		{
			name:      "fresh registry initializes successfully",
			registry:  prometheus.NewRegistry(),
			wantError: false,
		},
		{
			name:      "nil registry fails",
			registry:  nil,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := InitMetrics(tt.registry)
			if (err != nil) != tt.wantError {
				t.Errorf("InitMetrics() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestRecordDLQMessage(t *testing.T) {
	reg := prometheus.NewRegistry()
	if err := InitMetrics(reg); err != nil {
		t.Fatalf("failed to initialize metrics: %v", err)
	}

	RecordDLQMessage("build.finished", "publish_error")

	value := getCounterValue(t, DLQMessagesTotal.WithLabelValues("build.finished", "publish_error"))
	if value != 1 {
		t.Errorf("expected DLQMessagesTotal to be 1, got %v", value)
	}
}

func getCounterValue(t *testing.T, c prometheus.Counter) float64 {
	t.Helper()
	var metric dto.Metric
	if err := c.Write(&metric); err != nil {
		t.Fatalf("Error getting counter value: %v", err)
	}
	return metric.GetCounter().GetValue()
}
