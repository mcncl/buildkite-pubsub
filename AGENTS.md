# Buildkite PubSub - Improvements & Tasks for LLM Agents

**Last Updated**: 2025-11-10  
**Project Version**: v1.0.0  
**Total Lines of Code**: ~8,667 lines of Go

## Project Overview

This is a well-architected webhook handler service that forwards Buildkite build events to Google Cloud Pub/Sub. The codebase demonstrates solid engineering practices with:

- ✅ Good test coverage (65-100% across most packages)
- ✅ Comprehensive documentation
- ✅ Production-ready features (metrics, tracing, security)
- ✅ Clean architecture with proper separation of concerns
- ✅ Kubernetes deployment manifests
- ✅ CI/CD pipeline setup

## Assessment Summary

**Strengths:**
- Clean, well-structured Go code
- Good error handling with custom error types
- Comprehensive observability (Prometheus, OpenTelemetry)
- Security features (rate limiting, HMAC validation, CORS)
- Kubernetes-ready with proper health checks
- Good documentation coverage

**Areas for Improvement:**
- Test coverage gaps in some areas (publisher: 20.9%, cmd/webhook: 5.8%)
- Missing CI/CD security scanning
- No automated dependency updates (Renovate config exists but may need activation)
- Documentation could be more discoverable
- Missing performance benchmarks
- No chaos engineering/resilience testing
- Limited examples for common use cases

---

## Priority 0: Pre-Critical Foundation (Immediate Impact)

### TASK-000: Implement Application-Level Retry Logic ✅ COMPLETED
**Priority**: P0 (Pre-Critical - DONE!)  
**Complexity**: Medium  
**Actual Time**: ~4.5 hours  
**Files**: `pkg/webhook/handler.go`, `pkg/webhook/handler_retry_test.go`

**Status**: ✅ **COMPLETED** - Implemented with comprehensive test coverage

**What Was Implemented:**
1. ✅ Created `publishWithRetry()` method with exponential backoff
2. ✅ Backoff strategy: 100ms → 500ms → 1s → 2s → 5s → 10s
3. ✅ Only retries `errors.IsRetryable()` errors (connection, publish, rate_limit)
4. ✅ Non-retryable errors (auth, validation) fail immediately
5. ✅ Integrated `metrics.RecordPubsubRetry()` metric recording
6. ✅ Respects context cancellation
7. ✅ Thread-safe for concurrent calls
8. ✅ Comprehensive test suite with 11 test scenarios:
   - Success scenarios (4 tests)
   - Failure scenarios (3 tests)
   - Exponential backoff verification
   - Non-retryable error handling
   - Retry metrics recording
   - Context cancellation handling
   - Concurrent call safety
   - Edge cases (zero retries, large max retries)

**Test Results:**
- All 11 retry tests pass ✅
- All existing tests still pass ✅
- Coverage maintained at 87.8% ✅
- No TODOs in code ✅

**Configuration:**
- Currently hardcoded to 3 retries (can be made configurable in future)
- Exponential backoff prevents overwhelming Pub/Sub
- Context-aware for graceful cancellation

**Why This Was Done First:**
- Google Cloud Pub/Sub client retries network failures automatically
- BUT application doesn't retry on quota limits, permission errors, etc.
- Previously lost events on transient failures
- Config exists (`PubSubRetryMaxAttempts`) but wasn't used
- Metrics exist (`RecordPubsubRetry`) but weren't called
- Foundation for TASK-005 (DLQ) and TASK-006 (Circuit Breaker)

---

## Priority 1: Critical Improvements (High Impact)

### TASK-001: Improve Test Coverage for Publisher Package
**Priority**: P0  
**Complexity**: Medium  
**Estimated Time**: 4-6 hours  
**Files**: `internal/publisher/pubsub.go`, `internal/publisher/pubsub_test.go`

**Description:**
The publisher package has only 20.9% test coverage, which is critical as it handles the core Pub/Sub publishing functionality. Increase coverage to at least 80%.

**Requirements:**
- Add integration tests for `PubSubPublisher.Publish()` method
- Test error scenarios (connection failures, timeouts, quota exceeded)
- Test retry logic and exponential backoff
- Test batch publishing behavior
- Test publisher shutdown and cleanup
- Mock Google Cloud Pub/Sub client properly

