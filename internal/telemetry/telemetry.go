package telemetry

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/credentials"
)

// Provider wraps the OpenTelemetry trace provider and exporter
type Provider struct {
	tp     *sdktrace.TracerProvider
	exp    *otlptrace.Exporter
	config Config
	mu     sync.RWMutex
	isInit bool
}

// Config holds configuration for telemetry setup
type Config struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
	OTLPEndpoint   string
	OTLPHeaders    map[string]string
	BatchTimeout   int // seconds
	ExportTimeout  int // seconds
	MaxExportBatch int
	MaxQueueSize   int
}

// DefaultConfig returns a Config with reasonable defaults
func DefaultConfig() Config {
	return Config{
		BatchTimeout:   5,    // 5 seconds
		ExportTimeout:  30,   // 30 seconds
		MaxExportBatch: 512,  // 512 spans
		MaxQueueSize:   2048, // 2048 spans
	}
}

// Validate checks if the configuration is valid
func (c Config) Validate() error {
	if c.ServiceName == "" {
		return fmt.Errorf("service name cannot be empty")
	}
	if c.OTLPEndpoint == "" {
		return fmt.Errorf("OTLP endpoint cannot be empty")
	}
	return nil
}

// ConfigFromEnv creates a Config from standard OpenTelemetry environment variables
func ConfigFromEnv() Config {
	cfg := DefaultConfig()

	// Service name from OTEL_SERVICE_NAME or fallback
	if serviceName := os.Getenv("OTEL_SERVICE_NAME"); serviceName != "" {
		cfg.ServiceName = serviceName
	}

	// OTLP endpoint from OTEL_EXPORTER_OTLP_ENDPOINT
	if endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); endpoint != "" {
		cfg.OTLPEndpoint = endpoint
	}

	// Parse headers from OTEL_EXPORTER_OTLP_HEADERS
	if headers := os.Getenv("OTEL_EXPORTER_OTLP_HEADERS"); headers != "" {
		cfg.OTLPHeaders = parseHeaders(headers)
	}

	return cfg
}

// parseHeaders parses header string like "key1=value1,key2=value2"
func parseHeaders(headerStr string) map[string]string {
	headers := make(map[string]string)
	pairs := strings.Split(headerStr, ",")

	for _, pair := range pairs {
		if kv := strings.SplitN(strings.TrimSpace(pair), "=", 2); len(kv) == 2 {
			headers[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}

	return headers
}

// NewProvider creates a new telemetry provider
func NewProvider(cfg Config) (*Provider, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &Provider{
		config: cfg,
	}, nil
}

// Start initializes the telemetry provider with timeout
func (p *Provider) Start(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	// Add timeout to context
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.isInit {
		return fmt.Errorf("provider already initialized")
	}

	// Create OTLP exporter
	endpoint := p.config.OTLPEndpoint

	// Handle HTTPS URLs by extracting hostname and using proper port
	if strings.HasPrefix(endpoint, "https://") {
		endpoint = strings.TrimPrefix(endpoint, "https://")
		if !strings.Contains(endpoint, ":") {
			endpoint = endpoint + ":443"
		}
	} else if strings.HasPrefix(endpoint, "http://") {
		endpoint = strings.TrimPrefix(endpoint, "http://")
		if !strings.Contains(endpoint, ":") {
			endpoint = endpoint + ":80"
		}
	}

	clientOptions := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithTimeout(5 * time.Second),
	}

	// Add headers if provided (for Honeycomb authentication)
	if len(p.config.OTLPHeaders) > 0 {
		clientOptions = append(clientOptions, otlptracegrpc.WithHeaders(p.config.OTLPHeaders))
	}

	// Determine if we should use TLS
	if strings.Contains(p.config.OTLPEndpoint, "api.honeycomb.io") || strings.HasPrefix(p.config.OTLPEndpoint, "https://") {
		// Use TLS for Honeycomb and HTTPS endpoints
		clientOptions = append(clientOptions, otlptracegrpc.WithTLSCredentials(credentials.NewClientTLSFromCert(nil, "")))
	} else {
		// Use insecure for localhost/development
		clientOptions = append(clientOptions, otlptracegrpc.WithInsecure())
	}

	client := otlptracegrpc.NewClient(clientOptions...)

	exp, err := otlptrace.New(ctx, client)
	if err != nil {
		return fmt.Errorf("creating OTLP trace exporter: %w", err)
	}
	p.exp = exp

	// Create resource
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(p.config.ServiceName),
			semconv.ServiceVersionKey.String(p.config.ServiceVersion),
			attribute.String("environment", p.config.Environment),
		),
	)
	if err != nil {
		return fmt.Errorf("creating resource: %w", err)
	}

	// Create trace provider
	p.tp = sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(p.exp,
			sdktrace.WithMaxExportBatchSize(p.config.MaxExportBatch),
			sdktrace.WithMaxQueueSize(p.config.MaxQueueSize),
		),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	// Set global trace provider
	otel.SetTracerProvider(p.tp)
	p.isInit = true

	return nil
}

// Shutdown stops the telemetry provider
func (p *Provider) Shutdown(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.isInit {
		return nil
	}

	var errs []error

	if err := p.tp.Shutdown(ctx); err != nil {
		errs = append(errs, fmt.Errorf("shutting down trace provider: %w", err))
	}

	if err := p.exp.Shutdown(ctx); err != nil {
		errs = append(errs, fmt.Errorf("shutting down exporter: %w", err))
	}

	p.isInit = false

	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}
	return nil
}

// TracingMiddleware wraps an http.Handler with OpenTelemetry tracing
func (p *Provider) TracingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !p.isInit {
			next.ServeHTTP(w, r)
			return
		}

		tracer := p.tp.Tracer(p.config.ServiceName)
		ctx, span := tracer.Start(r.Context(),
			fmt.Sprintf("%s %s", r.Method, r.URL.Path),
			trace.WithAttributes(
				semconv.HTTPMethodKey.String(r.Method),
				semconv.HTTPRouteKey.String(r.URL.Path),
				semconv.HTTPTargetKey.String(r.URL.Path),
			),
		)
		defer span.End()

		// Add the span context to the request context
		r = r.WithContext(ctx)

		// Wrap the response writer to capture status code
		wrapped := newResponseWriter(w)
		next.ServeHTTP(wrapped, r)

		// Record response status
		span.SetAttributes(semconv.HTTPStatusCodeKey.Int(wrapped.statusCode))
	})
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{w, http.StatusOK}
}

func (w *responseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}
