package webhook

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
)

type HealthCheck struct {
	isReady *atomic.Bool
}

func NewHealthCheck() *HealthCheck {
	ready := &atomic.Bool{}
	ready.Store(false)
	return &HealthCheck{
		isReady: ready,
	}
}

func (h *HealthCheck) HealthHandler(w http.ResponseWriter, r *http.Request) {
	response := map[string]string{
		"status": "healthy",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *HealthCheck) ReadyHandler(w http.ResponseWriter, r *http.Request) {
	if !h.isReady.Load() {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	response := map[string]string{
		"status": "ready",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// SetReady marks the service as ready to receive traffic
func (h *HealthCheck) SetReady(ready bool) {
	h.isReady.Store(ready)
}
