# Buildkite Events Reference

A quick reference for Buildkite webhook payloads and how to work with them. For a complete list of events, see the [Buildkite Webhooks Documentation](https://buildkite.com/docs/apis/webhooks#webhook-events).

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
Example of a `build.finished` event:
```json
{
  "event": "build.finished",
  "build": {
    "id": "01234567-89ab-cdef-0123-456789abcdef",
    "url": "https://api.buildkite.com/v2/organizations/example/pipelines/test/builds/123",
    "web_url": "https://buildkite.com/example/test/builds/123",
    "number": 123,
    "state": "passed",
    "pipeline": "test",
    "organization": "example"
  }
}
```

## Event Filtering

Filter Pub/Sub subscriptions based on event types:

```bash
# Failed builds only
gcloud pubsub subscriptions create failed-builds \
  --topic buildkite-events \
  --filter="attributes.event_type = 'build.finished' AND data.build.state = 'failed'"
```

## Additional Resources

- [Buildkite Webhooks Documentation](https://buildkite.com/docs/apis/webhooks)
- [Webhook Event Types](https://buildkite.com/docs/apis/webhooks#webhook-events)
- [REST API Reference](https://buildkite.com/docs/apis/rest-api)