**Acceptance Criteria:**
- Publisher package test coverage >= 80%
- All error paths tested
- Integration tests pass with mocked Pub/Sub
- Tests run in CI pipeline

**Dependencies:** None

---

### TASK-002: Add Integration Tests for Main Application
**Priority**: P0  
**Complexity**: High  
**Estimated Time**: 6-8 hours  
**Files**: `cmd/webhook/main.go`, `cmd/webhook/main_test.go`

**Description:**
The main application has only 5.8% test coverage. Add end-to-end integration tests that verify the full request flow.

**Requirements:**
- Create integration test suite that starts the full server
- Test complete webhook flow from HTTP request to Pub/Sub publish
- Test middleware chain (auth, rate limiting, logging, tracing)
- Test graceful shutdown
- Test configuration loading from different sources
- Test health check endpoints
- Use testcontainers or similar for real Pub/Sub emulator

**Acceptance Criteria:**
- Main package test coverage >= 60%
- Integration tests cover happy path and error scenarios
- Tests are isolated and can run in parallel
- Tests cleanup resources properly

**Dependencies:** TASK-001 (publisher tests should be solid first)

---

### TASK-003: Implement Security Scanning in CI/CD Pipeline
**Priority**: P0  
**Complexity**: Low  
**Estimated Time**: 2-3 hours  
**Files**: `.buildkite/pipeline.yaml`

**Description:**
Add automated security scanning to catch vulnerabilities early in the development cycle.

