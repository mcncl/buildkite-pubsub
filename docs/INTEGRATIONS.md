# Integration Patterns

This guide provides common integration patterns and examples for processing Buildkite webhook events from Pub/Sub.

## Table of Contents
1. [Notification Systems](#notification-systems)
2. [CI/CD Integrations](#cicd-integrations)
3. [Metrics and Analytics](#metrics-and-analytics)
4. [Data Storage](#data-storage)
5. [Best Practices](#best-practices)

## Notification Systems

### Slack Integration

```python
from google.cloud.functions import CloudFunction
from slack_sdk import WebClient
import base64
import json
import os

def notify_slack(event, context):
    """Notify Slack channel about build status changes."""
    # Parse Pub/Sub message
    pubsub_message = base64.b64decode(event['data']).decode('utf-8')
    buildkite_event = json.loads(pubsub_message)

    if buildkite_event['event_type'] != 'build.finished':
        return

    build = buildkite_event['build']
    if build['state'] != 'failed':
        return

    # Format message
    blocks = [
        {
            "type": "section",
            "text": {
                "type": "mrkdwn",
                "text": f"*Build Failed*\nPipeline: {build['pipeline']}\nBuild: #{build['number']}"
            }
        },
        {
            "type": "actions",
            "elements": [
                {
                    "type": "button",
                    "text": {"type": "plain_text", "text": "View Build"},
                    "url": build['web_url']
                }
            ]
        }
    ]

    # Send to Slack
    client = WebClient(token=os.environ['SLACK_TOKEN'])
    client.chat_postMessage(
        channel="#builds",
        blocks=blocks
    )
```

### Microsoft Teams Integration

```python
import requests
import os
import json
import base64

def notify_teams(event, context):
    """Send build notifications to Microsoft Teams."""
    # Parse message
    pubsub_message = base64.b64decode(event['data']).decode('utf-8')
    buildkite_event = json.loads(pubsub_message)
    build = buildkite_event['build']

    webhook_url = os.environ['TEAMS_WEBHOOK_URL']

    card = {
        "@type": "MessageCard",
        "@context": "https://schema.org/extensions",
        "title": f"Build {build['state']} - {build['pipeline']}",
        "sections": [{
            "facts": [
                {"name": "Pipeline", "value": build['pipeline']},
                {"name": "Build", "value": f"#{build['number']}"},
                {"name": "State", "value": build['state']}
            ]
        }],
        "potentialAction": [{
            "@type": "OpenUri",
            "name": "View Build",
            "targets": [{"os": "default", "uri": build['web_url']}]
        }]
    }

    requests.post(webhook_url, json=card)
```

## CI/CD Integrations

### GitHub Status Updates

```python
import os
import requests
import base64
import json

def update_github_status(event, context):
    """Update GitHub commit status based on build results."""
    # Parse message
    pubsub_message = base64.b64decode(event['data']).decode('utf-8')
    buildkite_event = json.loads(pubsub_message)
    build = buildkite_event['build']

    if 'github' not in build.get('source', {}):
        return

    headers = {
        "Authorization": f"token {os.environ['GITHUB_TOKEN']}",
        "Accept": "application/vnd.github.v3+json"
    }

    status = {
        "state": "success" if build['state'] == "passed" else "failure",
        "description": f"Buildkite build #{build['number']} {build['state']}",
        "target_url": build['web_url'],
        "context": "buildkite/build"
    }

    requests.post(
        build['source']['github']['status_url'],
        headers=headers,
        json=status
    )
```

### JIRA Integration

```python
from jira import JIRA
import base64
import json
import os
import re

def update_jira_tickets(event, context):
    """Update JIRA tickets referenced in build commits."""
    # Parse message
    pubsub_message = base64.b64decode(event['data']).decode('utf-8')
    buildkite_event = json.loads(pubsub_message)
    build = buildkite_event['build']

    # Initialize JIRA client
    jira = JIRA(
        server=os.environ['JIRA_SERVER'],
        basic_auth=(os.environ['JIRA_USER'], os.environ['JIRA_TOKEN'])
    )

    # Extract JIRA tickets from commit messages
    ticket_pattern = re.compile(r'[A-Z]+-\d+')
    tickets = set(ticket_pattern.findall(build['message']))

    for ticket_id in tickets:
        # Add comment to each ticket
        comment = f"Build #{build['number']} {build['state']}\n{build['web_url']}"
        jira.add_comment(ticket_id, comment)

        # Update ticket status if build failed
        if build['state'] == 'failed':
            jira.transition_issue(ticket_id, 'Needs Review')
```

## Metrics and Analytics

### BigQuery Analytics

```python
from google.cloud import bigquery
import base64
import json
from datetime import datetime

def store_build_metrics(event, context):
    """Store build metrics in BigQuery for analysis."""
    client = bigquery.Client()
    table_id = "your-project.buildkite_metrics.builds"

    # Parse message
    pubsub_message = base64.b64decode(event['data']).decode('utf-8')
    buildkite_event = json.loads(pubsub_message)
    build = buildkite_event['build']

    # Calculate duration if available
    duration = None
    if build.get('started_at') and build.get('finished_at'):
        started = datetime.fromisoformat(build['started_at'].replace('Z', '+00:00'))
        finished = datetime.fromisoformat(build['finished_at'].replace('Z', '+00:00'))
        duration = (finished - started).total_seconds()

    row = {
        "build_id": build['id'],
        "pipeline": build['pipeline'],
        "state": build['state'],
        "duration_seconds": duration,
        "timestamp": build['finished_at'],
        "branch": build.get('branch'),
        "commit": build.get('commit')
    }

    errors = client.insert_rows_json(table_id, [row])
    if errors:
        raise Exception(f"BigQuery insert failed: {errors}")
```

## Data Storage

### Cloud Storage Archive

```python
from google.cloud import storage
import base64
import json
from datetime import datetime

def archive_build_data(event, context):
    """Archive build data to Cloud Storage with organized structure."""
    client = storage.Client()
    bucket = client.bucket(os.environ['ARCHIVE_BUCKET'])

    # Parse message
    pubsub_message = base64.b64decode(event['data']).decode('utf-8')
    buildkite_event = json.loads(pubsub_message)

    # Create organized path structure
    timestamp = datetime.fromisoformat(buildkite_event['build']['finished_at'].replace('Z', '+00:00'))
    path = (
        f"{timestamp.year:04d}/"
        f"{timestamp.month:02d}/"
        f"{timestamp.day:02d}/"
        f"{buildkite_event['build']['pipeline']}/"
        f"{buildkite_event['build']['number']}.json"
    )

    # Store with metadata
    blob = bucket.blob(path)
    blob.metadata = {
        'pipeline': buildkite_event['build']['pipeline'],
        'state': buildkite_event['build']['state'],
        'event_type': buildkite_event['event_type']
    }

    blob.upload_from_string(
        json.dumps(buildkite_event),
        content_type='application/json'
    )
```

## Best Practices

### 1. Error Handling

```python
from google.cloud.functions.context import Context
from typing import Any, Dict
import logging
import json
import base64

class ValidationError(Exception):
    pass

class RetryableError(Exception):
    pass

def robust_handler(event: Dict[str, Any], context: Context) -> None:
    """Template for robust event handling."""
    try:
        # Parse message safely
        try:
            pubsub_data = base64.b64decode(event['data']).decode('utf-8')
            message = json.loads(pubsub_data)
        except (KeyError, json.JSONDecodeError) as e:
            raise ValidationError(f"Invalid message format: {e}")

        # Validate required fields
        if 'event_type' not in message:
            raise ValidationError("Missing event_type")

        # Process with retries
        process_with_retries(message)

    except ValidationError as e:
        # Log and ignore invalid events
        logging.warning(f"Invalid event received: {e}")
        return  # Don't retry

    except RetryableError as e:
        # Retry on temporary failures
        logging.error(f"Temporary failure, will retry: {e}")
        raise  # Cloud Functions will retry

    except Exception as e:
        # Log unexpected errors
        logging.exception("Unexpected error")
        raise  # Cloud Functions will retry
```

### 2. Message Processing

```python
import time
from typing import Dict, Any
from tenacity import retry, stop_after_attempt, wait_exponential

@retry(
    stop=stop_after_attempt(3),
    wait=wait_exponential(multiplier=1, min=4, max=10)
)
def process_with_retries(message: Dict[str, Any]) -> None:
    """Process messages with retries and backoff."""
    # Extract event details
    event_type = message['event_type']
    build = message.get('build', {})

    # Process based on event type
    if event_type == 'build.finished':
        handle_build_finished(build)
    elif event_type == 'build.started':
        handle_build_started(build)
    else:
        logging.info(f"Ignoring event type: {event_type}")

def handle_build_finished(build: Dict[str, Any]) -> None:
    """Handle build.finished events."""
    # Implement your logic here
    pass

def handle_build_started(build: Dict[str, Any]) -> None:
    """Handle build.started events."""
    # Implement your logic here
    pass
```

### 3. Configuration Management

```python
from dataclasses import dataclass
from typing import Optional
import os

@dataclass
class Config:
    """Configuration for build event processor."""
    project_id: str
    notify_slack: bool
    slack_channel: Optional[str]
    archive_bucket: Optional[str]

    @classmethod
    def from_env(cls) -> 'Config':
        """Load configuration from environment variables."""
        return cls(
            project_id=os.environ['PROJECT_ID'],
            notify_slack=os.environ.get('NOTIFY_SLACK', '').lower() == 'true',
            slack_channel=os.environ.get('SLACK_CHANNEL'),
            archive_bucket=os.environ.get('ARCHIVE_BUCKET')
        )
```

### 4. Monitoring and Logging

```python
import logging
import time
from contextlib import contextmanager
from opencensus.ext.stackdriver import trace_exporter
from opencensus.trace import tracer

# Configure structured logging
logging.basicConfig(format='%(levelname)s - %(message)s')
logger = logging.getLogger(__name__)

# Configure tracing
tracer_exporter = trace_exporter.StackdriverExporter()
tracer = tracer.Tracer(exporter=tracer_exporter)

@contextmanager
def timed_operation(name: str):
    """Context manager for timing operations."""
    start = time.time()
    try:
        with tracer.span(name=name) as span:
            yield span
    finally:
        duration = time.time() - start
        logger.info(f"Operation {name} took {duration:.2f}s")
```

## Deployment

### Cloud Functions Deployment

```yaml
# function.yaml
runtime: python39
entry_point: process_build_event
env_variables:
  SLACK_CHANNEL: "#builds"
  ARCHIVE_BUCKET: "buildkite-archives"
  NOTIFY_SLACK: "true"

service_account_email: buildkite-processor@project.iam.gserviceaccount.com
```

```bash
# Deploy the function
gcloud functions deploy buildkite-processor \
  --trigger-topic buildkite-events \
  --runtime python39 \
  --env-vars-file function.yaml \
  --memory 256MB \
  --timeout 60s
```

Remember to grant appropriate IAM permissions to your service account for accessing resources like BigQuery, Cloud Storage, or other services used in your integrations.
