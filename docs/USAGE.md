# Usage Guide

This guide provides examples and patterns for working with Buildkite events once they're published to Pub/Sub.

## Event Structure

Events are published with the following attributes:
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

## Common Patterns

### 1. Event Filtering

Filter events using Pub/Sub subscription filters:

```bash
# Filter for specific event types
gcloud pubsub subscriptions create build-finished \
  --topic buildkite-events \
  --filter="attributes.event_type = \"build.finished\""

# Filter for specific organization
gcloud pubsub subscriptions create org-builds \
  --topic buildkite-events \
  --filter="attributes.event_type = \"build.finished\" AND organization = \"your-org\""
```

### 2. Event Storage

#### BigQuery Storage
```bash
# Create dataset and table
bq mk --dataset \
  your-project:buildkite_events

bq mk --table \
  --time_partitioning_field timestamp \
  your-project:buildkite_events.builds \
  ./schemas/builds.json
```

Example Cloud Function to store events:
```python
from google.cloud import bigquery

def store_event(event, context):
    client = bigquery.Client()
    dataset_id = "buildkite_events"
    table_id = "builds"

    table = client.get_table(f"{dataset_id}.{table_id}")
    rows_to_insert = [{
        "event_id": context.event_id,
        "timestamp": context.timestamp,
        "data": event
    }]

    client.insert_rows(table, rows_to_insert)
```

#### Cloud Storage Archive
```python
from google.cloud import storage

def archive_event(event, context):
    client = storage.Client()
    bucket = client.get_bucket("buildkite-events-archive")

    # Organize by year/month/day
    date = context.timestamp.split("T")[0]
    year, month, day = date.split("-")
    path = f"{year}/{month}/{day}/{context.event_id}.json"

    blob = bucket.blob(path)
    blob.upload_from_string(json.dumps(event))
```

### 3. Event Processing

#### Slack Notifications
```python
import os
from slack_sdk import WebClient

def notify_slack(event, context):
    if event["build"]["state"] != "failed":
        return

    client = WebClient(token=os.environ["SLACK_TOKEN"])

    client.chat_postMessage(
        channel="#builds",
        text=f"Build failed: {event['build']['web_url']}"
    )
```

#### GitHub Status Updates
```python
import os
import requests

def update_github_status(event, context):
    if "github" not in event["build"]["source"]:
        return

    token = os.environ["GITHUB_TOKEN"]
    headers = {
        "Authorization": f"token {token}",
        "Accept": "application/vnd.github.v3+json"
    }

    state = "success" if event["build"]["state"] == "passed" else "failure"

    requests.post(
        event["build"]["source"]["github"]["status_url"],
        headers=headers,
        json={
            "state": state,
            "description": f"Build #{event['build']['number']}",
            "target_url": event["build"]["web_url"]
        }
    )
```

### 4. Monitoring & Alerting

#### Cloud Monitoring Dashboard
```terraform
resource "google_monitoring_dashboard" "buildkite" {
  dashboard_json = jsonencode({
    displayName = "Buildkite Events Dashboard"
    gridLayout = {
      widgets = [
        {
          title = "Events by Type"
          xyChart = {
            dataSets = [{
              timeSeriesQuery = {
                timeSeriesFilter = {
                  filter = "metric.type=\"pubsub.googleapis.com/subscription/delivered_messages\""
                  aggregation = {
                    groupByFields = ["metric.labels.event_type"]
                  }
                }
              }
            }]
          }
        }
      ]
    }
  })
}
```

#### Alert Policies
```terraform
resource "google_monitoring_alert_policy" "failed_builds" {
  display_name = "Failed Builds Alert"
  conditions {
    display_name = "High Build Failure Rate"
    condition_threshold {
      filter = "metric.type=\"pubsub.googleapis.com/subscription/delivered_messages\" AND metric.labels.event_type=\"build.finished\" AND metric.labels.state=\"failed\""
      duration = "300s"
      comparison = "COMPARISON_GT"
      threshold_value = 5
    }
  }

  notification_channels = [
    google_monitoring_notification_channel.email.name
  ]
}
```

## Best Practices

1. **Event Retention**
   - Set appropriate message retention on subscriptions
   - Archive important events to Cloud Storage/BigQuery
   - Consider cost vs retention needs

2. **Error Handling**
   - Use dead-letter topics for failed message processing
   - Implement exponential backoff for retries
   - Log errors with context for debugging

3. **Performance**
   - Use message filtering at subscription level when possible
   - Batch inserts for storage operations
   - Monitor subscription backlog

4. **Security**
   - Use service accounts with minimal permissions
   - Encrypt sensitive data before storage
   - Regularly rotate credentials

5. **Monitoring**
   - Set up alerts for critical failures
   - Monitor message processing latency
   - Track error rates and types

## Troubleshooting

Common issues and solutions:

1. **Message Processing Delays**
   - Check subscription backlog
   - Verify Cloud Function/Cloud Run scaling
   - Check resource quotas

2. **Missing Events**
   - Verify webhook delivery in Buildkite
   - Check Pub/Sub message retention
   - Verify subscription filters

3. **Error Processing**
   - Check Cloud Function logs
   - Verify service account permissions
   - Check resource constraints

For more examples and detailed documentation, check out:
- [Cloud Functions Examples](https://github.com/GoogleCloudPlatform/python-docs-samples/tree/main/functions)
- [Pub/Sub Documentation](https://cloud.google.com/pubsub/docs)
- [Cloud Run Documentation](https://cloud.google.com/run/docs)