**Requirements:**
- Add Trivy for container image scanning
- Add gosec for Go security scanning
- Add dependency vulnerability scanning (nancy or govulncheck)
- Add secrets scanning (gitleaks is already configured in GitHub Actions, ensure it's in Buildkite too)
- Fail builds on high/critical vulnerabilities
- Generate security reports as artifacts

**Acceptance Criteria:**
- All security tools run in CI pipeline
- Security scan results available as build artifacts
- Pipeline fails on critical/high vulnerabilities
- Documentation updated with security scanning info

**Dependencies:** None

---

### TASK-004: Add Comprehensive Benchmarks
**Priority**: P1  
**Complexity**: Medium  
**Estimated Time**: 4-5 hours  
**Files**: `pkg/webhook/handler_bench_test.go`, `internal/buildkite/transform_bench_test.go`, `internal/publisher/benchmark_test.go`

**Description:**
Add performance benchmarks to track and prevent performance regressions.

**Requirements:**
- Benchmark webhook handler full request cycle
- Benchmark payload transformation
- Benchmark Pub/Sub publishing
- Benchmark middleware chain overhead
- Benchmark JSON parsing/serialization
- Add benchmark results to CI (store and compare)
- Document expected performance baselines

**Acceptance Criteria:**
- Benchmarks for all critical paths
- Benchmark tests run in CI
- Performance baseline documented
- Memory allocation profiling included

**Dependencies:** None

---

## Priority 2: Feature Enhancements (Medium-High Impact)

### TASK-005: Implement Dead Letter Queue for Failed Events
**Priority**: P1  
**Complexity**: Medium  
**Estimated Time**: 5-6 hours  
**Files**: `internal/publisher/pubsub.go`, `internal/config/config.go`, `docs/ARCHITECTURE.md`

**Description:**
Add dead letter queue (DLQ) functionality to handle events that fail to publish after retries.

**Requirements:**
- Create separate Pub/Sub topic for dead letter events
- Configure automatic routing of failed messages to DLQ after N retries
- Add DLQ-specific attributes (failure reason, retry count, original timestamp)
- Add configuration options for DLQ topic and retry threshold
- Add metrics for DLQ messages
- Document DLQ setup in GCP_SETUP.md
- Add example DLQ subscriber

**Acceptance Criteria:**
- Failed events automatically route to DLQ after max retries
- DLQ metrics exposed via Prometheus
- Documentation includes DLQ setup guide
- Tests verify DLQ behavior

**Dependencies:** TASK-001 (publisher tests)

---

### TASK-006: Add Circuit Breaker for Pub/Sub Publisher
**Priority**: P1  
**Complexity**: Medium  
**Estimated Time**: 4-5 hours  
**Files**: `internal/publisher/circuit_breaker.go`, `internal/publisher/pubsub.go`

**Description:**
Implement circuit breaker pattern to prevent cascading failures when Pub/Sub is unavailable.

**Requirements:**
- Implement circuit breaker with three states: Closed, Open, Half-Open
- Configure thresholds (failure rate, consecutive failures, timeout)
- Add circuit breaker state to health check
- Add Prometheus metrics for circuit breaker state
- Make circuit breaker configurable
- Add graceful degradation (return 503 when circuit is open)
- Add tests for all circuit breaker states

**Acceptance Criteria:**
- Circuit breaker prevents excessive calls during outages
- Circuit breaker state visible in metrics
- Health check reflects circuit breaker state
- Documentation includes circuit breaker configuration
- Tests cover all state transitions

**Dependencies:** TASK-001

---

### TASK-007: Add Event Schema Validation
**Priority**: P1  
**Complexity**: Medium  
**Estimated Time**: 5-6 hours  
**Files**: `internal/buildkite/validator.go`, `internal/buildkite/schema.go`

**Description:**
Add JSON schema validation for incoming Buildkite webhook payloads to catch malformed events early.

**Requirements:**
- Define JSON schemas for all supported Buildkite event types
- Implement schema validator using jsonschema library
- Add validation step before transformation
- Return detailed validation errors with field-level feedback
- Add metrics for validation failures
- Make strict validation optional via config
- Add tests for valid and invalid payloads

**Acceptance Criteria:**
- All Buildkite event types have schemas
- Invalid payloads rejected with detailed errors
- Validation metrics available
- Configuration option to enable/disable strict validation
- Tests cover all event types

**Dependencies:** None

---

### TASK-008: Implement Request Signing for Webhook Responses
**Priority**: P2  
**Complexity**: Low  
**Estimated Time**: 2-3 hours  
**Files**: `pkg/webhook/handler.go`, `internal/middleware/security/signing.go`

**Description:**
Add response signing so consumers can verify webhook responses haven't been tampered with.

**Requirements:**
- Generate HMAC signature for response bodies
- Add signature to response header (X-Webhook-Signature)
- Add timestamp to responses
- Document signature verification for consumers
- Add configuration for signing key
- Add tests for signature generation and verification

**Acceptance Criteria:**
- All webhook responses include signature header
- Signature verification example in documentation
- Configuration option to enable/disable signing
- Tests verify signature correctness

**Dependencies:** None

---

### TASK-009: Add Webhook Event Replay Capability
**Priority**: P2  
**Complexity**: High  
**Estimated Time**: 8-10 hours  
**Files**: `internal/replay/`, `cmd/replay/main.go`, `docs/REPLAY.md`

**Description:**
Create a tool to replay webhook events from a date range, useful for debugging and data recovery.

**Requirements:**
- Create separate CLI tool for event replay
- Query Pub/Sub subscription for messages in date range
- Support filtering by event type, pipeline, branch
- Support dry-run mode
- Add rate limiting to prevent overwhelming downstream
- Store replay state to support resume
- Add comprehensive logging
- Create documentation for replay tool

**Acceptance Criteria:**
- CLI tool can replay events from Pub/Sub
- Filtering works correctly
- Rate limiting prevents overload
- Tool can resume interrupted replays
- Documentation complete with examples

**Dependencies:** None

---

## Priority 3: Developer Experience (Medium Impact)

### TASK-010: Enhance CLI with Modern Framework (Kong or Cobra)
**Priority**: P2  
**Complexity**: Low-Medium  
**Estimated Time**: 3-4 hours  
**Files**: `cmd/webhook/main.go`, `go.mod`

**Description:**
Replace stdlib `flag` package with a modern CLI framework like Kong or Cobra to provide better UX, subcommands, and validation. Currently only has 3 flags, but framework would future-proof for additional commands.

**Current State:**
- Simple `flag` package with 3 flags: `--config`, `--log-level`, `--log-format`
- No subcommands
- No built-in validation or help generation
- Limited to running the server only

**Requirements:**
- Evaluate Kong vs Cobra (Kong recommended for simplicity and struct-based approach)
- Implement primary `serve` command (current behavior)
- Add `version` command showing build info and dependencies
- Add `validate` command to test configuration without starting server
- Add `config` subcommand with `show` to display parsed configuration
- Improve help text and examples
- Add flag validation (e.g., valid log levels)
- Support environment variable documentation in help
- Maintain backward compatibility where possible
- Add tests for CLI parsing

**Why Kong:**
- Struct-based configuration (more Go idiomatic)
- Excellent validation support
- Automatic help generation
- Environment variable support built-in
- Smaller dependency footprint than Cobra
- Great for single-binary CLI tools

**Why Cobra (alternative):**
- More mature and widely used
- Better for complex CLI apps with many subcommands
- Kubernetes, GitHub CLI, and Docker use it
- More community examples

**Acceptance Criteria:**
- CLI framework implemented (Kong or Cobra)
- `serve`, `version`, `validate`, and `config show` commands work
- Help text is clear and includes examples
- All flags validated properly
- Environment variables documented in help
- Backward compatibility maintained for critical flags
- Tests cover CLI parsing and validation
- Documentation updated with new commands

**Example Usage:**
```bash
# Run server (current behavior)
buildkite-webhook serve --config=config.yaml

# Show version
buildkite-webhook version

# Validate config
buildkite-webhook validate --config=config.yaml

# Show parsed configuration
buildkite-webhook config show --config=config.yaml

# Help text
buildkite-webhook --help
buildkite-webhook serve --help
```

**Dependencies:** None

---

### TASK-011: Create Interactive Setup Script
**Priority**: P2  
**Complexity**: Medium  
**Estimated Time**: 4-5 hours  
**Files**: `scripts/setup`, `scripts/lib/common.sh`

**Description:**
Create interactive setup script that guides users through initial configuration.

**Note:** This pairs well with TASK-010 (CLI enhancement) - could add `init` command to CLI.

**Requirements:**
- Interactive prompts for all required configuration
- Validate GCP project access
- Create Pub/Sub topics and subscriptions
- Create service account with minimal permissions
- Generate and store credentials securely
- Validate Buildkite access
- Configure local environment
- Support both first-time setup and updates
- Add comprehensive error handling

**Acceptance Criteria:**
- Script runs without errors
- User can go from zero to running in <10 minutes
- Script is idempotent (can run multiple times safely)
- Error messages are helpful and actionable

**Dependencies:** TASK-010 (optional - could be integrated as CLI command)

---

### TASK-013: Add Structured Logging with Log Levels
**Priority**: P2  
**Complexity**: Low  
**Estimated Time**: 2-3 hours  
**Files**: `internal/logging/logger.go`, `pkg/webhook/handler.go`

**Description:**
Enhance logging to support dynamic log level changes and structured fields consistently.

**Requirements:**
- Add endpoint to change log level at runtime (admin only)
- Ensure all logs use structured fields (no unstructured strings)
- Add correlation ID to all log entries from a request
- Add context-aware logging
- Document logging best practices
- Add log sampling for high-frequency logs

**Acceptance Criteria:**
- Log level changeable via API endpoint
- All logs properly structured
- Correlation IDs in all request logs
- Documentation updated with logging guide

**Dependencies:** None

---

### TASK-014: Create Example Subscribers
**Priority**: P2  
**Complexity**: Medium  
**Estimated Time**: 5-6 hours  
**Files**: `examples/subscribers/`, `docs/EXAMPLES.md`

**Description:**
Create example subscriber implementations for common use cases.

**Requirements:**
- Example 1: Slack notification subscriber (Cloud Function)
- Example 2: Metrics collector (aggregates build stats)
- Example 3: Data warehouse loader (BigQuery)
- Example 4: Status page updater
- Example 5: GitHub status check updater
- Include deployment instructions for each
- Add tests for example code
- Create comprehensive README for examples

**Acceptance Criteria:**
- 5+ working example subscribers
- Each example has deployment instructions
- Examples demonstrate best practices
- Tests verify example functionality

**Dependencies:** None

---

## Priority 4: Documentation & Observability (Medium Impact)

### TASK-015: Create Comprehensive Troubleshooting Guide
**Priority**: P2  
**Complexity**: Low  
**Estimated Time**: 3-4 hours  
**Files**: `docs/TROUBLESHOOTING.md`

**Description:**
Create detailed troubleshooting guide with common issues and solutions.

**Requirements:**
- Document common error messages and solutions
- Add debugging checklist
- Include log analysis guide
- Add network troubleshooting section
- Document GCP permission issues
- Add Buildkite webhook configuration issues
- Include Pub/Sub troubleshooting
- Add performance debugging guide
- Include links to relevant metrics/logs

**Acceptance Criteria:**
- Comprehensive troubleshooting guide
- Covers 20+ common issues
- Includes commands/scripts for diagnosis
- Links from error messages to guide

**Dependencies:** None

---

### TASK-016: Add Grafana Dashboards via ConfigMap
**Priority**: P2  
**Complexity**: Medium  
**Estimated Time**: 4-5 hours  
**Files**: `k8s/monitoring/grafana/dashboard-configmap.yaml`, `grafana/dashboard.json`

**Description:**
Implement proper Grafana dashboard provisioning via ConfigMap (currently documented as limited).

**Requirements:**
- Convert existing dashboard JSON to ConfigMap
- Add provisioning configuration
- Support multiple dashboards
- Add dashboard for:
  - Overview (requests, errors, latency)
  - Security (rate limits, auth failures)
  - Performance (pub/sub latency, payload sizes)
  - Business metrics (builds by pipeline, failure rates)
- Update monitoring documentation
- Add dashboard import instructions

**Acceptance Criteria:**
- Dashboards automatically loaded in Grafana
- All key metrics visualized
- Dashboards documented
- Easy to add new dashboards

**Dependencies:** None

---

### TASK-017: Add OpenAPI/Swagger Specification
**Priority**: P2  
**Complexity**: Low  
**Estimated Time**: 2-3 hours  
**Files**: `docs/openapi.yaml`, `cmd/webhook/main.go`

**Description:**
Create OpenAPI 3.0 specification for the webhook API.

**Requirements:**
- Document all endpoints (/webhook, /health, /ready, /metrics)
- Include request/response schemas
- Document authentication methods
- Add error response schemas
- Include example requests/responses
- Add endpoint to serve OpenAPI spec
- Generate API documentation from spec
- Include in README

**Acceptance Criteria:**
- Complete OpenAPI 3.0 spec
- Spec accessible via HTTP endpoint
- HTML documentation generated
- Examples work correctly

**Dependencies:** None

---

### TASK-018: Implement Distributed Tracing Enhancements
**Priority**: P2  
**Complexity**: Medium  
**Estimated Time**: 3-4 hours  
**Files**: `internal/telemetry/telemetry.go`, `pkg/webhook/handler.go`

**Description:**
Enhance distributed tracing with more detailed spans and attributes.

**Requirements:**
- Add span for each middleware
- Add detailed attributes (request size, user agent, IP)
- Add custom events for important operations
- Add baggage propagation for cross-service correlation
- Add sampler configuration based on attributes
- Add trace exemplars to metrics
- Document tracing best practices

**Acceptance Criteria:**
- More granular trace data
- Traces linked to metrics via exemplars
- Sampling configurable
- Documentation updated

**Dependencies:** None

---

## Priority 5: Reliability & Testing (Medium Impact)

### TASK-019: Add Chaos Engineering Tests
**Priority**: P2  
**Complexity**: High  
**Estimated Time**: 8-10 hours  
**Files**: `tests/chaos/`, `docs/CHAOS_TESTING.md`

**Description:**
Implement chaos engineering tests to verify system resilience.

**Requirements:**
- Test Pub/Sub unavailability
- Test network latency/packet loss
- Test memory pressure
- Test CPU throttling
- Test dependency failures
- Test graceful degradation
- Create chaos testing framework
- Add to CI/CD as optional step
- Document chaos test results

**Acceptance Criteria:**
- 5+ chaos scenarios tested
- System degrades gracefully
- No cascading failures
- Documentation complete

**Dependencies:** TASK-006 (circuit breaker)

---

### TASK-020: Add End-to-End Smoke Tests
**Priority**: P2  
**Complexity**: Medium  
**Estimated Time**: 4-5 hours  
**Files**: `tests/e2e/`, `.buildkite/pipeline.yaml`

**Description:**
Create end-to-end smoke tests that run against deployed environment.

**Requirements:**
- Test webhook from Buildkite to Pub/Sub
- Verify message delivery
- Test subscription filtering
- Test error handling
- Run after deployment in CI/CD
- Support multiple environments (staging, production)
- Add smoke test dashboard
- Document smoke test setup

**Acceptance Criteria:**
- Smoke tests run automatically post-deployment
- Tests verify critical paths
- Failures trigger alerts
- Tests don't affect production traffic

**Dependencies:** None

---

### TASK-021: Implement Canary Deployment Strategy
**Priority**: P2  
**Complexity**: High  
**Estimated Time**: 6-8 hours  
**Files**: `k8s/canary/`, `scripts/deploy-canary`, `docs/DEPLOYMENT.md`

**Description:**
Add canary deployment capability to safely roll out changes.

**Requirements:**
- Create canary deployment manifests
- Implement traffic splitting (10% -> 50% -> 100%)
- Add automated rollback on error rate increase
- Add canary analysis (compare metrics to baseline)
- Create deployment script
- Add Grafana dashboard for canary monitoring
- Document canary deployment process

**Acceptance Criteria:**
- Canary deployments work correctly
- Automatic rollback on failures
- Metrics compare canary to baseline
- Documentation complete

**Dependencies:** TASK-004 (benchmarks for baseline)

---

## Priority 6: Operations & Maintenance (Low-Medium Impact)

### TASK-022: Add Automated Dependency Updates
**Priority**: P2  
**Complexity**: Low  
**Estimated Time**: 2 hours  
**Files**: `renovate.json`, `.github/dependabot.yml`

**Description:**
Configure and activate automated dependency updates.

**Requirements:**
- Configure Renovate bot (config exists but may need activation)
- Add GitHub Dependabot for security updates
- Set up auto-merge for minor/patch updates
- Configure grouping for related dependencies
- Add tests that run on dependency PRs
- Set up notifications for major updates
- Document dependency update process

**Acceptance Criteria:**
- Automated PRs for dependency updates
- Security updates auto-merge after tests pass
- Major updates require manual review
- Documentation updated

**Dependencies:** TASK-003 (security scanning)

---

### TASK-023: Implement Configuration Hot Reload
**Priority**: P3  
**Complexity**: Medium  
**Estimated Time**: 4-5 hours  
**Files**: `internal/config/config.go`, `cmd/webhook/main.go`

**Description:**
Add ability to reload configuration without restarting the service.

**Requirements:**
- Watch configuration file for changes
- Reload on SIGHUP signal
- Validate new config before applying
- Rollback on validation failure
- Add endpoint to trigger reload
- Log configuration changes
- Add metrics for config reloads
- Document hot reload functionality

**Acceptance Criteria:**
- Configuration reloads without restart
- Invalid configs rejected safely
- Reload events logged and metriced
- Documentation updated

**Dependencies:** None

---

### TASK-024: Add Prometheus Alert Rules
**Priority**: P2  
**Complexity**: Low  
**Estimated Time**: 2-3 hours  
**Files**: `prometheus/alerts.yaml`, `docs/MONITORING.md`

**Description:**
Expand Prometheus alert rules with comprehensive coverage.

**Requirements:**
- High error rate alert
- High latency alert  
- Pub/Sub publish failures alert
- Authentication failure spike alert
- Rate limit triggered alert
- Circuit breaker open alert
- Pod restart alert
- Memory/CPU pressure alerts
- Add alert severity levels
- Document alert response procedures

**Acceptance Criteria:**
- 10+ alert rules defined
- Alerts have severity and runbook links
- Alert documentation complete
- Alerts tested in staging

**Dependencies:** None

---

### TASK-025: Create Backup and Restore Procedures
**Priority**: P2  
**Complexity**: Low  
**Estimated Time**: 2-3 hours  
**Files**: `scripts/backup`, `scripts/restore`, `docs/BACKUP_RESTORE.md`

**Description:**
Document and automate backup/restore procedures.

**Requirements:**
- Backup configuration (ConfigMaps, Secrets)
- Backup Pub/Sub topics and subscriptions
- Export Grafana dashboards
- Export Prometheus rules and alerts
- Create restore script
- Test backup/restore procedure
- Document disaster recovery process
- Add backup automation

**Acceptance Criteria:**
- Automated backup script
- Tested restore procedure
- Disaster recovery documentation
- Backups stored securely

**Dependencies:** None

---

### TASK-026: Implement Multi-Region Deployment Support
**Priority**: P3  
**Complexity**: High  
**Estimated Time**: 10-12 hours  
**Files**: `k8s/multi-region/`, `docs/MULTI_REGION.md`

**Description:**
Add support for multi-region deployment for high availability.

**Requirements:**
- Multi-region Kubernetes manifests
- Cross-region Pub/Sub topic replication
- Traffic management and routing
- Regional failover strategy
- Monitoring per region
- Document multi-region setup
- Add region-aware health checks
- Test failover scenarios

**Acceptance Criteria:**
- Service can run in multiple regions
- Automatic failover works
- Cross-region monitoring
- Complete documentation

**Dependencies:** TASK-021 (deployment strategy)

---

## Priority 7: Code Quality & Maintenance (Low Impact)

### TASK-027: Add Pre-commit Hooks
**Priority**: P3  
**Complexity**: Low  
**Estimated Time**: 1-2 hours  
**Files**: `.pre-commit-config.yaml`, `docs/CONTRIBUTING.md`

**Description:**
Set up pre-commit hooks to catch issues before commit.

**Requirements:**
- gofmt formatting check
- golint/golangci-lint
- go vet
- Test execution
- Security scanning (gosec)
- Git commit message validation
- Trailing whitespace check
- Documentation for setup

**Acceptance Criteria:**
- Pre-commit hooks installed
- Hooks run automatically
- Documentation updated
- Easy to setup for new contributors

**Dependencies:** None

---

### TASK-028: Refactor Configuration Loading
**Priority**: P3  
**Complexity**: Low  
**Estimated Time**: 2-3 hours  
**Files**: `internal/config/config.go`

**Description:**
Simplify configuration loading logic and improve testability.

**Requirements:**
- Extract file parsing to separate functions
- Simplify merge logic
- Add validation helper functions
- Improve error messages
- Add more comprehensive tests
- Document configuration precedence clearly

**Acceptance Criteria:**
- Configuration code more readable
- Better test coverage
- Clearer error messages
- Documentation improved

**Dependencies:** None

---

### TASK-029: Add Code Coverage Reports to CI
**Priority**: P3  
**Complexity**: Low  
**Estimated Time**: 1-2 hours  
**Files**: `.buildkite/pipeline.yaml`

**Description:**
Generate and publish code coverage reports in CI pipeline.

**Requirements:**
- Generate coverage report on every build
- Upload to coverage service (Codecov/Coveralls)
- Add coverage badge to README
- Enforce minimum coverage threshold
- Track coverage trends
- Comment coverage change on PRs

**Acceptance Criteria:**
- Coverage reports in CI
- Coverage badge in README
- Coverage tracked over time
- PR comments show coverage change

**Dependencies:** None

---

### TASK-030: Implement Structured Error Codes
**Priority**: P3  
**Complexity**: Low  
**Estimated Time**: 2-3 hours  
**Files**: `internal/errors/errors.go`, `internal/errors/codes.go`

**Description:**
Add structured error codes for easier error tracking and debugging.

**Requirements:**
- Define error code constants
- Add error codes to all error types
- Include error codes in API responses
- Document all error codes
- Add error code to metrics
- Create error code reference guide

**Acceptance Criteria:**
- All errors have unique codes
- Error codes in API responses
- Complete error code documentation
- Metrics include error codes

**Dependencies:** None

---

### TASK-031: Add Request/Response Logging with Sampling
**Priority**: P3  
**Complexity**: Low  
**Estimated Time**: 2 hours  
**Files**: `internal/middleware/logging/middleware.go`

**Description:**
Add detailed request/response logging with sampling to reduce log volume.

**Requirements:**
- Log full request/response for errors
- Sample successful requests (configurable rate)
- Redact sensitive data
- Add correlation IDs
- Configure via environment
- Add log volume metrics

**Acceptance Criteria:**
- Request/response logging works
- Sampling reduces log volume
- Sensitive data redacted
- Configuration documented

**Dependencies:** None

---

## Maintenance & Housekeeping Tasks

### TASK-032: Update Dependencies to Latest Versions
**Priority**: P3  
**Complexity**: Low  
**Estimated Time**: 1-2 hours  
**Files**: `go.mod`, `Dockerfile`

**Description:**
Update all dependencies to latest stable versions.

**Requirements:**
- Update Go version in go.mod
- Update all dependencies
- Update Docker base images
- Run full test suite
- Update documentation if APIs changed
- Test in staging environment

**Acceptance Criteria:**
- All dependencies current
- Tests pass
- No breaking changes
- Staging deployment successful

**Dependencies:** TASK-002 (need good tests first)

---

### TASK-033: Add CONTRIBUTING.md Guide
**Priority**: P3  
**Complexity**: Low  
**Estimated Time**: 2 hours  
**Files**: `CONTRIBUTING.md`

**Description:**
Create comprehensive contributing guide for new contributors.

**Requirements:**
- Code style guide
- Commit message conventions
- PR process
- Testing requirements
- Documentation requirements
- Development setup
- Contact information
- Code of conduct

**Acceptance Criteria:**
- Complete contributing guide
- Clear instructions
- Examples included
- Linked from README

**Dependencies:** None

---

### TASK-034: Add Security Policy
**Priority**: P3  
**Complexity**: Low  
**Estimated Time**: 1 hour  
**Files**: `SECURITY.md`

**Description:**
Create security policy for vulnerability reporting.

**Requirements:**
- Define supported versions
- Vulnerability reporting process
- Expected response time
- Security update policy
- Contact information
- PGP key for encrypted reports

**Acceptance Criteria:**
- SECURITY.md created
- Clear reporting process
- Contact info provided
- Linked from README

**Dependencies:** None

---

### TASK-035: Optimize Docker Image Size
**Priority**: P3  
**Complexity**: Low  
**Estimated Time**: 2 hours  
**Files**: `Dockerfile`, `Dockerfile.dev`

**Description:**
Reduce Docker image size for faster deployments.

**Requirements:**
- Use distroless or minimal base image
- Multi-stage build optimization
- Remove unnecessary files
- Compress binary with UPX (optional)
- Compare image sizes before/after
- Verify functionality

**Acceptance Criteria:**
- Image size reduced by >30%
- All functionality works
- Security not compromised
- Build time acceptable

**Dependencies:** None

---

### TASK-036: Add Performance SLIs/SLOs
**Priority**: P3  
**Complexity**: Low  
**Estimated Time**: 2-3 hours  
**Files**: `docs/SLOS.md`, `prometheus/slo-rules.yaml`

**Description:**
Define and implement Service Level Indicators and Objectives.

**Requirements:**
- Define SLIs (latency, availability, error rate)
- Set SLOs (e.g., 99.9% availability, p95 latency < 200ms)
- Implement SLI recording rules in Prometheus
- Create SLO dashboard in Grafana
- Document SLIs/SLOs
- Add error budget tracking

**Acceptance Criteria:**
- SLIs/SLOs defined and documented
- Prometheus rules created
- Dashboard shows SLO compliance
- Error budget tracked

**Dependencies:** TASK-016 (Grafana dashboards)

---

## Implementation Notes

### For LLM Agents

Each task is designed to be:
- **Self-contained**: Can be completed independently
- **Well-scoped**: Clear start and end points
- **Testable**: Has defined acceptance criteria
- **Documented**: Includes files to modify and requirements

### Task Selection Guidelines

1. **Start with P0 tasks** - Critical improvements that affect system reliability
2. **Group related tasks** - Consider dependencies when planning
3. **Balance risk** - Mix low-risk improvements with high-impact changes
4. **Test thoroughly** - Each task should include comprehensive tests
5. **Document changes** - Update relevant documentation with each task

### Success Metrics

Track these metrics as tasks are completed:
- Test coverage percentage (target: >80% overall)
- Number of security vulnerabilities (target: 0 critical/high)
- Mean time to recovery (MTTR) (target: <5 minutes)
- Deployment success rate (target: >99%)
- Documentation completeness (target: all features documented)

---

## Quick Wins (Can be done in <1 hour each)

1. Add coverage badge to README (TASK-028 partial)
2. Add CONTRIBUTING.md (TASK-032)
3. Add SECURITY.md (TASK-033)
4. Fix .gitignore gaps (add .env, credentials.json, etc.)
5. Add GitHub issue templates enhancement
6. Add pull request template
7. Add CODE_OWNERS file
8. Add changelog automation
9. Add Docker Compose for local dev with Pub/Sub emulator
10. Add VS Code workspace settings

---

## Long-term Vision (6-12 months)

- Full multi-cloud support (AWS SNS, Azure Event Grid)
- Plugin system for custom event processors
- Web UI for configuration and monitoring
- Real-time event preview/testing UI
- Machine learning for anomaly detection
- Auto-scaling based on event volume
- GraphQL API for event queries
- Terraform/Pulumi modules for easy deployment

---

**Total Tasks**: 36  
**Estimated Total Time**: 145-185 hours  
**Recommended Approach**: Complete P0 tasks first (20-25 hours), then incrementally improve with P1-P2 tasks.

**Note on CLI Enhancement (TASK-010)**: While the current 3-flag CLI is adequate, upgrading to Kong/Cobra future-proofs the application for additional commands like `validate`, `version`, `config show`, and the proposed `init` command from TASK-011.
