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

- [Google Cloud Project](https://cloud.google.com/) with Pub/Sub enabled
- [Buildkite](https://buildkite.com/) organization with webhook configuration access

## Quick Start

```bash
# Build and run with Docker
docker build -t buildkite-webhook .
docker run -p 8080:8080 \
  -e PROJECT_ID=your-project \
  -e TOPIC_ID=buildkite-events \
  -e BUILDKITE_WEBHOOK_TOKEN=your-token \
  buildkite-webhook
```

Configure your Buildkite webhook to point to your deployed service URL.

## Development

```bash
# Run locally (requires Go 1.24+)
go run cmd/webhook/main.go

# Run tests
go test ./...

# Run tests with Docker
docker compose --profile ci up test
```

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

MIT License - see [LICENSE](LICENSE) file for details.
