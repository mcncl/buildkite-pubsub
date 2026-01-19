# Distributed Tracing

The service supports OpenTelemetry tracing to observe request flow through the webhook handler.

## Environment Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `ENABLE_TRACING` | Enable tracing | `true` |
| `OTEL_SERVICE_NAME` | Service name | `buildkite-webhook` |
| `OTEL_ENVIRONMENT` | Environment label | `production` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | Collector endpoint | `https://api.honeycomb.io` |
| `OTEL_EXPORTER_OTLP_HEADERS` | Auth headers | `x-honeycomb-team=API_KEY` |

## Honeycomb Setup

1. Get an API key from [Honeycomb](https://honeycomb.io) → Team Settings → API Keys
2. Set environment variables:

```bash
ENABLE_TRACING=true
OTEL_SERVICE_NAME=buildkite-webhook
OTEL_ENVIRONMENT=production
OTEL_EXPORTER_OTLP_ENDPOINT=https://api.honeycomb.io
OTEL_EXPORTER_OTLP_HEADERS=x-honeycomb-team=YOUR_API_KEY
```

## Jaeger (Local Development)

```bash
# Start Jaeger
docker run -d --name jaeger \
  -p 16686:16686 \
  -p 4317:4317 \
  jaegertracing/all-in-one:latest

# Configure service
ENABLE_TRACING=true
OTEL_SERVICE_NAME=buildkite-webhook
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317
```

View traces at http://localhost:16686

## Spans

The service creates spans for:
- `POST /webhook` - HTTP request with method, status, duration
- `transform_payload` - Payload processing with event type
- `pubsub_publish` - Message publishing with pipeline attributes
