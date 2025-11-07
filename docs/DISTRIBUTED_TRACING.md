# Distributed Tracing Setup

This guide explains how to set up distributed tracing for the buildkite-pubsub webhook service using OpenTelemetry and various observability platforms.

## Table of Contents
1. [Overview](#overview)
2. [Honeycomb Setup](#honeycomb-setup)
3. [Jaeger Setup](#jaeger-setup)
4. [Cloud Run Configuration](#cloud-run-configuration)
5. [Local Development](#local-development)
6. [Troubleshooting](#troubleshooting)

## Overview

The buildkite-pubsub service supports distributed tracing using OpenTelemetry. Traces show the complete request flow:

- `POST /webhook` - HTTP request span with method, status, duration
- `transform_payload` - Payload processing with event type and build ID attributes
- `pubsub_publish` - Message publishing with pipeline and organization attributes

## Honeycomb Setup

### 1. Get API Key
1. Sign in to [Honeycomb](https://honeycomb.io)
2. Go to **Team Settings** → **API Keys**
3. Create a new API key with **Send Events** permissions
4. Copy the API key

### 2. Environment Variables

For **Cloud Run deployment**:
```bash
ENABLE_TRACING=true
OTEL_SERVICE_NAME=buildkite-webhook
OTEL_ENVIRONMENT=production
OTEL_EXPORTER_OTLP_ENDPOINT=https://api.honeycomb.io
OTEL_EXPORTER_OTLP_HEADERS=x-honeycomb-team=YOUR_API_KEY
```

For **local development**:
```bash
# .env file
ENABLE_TRACING=true
OTEL_SERVICE_NAME=buildkite-webhook
OTEL_ENVIRONMENT=development
OTEL_EXPORTER_OTLP_ENDPOINT=https://api.honeycomb.io
OTEL_EXPORTER_OTLP_HEADERS=x-honeycomb-team=YOUR_API_KEY
```

### 3. Deploy to Cloud Run

```bash
gcloud run services update buildkite-webhook \
  --region us-central1 \
  --set-env-vars="ENABLE_TRACING=true,OTEL_SERVICE_NAME=buildkite-webhook,OTEL_ENVIRONMENT=production,OTEL_EXPORTER_OTLP_ENDPOINT=https://api.honeycomb.io,OTEL_EXPORTER_OTLP_HEADERS=x-honeycomb-team=YOUR_API_KEY"
```

## Jaeger Setup

### 1. Local Jaeger
```bash
# Start Jaeger using Docker
docker run -d --name jaeger \
  -p 16686:16686 \
  -p 14250:14250 \
  jaegertracing/all-in-one:latest
```

### 2. Environment Variables
```bash
ENABLE_TRACING=true
OTEL_SERVICE_NAME=buildkite-webhook
OTEL_ENVIRONMENT=development
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:14250
```

### 3. View Traces
Open http://localhost:16686 to view traces in Jaeger UI.

## Cloud Run Configuration

### Build and Deploy with Tracing

1. **Build image:**
```bash
export PROJECT_ID=your-project-id
docker build -t gcr.io/$PROJECT_ID/buildkite-webhook:latest .
docker push gcr.io/$PROJECT_ID/buildkite-webhook:latest
```

2. **Deploy with tracing:**
```bash
gcloud run deploy buildkite-webhook \
  --image gcr.io/$PROJECT_ID/buildkite-webhook:latest \
  --platform managed \
  --region us-central1 \
  --allow-unauthenticated \
  --set-env-vars="ENABLE_TRACING=true,PROJECT_ID=$PROJECT_ID,TOPIC_ID=buildkite-events,OTEL_SERVICE_NAME=buildkite-webhook,OTEL_ENVIRONMENT=production,OTEL_EXPORTER_OTLP_ENDPOINT=https://api.honeycomb.io,OTEL_EXPORTER_OTLP_HEADERS=x-honeycomb-team=YOUR_API_KEY"
```

### Environment Variables Reference

| Variable | Description | Example |
|----------|-------------|---------|
| `ENABLE_TRACING` | Enable/disable tracing | `true` |
| `OTEL_SERVICE_NAME` | Service name in traces | `buildkite-webhook` |
| `OTEL_ENVIRONMENT` | Environment label | `production`, `staging`, `development` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OTLP collector endpoint | `https://api.honeycomb.io` |
| `OTEL_EXPORTER_OTLP_HEADERS` | Authentication headers | `x-honeycomb-team=API_KEY` |

## Local Development

### 1. Environment Setup
```bash
# Copy environment template
cp .env.example .env

# Edit .env file with your tracing configuration
ENABLE_TRACING=true
OTEL_SERVICE_NAME=buildkite-webhook
OTEL_ENVIRONMENT=development
OTEL_EXPORTER_OTLP_ENDPOINT=https://api.honeycomb.io
OTEL_EXPORTER_OTLP_HEADERS=x-honeycomb-team=YOUR_API_KEY
```

### 2. Start Service
```bash
# Load environment variables and start
export $(grep -v '^#' .env | xargs) && go run cmd/webhook/main.go
```

### 3. Test Tracing
```bash
# Send test webhook
curl -X POST http://localhost:8888/webhook \
  -H "X-Buildkite-Token: your-webhook-token" \
  -H "Content-Type: application/json" \
  -d '{
    "event": "build.started",
    "build": {
      "id": "test-build-123",
      "number": 42,
      "state": "running"
    },
    "pipeline": {
      "name": "test-pipeline"
    },
    "sender": {
      "name": "test-user"
    }
  }'
```

Check your observability platform for traces within 1-2 minutes.

### Debug Mode

Enable debug logging temporarily:
```bash
# Add to environment variables
LOG_LEVEL=debug
```

This will show telemetry configuration details in startup logs.

### Sampling Configuration

By default, all traces are sampled (100%). To reduce volume in production:

1. **Modify sampling** in `internal/telemetry/telemetry.go`:
```go
sdktrace.WithSampler(sdktrace.TraceIDRatioBased(0.1)), // 10% sampling
```

2. **Rebuild and deploy** the service

## Integration Examples

### Custom Attributes
Add custom attributes to spans:
```go
span.SetAttributes(
    attribute.String("user.id", userID),
    attribute.String("pipeline.slug", pipeline.Slug),
)
```

### Error Recording
Record errors in spans:
```go
if err != nil {
    span.RecordError(err)
    span.SetStatus(codes.Error, err.Error())
}
```
