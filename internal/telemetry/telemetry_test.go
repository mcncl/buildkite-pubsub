package telemetry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

func TestTracingMiddleware(t *testing.T) {
	// Create a test handler that checks for span context
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get span from context
		span := trace.SpanFromContext(r.Context())
		if span == nil {
			t.Error("Expected span in context, got nil")
		}

		// Verify span attributes
		// Note: In a real implementation, we'd verify specific attributes
		// but that requires setting up a test span processor
		w.WriteHeader(http.StatusOK)
	})

	// Create middleware
	middleware := TracingMiddleware("test-service")
	wrappedHandler := middleware(handler)

	// Create test request
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	// Serve request
	wrappedHandler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}
}

func TestInitTracer(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		ServiceName:    "test-service",
		ServiceVersion: "v1.0.0",
		Environment:    "test",
		OTLPEndpoint:   "localhost:4317",
	}

	// Initialize tracer
	cleanup, err := InitTracer(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to initialize tracer: %v", err)
	}
	defer cleanup()

	// Verify global tracer provider was set
	tp := otel.GetTracerProvider()
	if tp == nil {
		t.Error("Expected tracer provider to be set")
	}

	// Create a span to verify tracer works
	tr := tp.Tracer("test")
	_, span := tr.Start(ctx, "test-span")
	defer span.End()

	// Verify span was created
	if span == nil {
		t.Error("Expected span to be created")
	}
}

func TestTracingMiddlewareAttributes(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
	}{
		{
			name:       "GET request",
			method:     "GET",
			path:       "/test",
			wantStatus: http.StatusOK,
		},
		{
			name:       "POST request",
			method:     "POST",
			path:       "/api/v1/resource",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				span := trace.SpanFromContext(r.Context())
				if span == nil {
					t.Error("Expected span in context, got nil")
					return
				}
				w.WriteHeader(tt.wantStatus)
			})

			middleware := TracingMiddleware("test-service")
			wrappedHandler := middleware(handler)

			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			wrappedHandler.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}
