package telemetry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestProviderLifecycle(t *testing.T) {
	cfg := Config{
		ServiceName:    "test-service",
		ServiceVersion: "v1.0.0",
		Environment:    "test",
		OTLPEndpoint:   "localhost:4317",
	}

	// Create provider
	provider, err := NewProvider(cfg)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}

	// Test double initialization
	ctx := context.Background()
	if err := provider.Start(ctx); err != nil {
		t.Fatalf("First Start() error = %v", err)
	}

	if err := provider.Start(ctx); err == nil {
		t.Error("Second Start() should fail")
	}

	// Test shutdown
	if err := provider.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown() error = %v", err)
	}

	// Test double shutdown
	if err := provider.Shutdown(ctx); err != nil {
		t.Error("Second Shutdown() should not return error")
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name      string
		config    Config
		wantError bool
	}{
		{
			name: "valid config",
			config: Config{
				ServiceName:    "test-service",
				ServiceVersion: "v1.0.0",
				Environment:    "test",
				OTLPEndpoint:   "localhost:4317",
			},
			wantError: false,
		},
		{
			name: "empty service name",
			config: Config{
				ServiceVersion: "v1.0.0",
				Environment:    "test",
				OTLPEndpoint:   "localhost:4317",
			},
			wantError: true,
		},
		{
			name: "empty endpoint",
			config: Config{
				ServiceName:    "test-service",
				ServiceVersion: "v1.0.0",
				Environment:    "test",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewProvider(tt.config)
			if (err != nil) != tt.wantError {
				t.Errorf("NewProvider() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestTracingMiddleware(t *testing.T) {
	// Setup a mock OTLP server
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := Config{
		ServiceName:    "test-service",
		ServiceVersion: "v1.0.0",
		Environment:    "test",
		OTLPEndpoint:   srv.Listener.Addr().String(),
	}

	provider, err := NewProvider(cfg)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}

	ctx := context.Background()
	if err := provider.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer provider.Shutdown(ctx)

	tests := []struct {
		name          string
		method        string
		path          string
		handlerStatus int
	}{
		{
			name:          "success response",
			method:        "GET",
			path:          "/test",
			handlerStatus: http.StatusOK,
		},
		{
			name:          "error response",
			method:        "POST",
			path:          "/test",
			handlerStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify we have a context with span
				span := r.Context().Value("trace-context")
				if span == nil && provider.isInit {
					t.Error("Expected span in context when provider is initialized")
				}
				w.WriteHeader(tt.handlerStatus)
			})

			// Create middleware
			wrappedHandler := provider.TracingMiddleware(handler)

			// Create request
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			// Serve request
			wrappedHandler.ServeHTTP(w, req)

			if w.Code != tt.handlerStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.handlerStatus)
			}
		})
	}
}

func TestResponseWriter(t *testing.T) {
	tests := []struct {
		name        string
		writeCode   int
		writeBody   string
		writeHeader string
	}{
		{
			name:        "success response",
			writeCode:   http.StatusOK,
			writeBody:   "success",
			writeHeader: "text/plain",
		},
		{
			name:        "error response",
			writeCode:   http.StatusBadRequest,
			writeBody:   "error",
			writeHeader: "application/json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			rw := newResponseWriter(w)

			if tt.writeHeader != "" {
				rw.Header().Set("Content-Type", tt.writeHeader)
			}

			rw.WriteHeader(tt.writeCode)
			if rw.statusCode != tt.writeCode {
				t.Errorf("got status %d, want %d", rw.statusCode, tt.writeCode)
			}

			if tt.writeBody != "" {
				rw.Write([]byte(tt.writeBody))
				if w.Body.String() != tt.writeBody {
					t.Errorf("got body %q, want %q", w.Body.String(), tt.writeBody)
				}
			}

			if tt.writeHeader != "" {
				if got := w.Header().Get("Content-Type"); got != tt.writeHeader {
					t.Errorf("got Content-Type %q, want %q", got, tt.writeHeader)
				}
			}
		})
	}
}

func TestConcurrentAccess(t *testing.T) {
	cfg := Config{
		ServiceName:    "test-service",
		ServiceVersion: "v1.0.0",
		Environment:    "test",
		OTLPEndpoint:   "localhost:4317",
	}

	provider, err := NewProvider(cfg)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}

	ctx := context.Background()
	if err := provider.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Run concurrent operations
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- true }()

			// Create and use middleware
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(10 * time.Millisecond) // Simulate work
				w.WriteHeader(http.StatusOK)
			})

			wrapped := provider.TracingMiddleware(handler)
			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()
			wrapped.ServeHTTP(w, req)
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Clean up
	if err := provider.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown() error = %v", err)
	}
}
