package request

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWithRequestID(t *testing.T) {
	tests := []struct {
		name          string
		providedID    string
		wantIDPresent bool
	}{
		{
			name:          "adds request ID when none provided",
			providedID:    "",
			wantIDPresent: true,
		},
		{
			name:          "uses provided request ID",
			providedID:    "test-id-123",
			wantIDPresent: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := WithRequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Check context
				id := r.Context().Value(RequestIDKey)
				if id == nil {
					t.Error("requestID not found in context")
				}

				// Check header
				if got := w.Header().Get(RequestIDHeader); got == "" {
					t.Error("X-Request-ID header not set")
				}

				// If ID was provided, verify it's used
				if tt.providedID != "" && id != tt.providedID {
					t.Errorf("got request ID %v, want %v", id, tt.providedID)
				}
			}))

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.providedID != "" {
				req.Header.Set(RequestIDHeader, tt.providedID)
			}

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
		})
	}
}
