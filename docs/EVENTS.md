# Buildkite Events Reference

A comprehensive guide to Buildkite webhook payloads and event filtering strategies. For a complete list of events, see the [Buildkite Webhooks Documentation](https://buildkite.com/docs/apis/webhooks#webhook-events).

## Table of Contents

- [Event Types](#event-types)
- [Example Payloads](#example-payloads)
- [Event Filtering](#event-filtering)
  - [Basic Filtering](#basic-filtering)
  - [Filtering by Pipeline](#filtering-by-pipeline)
  - [Filtering by Build Status](#filtering-by-build-status)
  - [Complex Filters](#complex-filters)
  - [Branch-based Filtering](#branch-based-filtering)
- [Practical Examples](#practical-examples)
- [Additional Resources](#additional-resources)

## Event Types

The webhook service forwards all Buildkite event types to Pub/Sub. Common event types include:

| Event Type | Description | When It's Triggered |
|------------|-------------|---------------------|
| `ping` | Webhook test | When webhook is first configured |
| `build.scheduled` | Build scheduled | Build queued but not started |
| `build.running` | Build started | Build begins execution |
| `build.finished` | Build completed | Build ends (any state) |
| `job.scheduled` | Job scheduled | Individual job queued |
| `job.started` | Job started | Individual job begins |
| `job.finished` | Job completed | Individual job ends |

## Example Payloads

### Ping Event
Sent when testing webhook configuration:
```json
{
  "event": "ping",
  "organization": {
    "id": "123-org-id",
    "name": "Example Org"
  }
}
```

### Build Event
Example of a `build.finished` event (transformed format):
```json
{
  "event_type": "build.finished",
  "build": {
    "id": "01234567-89ab-cdef-0123-456789abcdef",
    "url": "https://api.buildkite.com/v2/organizations/example/pipelines/test/builds/123",
    "web_url": "https://buildkite.com/example/test/builds/123",
    "number": 123,
    "state": "passed",
    "branch": "main",
    "commit": "abc123def456",
    "pipeline": "test",
    "organization": "example",
    "created_at": "2025-01-01T10:00:00Z",
    "started_at": "2025-01-01T10:01:00Z",
    "finished_at": "2025-01-01T10:15:00Z"
  },
  "pipeline": {
    "id": "pipeline-uuid",
    "name": "test",
    "description": "Test pipeline",
    "repository": "https://github.com/example/repo"
  },
  "sender": {
    "id": "user-uuid",
    "name": "John Doe"
  }
}
```

Build states include: `passed`, `failed`, `blocked`, `canceled`, `canceling`, `skipped`, `not_run`, `running`, `scheduled`

## Event Filtering

Google Cloud Pub/Sub allows filtering subscriptions based on message attributes and message data. This enables you to create targeted subscriptions for specific events without processing all events.

**Note**: Pub/Sub filters use a SQL-like syntax. String values must be enclosed in single quotes.

### Basic Filtering

#### Subscribe to Specific Event Types

```bash
# Only build.finished events
gcloud pubsub subscriptions create build-finished-only \
  --topic buildkite-events \
  --filter="attributes.event_type = 'build.finished'"

# Only job events
gcloud pubsub subscriptions create job-events \
  --topic buildkite-events \
  --filter="attributes.event_type:('job.started', 'job.finished')"

# All build events (scheduled, running, finished)
gcloud pubsub subscriptions create all-build-events \
  --topic buildkite-events \
  --filter="attributes.event_type LIKE 'build.%'"
```

### Filtering by Pipeline

#### Single Pipeline

```bash
# Events for a specific pipeline
gcloud pubsub subscriptions create production-pipeline \
  --topic buildkite-events \
  --filter="attributes.event_type = 'build.finished' AND attributes:pipeline = 'production-deploy'"
```

#### Multiple Pipelines

```bash
# Events from multiple critical pipelines
gcloud pubsub subscriptions create critical-pipelines \
  --topic buildkite-events \
  --filter="attributes.event_type = 'build.finished' AND attributes.pipeline:('production-deploy', 'staging-deploy', 'database-migration')"
```

#### Pipeline Pattern Matching

```bash
# All deploy pipelines
gcloud pubsub subscriptions create deploy-pipelines \
  --topic buildkite-events \
  --filter="attributes.event_type = 'build.finished' AND attributes.pipeline LIKE '%deploy%'"
```

### Filtering by Build Status

#### Failed Builds Only

```bash
# Only failed builds
gcloud pubsub subscriptions create failed-builds \
  --topic buildkite-events \
  --filter="attributes.event_type = 'build.finished' AND attributes.build_state = 'failed'"
```

#### Specific Build States

```bash
# Passed or failed (exclude skipped, canceled, etc.)
gcloud pubsub subscriptions create completed-builds \
  --topic buildkite-events \
  --filter="attributes.event_type = 'build.finished' AND attributes.build_state:('passed', 'failed')"

# Only blocked builds (waiting for manual approval)
gcloud pubsub subscriptions create blocked-builds \
  --topic buildkite-events \
  --filter="attributes.event_type = 'build.finished' AND attributes.build_state = 'blocked'"
```

### Complex Filters

#### Failed Production Builds

```bash
# Critical alerts: production pipeline failures
gcloud pubsub subscriptions create production-failures \
  --topic buildkite-events \
  --filter="attributes.event_type = 'build.finished' AND attributes.pipeline = 'production-deploy' AND attributes.build_state = 'failed'"
```

#### Multiple Conditions with OR Logic

```bash
# Failed or blocked builds from production
gcloud pubsub subscriptions create production-issues \
  --topic buildkite-events \
  --filter="attributes.event_type = 'build.finished' AND attributes.pipeline = 'production-deploy' AND attributes.build_state:('failed', 'blocked')"
```

#### Excluding Event Types

```bash
# All events except ping
gcloud pubsub subscriptions create no-ping-events \
  --topic buildkite-events \
  --filter="attributes.event_type != 'ping'"
```

### Branch-based Filtering

```bash
# Main branch builds only
gcloud pubsub subscriptions create main-branch-builds \
  --topic buildkite-events \
  --filter="attributes.event_type = 'build.finished' AND attributes.branch = 'main'"

# Production branches (main, master, production)
gcloud pubsub subscriptions create production-branches \
  --topic buildkite-events \
  --filter="attributes.event_type = 'build.finished' AND attributes.branch:('main', 'master', 'production')"

# Release branches
gcloud pubsub subscriptions create release-builds \
  --topic buildkite-events \
  --filter="attributes.event_type = 'build.finished' AND attributes.branch LIKE 'release/%'"

# Pull request builds
gcloud pubsub subscriptions create pr-builds \
  --topic buildkite-events \
  --filter="attributes.event_type = 'build.finished' AND attributes.branch LIKE 'pr-%'"
```

## Practical Examples

### Use Case 1: Slack Notifications for Production Failures

Create a subscription that triggers a Cloud Function to send Slack alerts only for production failures:

```bash
gcloud pubsub subscriptions create slack-production-alerts \
  --topic buildkite-events \
  --filter="attributes.event_type = 'build.finished' AND attributes.pipeline = 'production-deploy' AND attributes.build_state = 'failed'" \
  --push-endpoint="https://your-region-your-project.cloudfunctions.net/send-slack-alert"
```

### Use Case 2: Build Metrics Collection

Collect metrics for all completed builds (passed or failed) on main branch:

```bash
gcloud pubsub subscriptions create metrics-main-branch \
  --topic buildkite-events \
  --filter="attributes.event_type = 'build.finished' AND attributes.branch = 'main' AND attributes.build_state:('passed', 'failed')"
```

### Use Case 3: Quality Gate - Blocked Builds Requiring Approval

Monitor builds waiting for manual approval:

```bash
gcloud pubsub subscriptions create approval-required \
  --topic buildkite-events \
  --filter="attributes.event_type = 'build.finished' AND attributes.build_state = 'blocked'"
```

## Filter Syntax Reference

### Operators

| Operator | Description | Example |
|----------|-------------|---------|
| `=` | Equals | `attributes.state = 'failed'` |
| `!=` | Not equals | `attributes.state != 'passed'` |
| `IN` | In list | `attributes.state:('failed', 'blocked')` |
| `LIKE` | Pattern match | `attributes.branch LIKE 'feature/%'` |
| `AND` | Logical AND | `event_type = 'build' AND state = 'failed'` |
| `OR` | Logical OR | Use `IN` operator or separate subscriptions |

### Testing Filters

Test your filter before creating production subscriptions:

```bash
# Create a test subscription
gcloud pubsub subscriptions create test-filter \
  --topic buildkite-events \
  --filter="YOUR_FILTER_HERE"

# Pull a few messages to verify
gcloud pubsub subscriptions pull test-filter --limit=5

# Delete when done
gcloud pubsub subscriptions delete test-filter
```

## Additional Resources

- [Buildkite Webhooks Documentation](https://buildkite.com/docs/apis/webhooks)
- [Webhook Event Types](https://buildkite.com/docs/apis/webhooks#webhook-events)
- [REST API Reference](https://buildkite.com/docs/apis/rest-api)
- [Google Cloud Pub/Sub Filters](https://cloud.google.com/pubsub/docs/subscription-message-filter)
- [Pub/Sub Filter Syntax](https://cloud.google.com/pubsub/docs/filtering)
