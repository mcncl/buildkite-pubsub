package logging

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mcncl/buildkite-pubsub/internal/middleware/request"
)

func TestWithLogging(t *testing.T) {
	tests := []struct {
		name          string
		requestID     string
		method        string
		path          string
		status        int
		wantInLog     []string
		dontWantInLog []string
	}{
		{
			name:      "logs successful request",
			requestID: "test-id",
			method:    http.MethodGet,
			path:      "/test",
			status:    http.StatusOK,
			wantInLog: []string{
				"test-id",
				"GET",
				"/test",
				"200",
			},
		},
		{
			name:      "logs error request",
			requestID: "error-id",
			method:    http.MethodPost,
			path:      "/error",
			status:    http.StatusBadRequest,
			wantInLog: []string{
				"error-id",
				"POST",
				"/error",
				"400",
			},
		},
		{
			name:   "handles missing request ID",
			method: http.MethodGet,
			path:   "/test",
			status: http.StatusOK,
			wantInLog: []string{
				"unknown",
				"GET",
				"/test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture log output
			var buf bytes.Buffer
			log.SetOutput(&buf)

			handler := WithLogging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
			}))

			req := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.requestID != "" {
				ctx := context.WithValue(req.Context(), request.RequestIDKey, tt.requestID)
				req = req.WithContext(ctx)
			}

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			logOutput := buf.String()

			// Check required elements are in log
			for _, want := range tt.wantInLog {
				if !strings.Contains(logOutput, want) {
					t.Errorf("log output missing %q", want)
				}
			}

			// Check unwanted elements are not in log
			for _, dontWant := range tt.dontWantInLog {
				if strings.Contains(logOutput, dontWant) {
					t.Errorf("log output should not contain %q", dontWant)
				}
			}
		})
	}
}

func TestGetRequestID(t *testing.T) {
	tests := []struct {
		name      string
		requestID string
		want      string
	}{
		{
			name:      "returns existing request ID",
			requestID: "test-id",
			want:      "test-id",
		},
		{
			name: "returns unknown for missing request ID",
			want: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.requestID != "" {
				ctx := context.WithValue(req.Context(), request.RequestIDKey, tt.requestID)
				req = req.WithContext(ctx)
			}

			got := getRequestID(req)
			if got != tt.want {
				t.Errorf("getRequestID() = %v, want %v", got, tt.want)
			}
		})
	}
}
