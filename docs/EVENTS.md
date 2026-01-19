# Buildkite Events

The webhook forwards all Buildkite events to Pub/Sub. For the complete list, see [Buildkite Webhooks Documentation](https://buildkite.com/docs/apis/webhooks#webhook-events).

## Common Events

| Event | Description |
|-------|-------------|
| `ping` | Webhook test |
| `build.scheduled` | Build queued |
| `build.running` | Build started |
| `build.finished` | Build completed (any state) |
| `job.scheduled` | Job queued |
| `job.started` | Job started |
| `job.finished` | Job completed |

## Message Format

Events are published with these attributes for filtering:

```json
{
  "origin": "buildkite-webhook",
  "event_type": "build.finished",
  "pipeline": "my-pipeline",
  "branch": "main",
  "build_state": "passed"
}
```

## Filtering Subscriptions

Pub/Sub subscriptions can filter messages using a SQL-like syntax.

### Examples

```bash
# Only build.finished events
gcloud pubsub subscriptions create build-finished \
  --topic buildkite-events \
  --filter="attributes.event_type = 'build.finished'"

# Failed builds only
gcloud pubsub subscriptions create failed-builds \
  --topic buildkite-events \
  --filter="attributes.event_type = 'build.finished' AND attributes.build_state = 'failed'"

# Specific pipeline
gcloud pubsub subscriptions create production-builds \
  --topic buildkite-events \
  --filter="attributes.pipeline = 'production-deploy'"

# Main branch only
gcloud pubsub subscriptions create main-branch \
  --topic buildkite-events \
  --filter="attributes.branch = 'main'"

# Combine filters
gcloud pubsub subscriptions create production-failures \
  --topic buildkite-events \
  --filter="attributes.event_type = 'build.finished' AND attributes.pipeline = 'production-deploy' AND attributes.build_state = 'failed'"
```

### Filter Operators

| Operator | Example |
|----------|---------|
| `=` | `attributes.state = 'failed'` |
| `!=` | `attributes.state != 'passed'` |
| `:()` (IN) | `attributes.state:('failed', 'blocked')` |
| `LIKE` | `attributes.branch LIKE 'release/%'` |
| `AND` | `event_type = 'build.finished' AND state = 'failed'` |

## Processing Events

### Cloud Function Example

```python
import base64
import json

def process_event(event, context):
    message = json.loads(base64.b64decode(event['data']).decode('utf-8'))
    
    if message['event_type'] == 'build.finished':
        build = message['build']
        print(f"Build #{build['number']} {build['state']}")
```

### Cloud Run Example

```python
from flask import Flask, request
import base64, json

app = Flask(__name__)

@app.route('/', methods=['POST'])
def handle():
    envelope = request.get_json()
    message = json.loads(base64.b64decode(envelope['message']['data']).decode())
    # Process message
    return '', 204
```

## Resources

- [Buildkite Webhooks](https://buildkite.com/docs/apis/webhooks)
- [Pub/Sub Filtering](https://cloud.google.com/pubsub/docs/filtering)
