package telemetry

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
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

// NewProvider creates a new telemetry provider
func NewProvider(cfg Config) (*Provider, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &Provider{
		config: cfg,
	}, nil
}

// Start initializes the telemetry provider
func (p *Provider) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.isInit {
		return fmt.Errorf("provider already initialized")
	}

	// Create OTLP exporter
	client := otlptracegrpc.NewClient(
		otlptracegrpc.WithEndpoint(p.config.OTLPEndpoint),
		otlptracegrpc.WithInsecure(),
	)

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
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(0.1))),
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
