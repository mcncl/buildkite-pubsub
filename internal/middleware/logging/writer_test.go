package logging

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResponseWriter(t *testing.T) {
	tests := []struct {
		name        string
		writeStatus int
		wantStatus  int
	}{
		{
			name:        "captures OK status",
			writeStatus: http.StatusOK,
			wantStatus:  http.StatusOK,
		},
		{
			name:        "captures error status",
			writeStatus: http.StatusBadRequest,
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "defaults to 200",
			writeStatus: 0,
			wantStatus:  http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rw := NewResponseWriter(httptest.NewRecorder())

			if tt.writeStatus != 0 {
				rw.WriteHeader(tt.writeStatus)
			}

			if rw.Status() != tt.wantStatus {
				t.Errorf("got status %d, want %d", rw.Status(), tt.wantStatus)
			}
		})
	}
}

func TestResponseWriterImplementsInterface(t *testing.T) {
	var _ http.ResponseWriter = &ResponseWriter{}
}
