# Buildkite PubSub Webhook

A lightweight, extensible webhook handler that forwards Buildkite events to Google Cloud Pub/Sub. Built to provide GCP feature parity with Buildkite's AWS EventBridge integration.

## Features

- 🔒 Secure webhook token validation
- 🔄 Standardized event transformation
- 📊 Support for all Buildkite event types
- 🚀 Multiple deployment options (Cloud Functions, Cloud Run, standalone, Kubernetes)
- 📝 Comprehensive error handling and monitoring
- 🎯 Built-in health checks
- 📈 Event tracking and metrics

## Getting Started

### Prerequisites

- Go 1.20 or higher
- Google Cloud Project with Pub/Sub enabled
- Buildkite webhook token
- One of:
  - Kubernetes cluster
  - Cloud Run access
  - Local development environment

### Quick Start

1. Set up required environment variables:
```bash
export PROJECT_ID="your-gcp-project"
export TOPIC_ID="buildkite-events"
export BUILDKITE_WEBHOOK_TOKEN="your-buildkite-webhook-token"
```

2. Create a Pub/Sub topic:
```bash
gcloud pubsub topics create buildkite-events
```

3. Create a subscription for testing (optional):
```bash
gcloud pubsub subscriptions create buildkite-events-test \
    --topic buildkite-events \
    --message-retention-duration="1h"
```

### Running Locally

```bash
go run cmd/webhook/main.go
```

## Deployment

### Kubernetes (Recommended)

```bash
# Create namespace and secrets
kubectl create namespace buildkite-webhook
kubectl apply -f k8s/secrets.yaml

# Deploy the application
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/service.yaml
kubectl apply -f k8s/ingress.yaml
```

### Google Cloud Run

```bash
gcloud builds submit --tag gcr.io/your-project/buildkite-webhook
gcloud run deploy buildkite-webhook \
  --image gcr.io/your-project/buildkite-webhook \
  --platform managed \
  --set-env-vars "PROJECT_ID=your-project-id,TOPIC_ID=buildkite-events,BUILDKITE_WEBHOOK_TOKEN=your-webhook-token"
```

## Configuration

| Variable | Description | Required | Default |
|----------|-------------|----------|---------|
| PROJECT_ID | GCP Project ID | Yes | - |
| TOPIC_ID | Pub/Sub Topic ID | Yes | - |
| BUILDKITE_WEBHOOK_TOKEN | Buildkite Webhook Token | Yes | - |
| PORT | Server Port | No | 8080 |

## Monitoring

The service exposes the following endpoints:
- `/health` - Liveness probe
- `/ready` - Readiness probe

Metrics available:
- Request latency
- Request count by status
- Publishing latency
- Error count

## Security

- Webhook token validation
- TLS termination
- Rate limiting
- Security headers
- CORS configuration
- Read-only filesystem
- Non-root user execution

## Development

```bash
# Run tests
go test ./...

# Run with hot reload
air

# Build
go build ./cmd/webhook
```

## What's Next?

See [USAGE.md](docs/USAGE.md) for examples of how to:
- Set up event subscriptions
- Filter events
- Process events with Cloud Functions
- Store events in various backends
- Set up monitoring and alerting

## Contributing

1. Fork the repository
2. Create your feature branch
3. Commit your changes
4. Push to the branch
5. Open a Pull Request

## License

MIT License - see [LICENSE](LICENSE) file for details.
