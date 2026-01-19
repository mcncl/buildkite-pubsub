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

- [Quick Start](docs/QUICK_START.md) - Get running locally in minutes
- [GCP Setup](docs/GCP_SETUP.md) - Configure Google Cloud project
- [Kubernetes Deployment](docs/K8S_DEPLOYMENT.md) - Deploy to Kubernetes
- [Events](docs/EVENTS.md) - Event types and Pub/Sub filtering
- [Monitoring](docs/MONITORING.md) - Metrics and dashboards
- [Distributed Tracing](docs/DISTRIBUTED_TRACING.md) - OpenTelemetry setup

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

See [GCP Setup](docs/GCP_SETUP.md) for Google Cloud configuration or [Kubernetes Deployment](docs/K8S_DEPLOYMENT.md) for cluster deployment.

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

For security vulnerabilities, please see [SECURITY.md](SECURITY.md).

## License

MIT License - see [LICENSE](LICENSE) file for details.
