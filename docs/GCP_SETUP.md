# Google Cloud Setup Guide

This guide walks through setting up Google Cloud Project access for the Buildkite Webhook service. It covers creating a service account with the minimum required permissions.

## Prerequisites

- [Google Cloud SDK](https://cloud.google.com/sdk/docs/install) installed
- Access to a Google Cloud Project with owner or security admin permissions
- Project ID where you want to deploy the webhook service

## Service Account Setup

### 1. Set Environment Variables

```bash
# Set your project ID
export PROJECT_ID="your-project-id"
```

### 2. Create Service Account

```bash
# Create a new service account for the webhook service
gcloud iam service-accounts create buildkite-webhook \
    --description="Service account for Buildkite webhook" \
    --display-name="Buildkite Webhook"
```

### 3. Grant Required Permissions

#### Option A: Using Predefined Roles (Recommended for most users)

```bash
# For publishing messages only
gcloud projects add-iam-policy-binding $PROJECT_ID \
    --member="serviceAccount:buildkite-webhook@$PROJECT_ID.iam.gserviceaccount.com" \
    --role="roles/pubsub.publisher"

# Optional: For full Pub/Sub management (including topic creation)
# Only needed if you want the service to create topics automatically
gcloud projects add-iam-policy-binding $PROJECT_ID \
    --member="serviceAccount:buildkite-webhook@$PROJECT_ID.iam.gserviceaccount.com" \
    --role="roles/pubsub.admin"
```

#### Option B: Using Custom Role (More restrictive)

```bash
# Create a custom role with minimal permissions
gcloud iam roles create buildkite_webhook \
    --project=$PROJECT_ID \
    --title="Buildkite Webhook" \
    --description="Custom role for Buildkite webhook service" \
    --permissions="pubsub.topics.publish,pubsub.topics.get"

# Assign the custom role to the service account
gcloud projects add-iam-policy-binding $PROJECT_ID \
    --member="serviceAccount:buildkite-webhook@$PROJECT_ID.iam.gserviceaccount.com" \
    --role="projects/$PROJECT_ID/roles/buildkite_webhook"
```

### 4. Create and Download Credentials

```bash
# Create and download the service account key
gcloud iam service-accounts keys create credentials.json \
    --iam-account=buildkite-webhook@$PROJECT_ID.iam.gserviceaccount.com
```

### 5. Verify Setup

```bash
# List service account's IAM policy bindings
gcloud projects get-iam-policy $PROJECT_ID \
    --flatten="bindings[].members" \
    --format='table(bindings.role)' \
    --filter="bindings.members:buildkite-webhook@$PROJECT_ID.iam.gserviceaccount.com"

# Verify the credentials file exists
ls -l credentials.json
```

# Development Workflow

## Development Environment Setup

When developing locally, you'll want to verify your GCP configuration before starting the Kubernetes deployment:

### 1. Configure Development Project

```bash
# Set default project for development
gcloud config set project $PROJECT_ID

# Enable required APIs if not already enabled
gcloud services enable \
    pubsub.googleapis.com \
    monitoring.googleapis.com
```

### 2. Development Topic Setup

For development, create a separate topic to avoid interfering with production:

```bash
# Create development topic
gcloud pubsub topics create buildkite-events-dev

# Create test subscription for development
gcloud pubsub subscriptions create buildkite-events-dev-sub \
    --topic buildkite-events-dev \
    --message-retention-duration=1d \
    --expiration-period=7d
```

### 3. Local Environment Configuration

Set up your local environment for development:

```bash
# Configure credentials for local development
export GOOGLE_APPLICATION_CREDENTIALS="$(pwd)/credentials.json"
export PROJECT_ID="your-project-id"
export TOPIC_ID="buildkite-events-dev"  # Use development topic
```

### 4. Running Integration Tests

Before deploying to Kubernetes, validate your setup with integration tests:

```bash
# Run integration tests (requires valid credentials and development topic)
go test ./... -tags=integration
```

### Development vs Production Settings

| Setting | Development | Production |
|---------|-------------|------------|
| Topic Name | buildkite-events-dev | buildkite-events |
| Permissions | pubsub.publisher | Custom role or pubsub.admin |
| Monitoring | Basic metrics | Full monitoring stack |
| Retention | 1-7 days | Based on business needs |

### Transitioning to Production

When moving to production:

1. Create separate service accounts for dev and prod
2. Use more restrictive permissions in production
3. Enable audit logging for production topics
4. Set up appropriate retention policies
5. Configure monitoring and alerting

## Security Considerations

1. **Key Rotation**: Consider implementing regular key rotation for the service account
2. **Minimal Permissions**: The custom role option provides the most restrictive set of permissions
3. **Key Storage**: Store the credentials.json file securely and never commit it to version control

## Common Issues

1. **Permission Denied Errors**:
   - Verify the service account has the correct roles assigned
   - Check that the credentials file is mounted correctly in the pod
   - Ensure the PROJECT_ID matches the one where permissions were granted

2. **Missing Permissions**:
   ```bash
   # Check effective permissions for the service account
   gcloud iam service-accounts get-iam-policy \
       buildkite-webhook@$PROJECT_ID.iam.gserviceaccount.com
   ```

3. **Invalid Credentials**:
   - Verify the credentials file is valid JSON
   - Check the file permissions in the pod
   - Ensure GOOGLE_APPLICATION_CREDENTIALS is set correctly

## Next Steps

After completing this setup:

1. Store the credentials.json file securely
2. Follow the [Quick Start Guide](QUICK_START.md) to deploy the webhook service
3. Consider setting up monitoring for the service account's activity

## Cleanup

If you need to remove the service account:

```bash
# Delete the service account
gcloud iam service-accounts delete \
    buildkite-webhook@$PROJECT_ID.iam.gserviceaccount.com

# If you created a custom role, delete it
gcloud iam roles delete buildkite_webhook --project=$PROJECT_ID

# Remove the credentials file
rm credentials.json
```
