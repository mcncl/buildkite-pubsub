# Usage Guide

This guide explains how to work with Buildkite events in Google Cloud Pub/Sub after setting up the webhook service.

## Event Overview

### Event Types

The service forwards all Buildkite webhook events to Pub/Sub. Common event types include:

- `build.scheduled` - Build has been scheduled
- `build.started` - Build has started running
- `build.finished` - Build has completed
- `job.started` - Individual job has started
- `job.finished` - Individual job has completed

### Event Format

Each event is published with standard attributes:
```json
{
  "origin": "buildkite-webhook",
  "event_type": "build.finished"
}
```

Example event payload:
```json
{
  "event_type": "build.finished",
  "build": {
    "id": "019439b6-95f9-4326-81fb-25ac99289820",
    "url": "https://api.buildkite.com/v2/organizations/example/pipelines/test/builds/123",
    "web_url": "https://buildkite.com/example/test/builds/123",
    "number": 123,
    "state": "passed",
    "pipeline": "test",
    "organization": "example"
  }
}
```

## Working with Events

### Creating Subscriptions

1. Basic subscription:
```bash
# Create a subscription for all events
gcloud pubsub subscriptions create buildkite-events-all \
  --topic buildkite-events
```

2. Filtered subscription:
```bash
# Subscribe to specific event types
gcloud pubsub subscriptions create build-finished \
  --topic buildkite-events \
  --filter="attributes.event_type = \"build.finished\""

# Filter by organization
gcloud pubsub subscriptions create org-builds \
  --topic buildkite-events \
  --filter="attributes.event_type = \"build.finished\" AND organization = \"your-org\""
```

### Processing Events

Here are common patterns for event processing:

1. **Cloud Functions**
```python
def process_event(event, context):
    """Process a Pub/Sub message from Buildkite."""
    import base64
    import json

    # Decode the Pub/Sub message
    pubsub_message = base64.b64decode(event['data']).decode('utf-8')
    buildkite_event = json.loads(pubsub_message)

    # Process based on event type
    event_type = buildkite_event['event_type']
    if event_type == 'build.finished':
        handle_build_finished(buildkite_event)
    elif event_type == 'build.started':
        handle_build_started(buildkite_event)
```

2. **Cloud Run**
```python
from flask import Flask, request
import json

app = Flask(__name__)

@app.route('/', methods=['POST'])
def handle_pubsub():
    envelope = request.get_json()
    message = json.loads(
        base64.b64decode(envelope['message']['data']).decode('utf-8')
    )

    process_buildkite_event(message)
    return '', 204
```

## Storage Patterns

### Short-term Analysis

For short-term analysis and monitoring, use Pub/Sub subscriptions with appropriate retention:

```bash
# Create subscription with 7-day retention
gcloud pubsub subscriptions create buildkite-analysis \
  --topic buildkite-events \
  --message-retention-duration="7d" \
  --expiration-period="7d"
```

### Long-term Storage

For long-term storage and analysis:

1. **BigQuery** - For analysis and querying:
```bash
# Create dataset and table
bq mk --dataset your-project:buildkite_events
bq mk --table \
  --time_partitioning_field timestamp \
  --schema 'event_id:STRING,timestamp:TIMESTAMP,data:JSON' \
  your-project:buildkite_events.builds
```

2. **Cloud Storage** - For archival:
```bash
# Create archive bucket with lifecycle policy
gsutil mb gs://buildkite-events-archive
gsutil lifecycle set lifecycle-policy.json gs://buildkite-events-archive
```

## Best Practices

### Event Processing

1. **Idempotency**
   - Use event IDs for deduplication
   - Design handlers to be idempotent
   - Use atomic operations where possible

2. **Error Handling**
   - Implement exponential backoff for retries
   - Use dead-letter topics for failed messages
   - Log errors with context

3. **Performance**
   - Filter events at subscription level
   - Batch operations when possible
   - Monitor subscription backlog

### Security

1. **Webhook Authentication**
   
   The service supports two authentication methods:
   
   **Token-based (Simple)**
   ```bash
   # Set in Buildkite webhook configuration
   export BUILDKITE_WEBHOOK_TOKEN="your-token-here"
   ```
   
   **HMAC Signature (Recommended)**
   ```bash
   # More secure - prevents token interception
   export BUILDKITE_WEBHOOK_HMAC_SECRET="your-secret-here"
   ```
   
   HMAC signatures provide:
   - Protection against man-in-the-middle attacks
   - Timestamp validation (prevents replay attacks)
   - Cryptographic verification of message integrity
   
   Configure in Buildkite UI by selecting "Webhook Signature" and entering your secret.

2. **Access Control**
   - Use service accounts with minimal permissions
   - Rotate credentials regularly
   - Encrypt sensitive data

3. **Monitoring**
   - Monitor message processing latency
   - Set up alerts for processing failures
   - Track subscription backlogs

### Debug Tools

1. **Pub/Sub**
```bash
# Check subscription backlog
gcloud pubsub subscriptions seek buildkite-events-all --time="2023-01-01T00:00:00Z"

# View subscription details
gcloud pubsub subscriptions describe buildkite-events-all
```

2. **Logs**
```bash
# View webhook logs
kubectl logs -n buildkite-webhook -l app=buildkite-webhook

# View processor logs (Cloud Functions)
gcloud functions logs read event-processor
```

For more examples and reference implementations, check out:
- [Event Processing Examples](./examples/)
- [Integration Patterns](./docs/INTEGRATIONS.md)
- [Monitoring Guide](./docs/MONITORING.md)
