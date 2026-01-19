# Buildkite PubSub

A webhook handler that securely forwards Buildkite build events to Google Cloud Pub/Sub, enabling event-driven architectures and integrations with your Buildkite pipelines.

[![Build status](https://badge.buildkite.com/868c0ddbafe1a0b410fa2ed43c29dcf2a9e2eb50635069cfee.svg)](https://buildkite.com/no-assembly/buildkite-pubsub?branch=main)

## Overview

This service connects Buildkite's webhook system to Google Cloud Pub/Sub, allowing you to:

- Receive Buildkite build events (status changes, pipeline updates, etc.)
- Forward events securely to Pub/Sub topics
- Handle failures gracefully with Dead Letter Queue support
- Protect against cascading failures with Circuit Breaker pattern
- Monitor and alert on webhook delivery
- Filter and process events using Pub/Sub subscriptions
- Build event-driven workflows and integrations

## Prerequisites

- [Go 1.21+](https://golang.org/dl/) for development
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

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

For security vulnerabilities, please see [SECURITY.md](SECURITY.md).

## License

MIT License - see [LICENSE](LICENSE) file for details.
