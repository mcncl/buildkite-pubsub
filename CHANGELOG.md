# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2026-01-01

This release marks the first production-ready version of Buildkite PubSub.

### Added

- **Dead Letter Queue (DLQ) Support**: Failed messages that exhaust retry attempts are now optionally sent to a configurable DLQ topic for later analysis and reprocessing
  - New configuration options: `ENABLE_DLQ` and `DLQ_TOPIC_ID`
  - DLQ messages include enriched metadata (failure reason, retry count, original attributes)
  - New Prometheus metric: `buildkite_dlq_messages_total`

- **Circuit Breaker Pattern**: Protects the system from cascading failures when Pub/Sub is unavailable
  - Three states: Closed (normal), Open (failing fast), Half-Open (testing recovery)
  - Configurable thresholds for failure detection and recovery
  - New Prometheus metrics: `buildkite_circuit_breaker_state`, `buildkite_circuit_breaker_trips_total`

- **Security Scanning in CI**: Automated vulnerability detection in the build pipeline
  - `gosec` for Go security static analysis
  - `govulncheck` for known vulnerability detection in dependencies

- **Documentation**:
  - `CONTRIBUTING.md` - Guidelines for contributors
  - `SECURITY.md` - Security policy and vulnerability reporting process
  - `.pre-commit-config.yaml` - Pre-commit hooks for code quality

### Changed

- Publisher package test coverage improved from 20.9% to 97.7%

### Dependencies

- Updated `google.golang.org/grpc` to v1.78.0
- Updated `google.golang.org/api` to v0.258.0
- Updated base Docker images

## [0.4.2] - Previous Release

See git history for changes prior to v1.0.0.
