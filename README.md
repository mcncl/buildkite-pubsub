# Buildkite PubSub Webhook

A production-ready webhook handler that forwards Buildkite events to Google Cloud Pub/Sub, with comprehensive monitoring and security features.

## Overview

This service provides:
- Secure webhook handling with token validation
- Event transformation and standardization
- Pub/Sub message publishing
- Comprehensive monitoring with Prometheus and Grafana
- Full Kubernetes deployment support
- Rate limiting and security controls
- Health monitoring and alerts

## Quick Start

See our [Quick Start Guide](docs/QUICK_START.md) for a complete setup walkthrough.

## Features

- 🔒 **Security**
  - Webhook token validation
  - Rate limiting (global and per-IP)
  - TLS termination
  - Security headers
  - CORS configuration

- 📊 **Monitoring**
  - Prometheus metrics
  - Grafana dashboards
  - Alert templates
  - Health checks

- 🚀 **Deployment**
  - Kubernetes manifests
  - Horizontal Pod Autoscaling
  - Resource management
  - Rolling updates

- 📝 **Event Handling**
  - Standardized event transformation
  - Support for all Buildkite event types
  - Configurable event filtering
  - Error handling and retries

## Documentation

- [Quick Start Guide](docs/QUICK_START.md) - Complete setup walkthrough
- [Usage Guide](docs/USAGE.md) - Examples and patterns

## Requirements

- Go 1.20+
- Google Cloud Project with Pub/Sub enabled
- Kubernetes cluster (for production deployment)
- Buildkite webhook token

## Development

```bash
# Run locally
go run cmd/webhook/main.go

# Run tests
go test ./...

# Build
go build ./cmd/webhook
```

## Contributing

1. Fork the repository
2. Create your feature branch
3. Commit your changes
4. Push to the branch
5. Open a Pull Request

## License

MIT License - see [LICENSE](LICENSE) file for details.
