# Google Cloud Setup Guide

## Prerequisites

- [Google Cloud SDK](https://cloud.google.com/sdk/docs/install) installed
- Active Google Cloud account with project access
- `gcloud` CLI configured

## 1. Initial Setup and Authentication

```bash
# Log in to Google Cloud
gcloud auth login

# List available projects
gcloud projects list

# Set the project you want to work with
export PROJECT_ID="your-project-id"
gcloud config set project $PROJECT_ID
```

## 2. Required IAM Permissions

Ensure you have these roles (ask project admin if needed):

```bash
# Check your current permissions
gcloud projects get-iam-policy $PROJECT_ID \
    --flatten="bindings[].members" \
    --format='table(bindings.role)' \
    --filter="bindings.members:$(gcloud config get-value account)"

# If missing, project admin should run:
gcloud projects add-iam-policy-binding $PROJECT_ID \
    --member="user:your_email@example.com" \
    --role="roles/pubsub.admin"

gcloud projects add-iam-policy-binding $PROJECT_ID \
    --member="user:your_email@example.com" \
    --role="roles/iam.serviceAccountAdmin"

gcloud projects add-iam-policy-binding $PROJECT_ID \
    --member="user:your_email@example.com" \
    --role="roles/secretmanager.admin"
```

## 3. Enable Required APIs

```bash
# Enable necessary APIs
gcloud services enable \
    pubsub.googleapis.com \
    secretmanager.googleapis.com \
    run.googleapis.com \
    containerregistry.googleapis.com
```

## 4. Create Pub/Sub Topics

```bash
# Set topic name
export TOPIC_ID="buildkite-events"

# Create the main topic
gcloud pubsub topics create $TOPIC_ID

# Verify creation
gcloud pubsub topics list | grep $TOPIC_ID
```

### Optional: Create Dead Letter Queue Topic

If you want to capture failed messages for later analysis:

```bash
# Create DLQ topic
export DLQ_TOPIC_ID="buildkite-events-dlq"
gcloud pubsub topics create $DLQ_TOPIC_ID

# Create a subscription to review failed messages
gcloud pubsub subscriptions create buildkite-dlq-sub \
    --topic=$DLQ_TOPIC_ID \
    --ack-deadline=60
```

## 5. Create Service Account

```bash
# Create service account
export SERVICE_ACCOUNT_NAME="buildkite-webhook"

gcloud iam service-accounts create $SERVICE_ACCOUNT_NAME \
    --description="Service account for Buildkite webhook" \
    --display-name="Buildkite Webhook"

# Grant Pub/Sub permissions
# Publisher role for publishing messages
gcloud projects add-iam-policy-binding $PROJECT_ID \
    --member="serviceAccount:${SERVICE_ACCOUNT_NAME}@${PROJECT_ID}.iam.gserviceaccount.com" \
    --role="roles/pubsub.publisher"

# Viewer role for topic existence checks
gcloud projects add-iam-policy-binding $PROJECT_ID \
    --member="serviceAccount:${SERVICE_ACCOUNT_NAME}@${PROJECT_ID}.iam.gserviceaccount.com" \
    --role="roles/pubsub.viewer"

# Create and download service account key
gcloud iam service-accounts keys create credentials.json \
    --iam-account=${SERVICE_ACCOUNT_NAME}@${PROJECT_ID}.iam.gserviceaccount.com

# Verify service account
gcloud iam service-accounts list | grep $SERVICE_ACCOUNT_NAME
```

## 6. Create Buildkite Webhook Secret

The webhook uses HMAC signature verification for secure authentication. You'll need the signing secret from your Buildkite webhook configuration.

```bash
# Store your Buildkite webhook HMAC signing secret securely
# Get this from: Buildkite → Settings → Notification Services → Webhooks → Signing Secret
echo -n "YOUR_BUILDKITE_SIGNING_SECRET" | \
    gcloud secrets create buildkite-webhook-hmac-secret \
    --replication-policy="automatic" \
    --data-file=-

# Grant service account access to secret
gcloud secrets add-iam-policy-binding buildkite-webhook-hmac-secret \
    --member="serviceAccount:${SERVICE_ACCOUNT_NAME}@${PROJECT_ID}.iam.gserviceaccount.com" \
    --role="roles/secretmanager.secretAccessor"
```

## 7. Set Environment Variables for Local Testing

Create a `.env` file:

```bash
# Create environment file
cat > .env << EOF
# GCP Configuration
PROJECT_ID=$PROJECT_ID
TOPIC_ID=$TOPIC_ID
GOOGLE_APPLICATION_CREDENTIALS=./credentials.json

# Buildkite HMAC signing secret (from Buildkite webhook settings)
BUILDKITE_WEBHOOK_HMAC_SECRET=your-signing-secret-here

# Tracing (optional)
ENABLE_TRACING=true
OTEL_SERVICE_NAME=buildkite-webhook
OTEL_EXPORTER_OTLP_ENDPOINT=https://api.honeycomb.io
OTEL_EXPORTER_OTLP_HEADERS="x-honeycomb-team=your-honeycomb-key"

# Server
PORT=8888
EOF
```

## 8. Test Local Setup

```bash
# Load environment variables
export $(grep -v '^#' .env | xargs)

# Run the webhook locally
go run cmd/webhook/main.go

# In another terminal, test with a sample webhook
# Note: HMAC validation requires computing a signature. For local testing,
# you can either configure a real Buildkite webhook to point to your local
# server (via ngrok/cloudflared), or temporarily set BUILDKITE_WEBHOOK_TOKEN
# for simpler token-based auth during development.
```

## 9. Cloud Run Deployment (Optional)

```bash
# Configure Docker for GCR
gcloud auth configure-docker gcr.io

# Build and push container (--platform ensures correct architecture for Cloud Run)
docker build --platform linux/amd64 -t gcr.io/$PROJECT_ID/buildkite-webhook .
docker push gcr.io/$PROJECT_ID/buildkite-webhook

# Deploy to Cloud Run
export REGION="us-central1"  # Choose your preferred region

gcloud run deploy buildkite-webhook \
  --image gcr.io/$PROJECT_ID/buildkite-webhook \
  --platform managed \
  --region $REGION \
  --allow-unauthenticated \
  --service-account=${SERVICE_ACCOUNT_NAME}@${PROJECT_ID}.iam.gserviceaccount.com \
  --set-env-vars="PROJECT_ID=$PROJECT_ID,TOPIC_ID=$TOPIC_ID,ENABLE_TRACING=true" \
  --set-secrets="BUILDKITE_WEBHOOK_HMAC_SECRET=buildkite-webhook-hmac-secret:latest"

# Optional: Enable Dead Letter Queue
# Add these environment variables to capture failed messages:
# --update-env-vars="ENABLE_DLQ=true,DLQ_TOPIC_ID=buildkite-events-dlq"

# Get the service URL
gcloud run services describe buildkite-webhook --region $REGION --format 'value(status.url)'
```

## 10. Configure Distributed Tracing (Optional)

### Honeycomb Setup
```bash
# Add Honeycomb environment variables for tracing
gcloud run services update buildkite-webhook \
  --region $REGION \
  --update-env-vars="ENABLE_TRACING=true,OTEL_SERVICE_NAME=buildkite-webhook,OTEL_ENVIRONMENT=production,OTEL_EXPORTER_OTLP_ENDPOINT=https://api.honeycomb.io,OTEL_EXPORTER_OTLP_HEADERS=x-honeycomb-team=YOUR_HONEYCOMB_API_KEY"
```

### Alternative: Local Jaeger
```bash
# For development with local Jaeger
gcloud run services update buildkite-webhook \
  --region $REGION \
  --update-env-vars="ENABLE_TRACING=true,OTEL_SERVICE_NAME=buildkite-webhook,OTEL_ENVIRONMENT=development,OTEL_EXPORTER_OTLP_ENDPOINT=localhost:14250"
```

See [Distributed Tracing Guide](DISTRIBUTED_TRACING.md) for detailed setup instructions.

## 11. Verification

```bash
# Check all resources are created
echo "Project: $PROJECT_ID"
echo "Topic: $(gcloud pubsub topics list --filter=name:$TOPIC_ID --format='value(name)')"
echo "Service Account: $(gcloud iam service-accounts list --filter=email:${SERVICE_ACCOUNT_NAME}@${PROJECT_ID}.iam.gserviceaccount.com --format='value(email)')"
echo "Secret: $(gcloud secrets list --filter=name:buildkite-webhook-hmac-secret --format='value(name)')"
echo "Service URL: $(gcloud run services describe buildkite-webhook --region $REGION --format='value(status.url)')"
```

### Test Webhook

Configure Buildkite to send webhooks to your service URL:

1. Go to **Buildkite → Settings → Notification Services → Add → Webhook**
2. Set the **Webhook URL** to: `${SERVICE_URL}/webhook`
3. Copy the **Signing Secret** (this should match what you stored in GCP Secret Manager)
4. Select the events you want to receive
5. Save and test with the "Send Test" button

## Security Notes

- Keep `credentials.json` secure and never commit to version control
- Add `credentials.json` to your `.gitignore`
- Rotate service account keys periodically
- Use minimal required permissions
- HMAC signature verification protects against replay attacks (5-minute window)

## Cleanup

Use the cleanup script when done:

```bash
# For full cleanup (requires admin permissions)
./scripts/gcp_cleanup --project $PROJECT_ID

# Or manual cleanup of what you can delete
gcloud pubsub topics delete $TOPIC_ID --quiet
# (Service accounts and secrets require admin permissions)
```

## Troubleshooting

- Ensure you're using the correct project ID
- Verify all APIs are enabled
- Check IAM permissions if commands fail
- Confirm service account key file exists and is readable
- For permission errors, contact your project administrator