# Buildkite PubSub

A webhook handler that securely forwards Buildkite build events to Google Cloud Pub/Sub, enabling event-driven architectures and integrations with your Buildkite pipelines.

[![Build status](https://badge.buildkite.com/5199de1bb7bfbc37a604373b26605143f70ac6569ee2bfec6e.svg)](https://buildkite.com/testkite/buildkite-pub-sub)

## Overview

This service connects Buildkite's webhook system to Google Cloud Pub/Sub, allowing you to:

- Receive Buildkite build events (status changes, pipeline updates, etc.)
- Forward events securely to Pub/Sub topics
- Monitor and alert on webhook delivery
- Filter and process events using Pub/Sub subscriptions
- Build event-driven workflows and integrations

## Prerequisites

- [Go 1.20+](https://golang.org/dl/) for development
- [Docker](https://docs.docker.com/get-docker/) for container builds
- [kubectl](https://kubernetes.io/docs/tasks/tools/) for deployment
- [Orbstack](https://orbstack.dev/) for local Kubernetes
- [ngrok](https://ngrok.com/) for local webhook testing
- [Google Cloud Project](https://cloud.google.com/) with Pub/Sub enabled
- Buildkite organization admin access for webhook configuration

## Documentation

### Getting Started

- [Quick Start Guide](docs/QUICK_START.md) - Get up and running in minutes
- [Google Cloud Setup](docs/GCP_SETUP.md) - Configure GCP project and permissions
- [Testing Guide](docs/TESTING.md) - Local development through production testing

### Operations

- [Usage Guide](docs/USAGE.md) - Event patterns and integration examples
- [Monitoring](docs/MONITORING.md) - Metrics, dashboards, and alerts
- [Kubernetes Deployment](docs/K8S_DEPLOYMENT.md) - Production deployment configuration

See [docs/](docs/) for complete documentation including architecture, distributed tracing, event schemas, and more.

## Local Development

```bash
# Run locally
go run cmd/webhook/main.go

# Run tests
go test ./...

# Build container
docker build -t buildkite-webhook .
```

## Deployment

Follow the [Quick Start Guide](docs/QUICK_START.md) for complete deployment instructions, or see individual guides:

- [GCP Setup Guide](docs/GCP_SETUP.md) for Google Cloud configuration
- [Usage Guide](docs/USAGE.md) for event handling patterns

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -am 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

MIT License - see [LICENSE](LICENSE) file for details.
