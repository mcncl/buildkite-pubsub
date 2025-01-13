# Quick Start Guide

Get started with the Buildkite PubSub Webhook service locally in under 10 minutes.

## Prerequisites

- [Go 1.20+](https://golang.org/dl/)
- [ngrok](https://ngrok.com/)
- A Buildkite organization (admin access)

## Setup

1. Clone the repository:
```bash
git clone <your-repo-url>
cd buildkite-pubsub
```

2. Generate a webhook token:
```bash
# You can use any secure random string
export BUILDKITE_WEBHOOK_TOKEN=$(openssl rand -hex 16)
```

3. Run the service:
```bash
go run cmd/webhook/main.go
```

4. In another terminal, start ngrok:
```bash
ngrok http 8080
```

5. Configure the webhook in Buildkite:
   - Go to Organization Settings → Webhooks
   - Click "Add Webhook"
   - URL: Your ngrok URL + `/webhook` (e.g., `https://abc123.ngrok.io/webhook`)
   - Events: Select "All Events" for testing
   - Token: Use the value from `$BUILDKITE_WEBHOOK_TOKEN`
   - SSL: Enable (ngrok provides SSL)

6. Test the webhook:
```bash
# Send a test ping using your ngrok URL
curl -X POST \
  -H "Content-Type: application/json" \
  -H "X-Buildkite-Token: $BUILDKITE_WEBHOOK_TOKEN" \
  https://your-ngrok-url/webhook \
  -d '{"event":"ping"}'

# Expected response:
# {"message":"Pong! Webhook received successfully"}
```

## Verify Webhook Events

1. Create a build in Buildkite
2. Watch the service logs to see the events
3. You should see build.scheduled, build.started, and build.finished events

## Next Steps

- Ready to deploy to GCP? See the [GCP Setup Guide](GCP_SETUP.md)
- Want to explore event types? Check the [Usage Guide](USAGE.md)
- Need monitoring? See the [Monitoring Guide](MONITORING.md)
