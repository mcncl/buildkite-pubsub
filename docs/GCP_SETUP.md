# Google Cloud Setup Guide

## Prerequisites

Before beginning, ensure you have:
- [Google Cloud SDK](https://cloud.google.com/sdk/docs/install) installed
- An active Google Cloud account with project creation/management permissions
- `gcloud` CLI configured
- Docker installed

## Initial Setup and Authentication

### 1. Authenticate and Set Up gcloud

```bash
# Log in to Google Cloud
gcloud auth login

# List available projects
gcloud projects list

# Set the project you want to work with
gcloud config set project YOUR_PROJECT_ID
```

### 2. Verify and Configure IAM Permissions

Before creating resources, ensure you have the necessary IAM roles:

```bash
# Check current user's roles
gcloud auth list
gcloud projects get-iam-policy YOUR_PROJECT_ID \
    --flatten="bindings[].members" \
    --format='table(bindings.role)' \
    --filter="bindings.members:$(gcloud config get-value account)"
```

### 3. Assign Required IAM Roles

If you lack the necessary permissions, have a project administrator assign these roles:

```bash
# Commands to be run by a project administrator
gcloud projects add-iam-policy-binding YOUR_PROJECT_ID \
    --member="user:your_email@example.com" \
    --role="roles/pubsub.admin"

gcloud projects add-iam-policy-binding YOUR_PROJECT_ID \
    --member="user:your_email@example.com" \
    --role="roles/iam.serviceAccountAdmin"

gcloud projects add-iam-policy-binding YOUR_PROJECT_ID \
    --member="user:your_email@example.com" \
    --role="roles/secretmanager.admin"
```

## Pub/Sub Configuration

### 1. Enable Required APIs

```bash
# Enable necessary APIs
gcloud services enable \
    pubsub.googleapis.com \
    secretmanager.googleapis.com \
    run.googleapis.com \
    containerregistry.googleapis.com
```

### 2. Create Pub/Sub Topic

```bash
# Set topic name
export PROJECT_ID="your-project-id"
export TOPIC_ID="buildkite-events"

# Create the topic
gcloud pubsub topics create $TOPIC_ID
```

## Service Account Setup

### 1. Create Service Account

```bash
# Create service account
export SERVICE_ACCOUNT_NAME="buildkite-webhook"

gcloud iam service-accounts create $SERVICE_ACCOUNT_NAME \
    --description="Service account for Buildkite webhook" \
    --display-name="Buildkite Webhook"
```

### 2. Assign Permissions

```bash
# Pub/Sub Publisher role
gcloud projects add-iam-policy-binding $PROJECT_ID \
    --member="serviceAccount:${SERVICE_ACCOUNT_NAME}@${PROJECT_ID}.iam.gserviceaccount.com" \
    --role="roles/pubsub.publisher"

# Optional additional roles as needed
gcloud projects add-iam-policy-binding $PROJECT_ID \
    --member="serviceAccount:${SERVICE_ACCOUNT_NAME}@${PROJECT_ID}.iam.gserviceaccount.com" \
    --role="roles/cloudrun.invoker"
```

## Secrets Management

### 1. Create Buildkite Webhook Secret

```bash
# Create secret for Buildkite webhook token
echo -n "YOUR_BUILDKITE_WEBHOOK_TOKEN" | \
    gcloud secrets create buildkite-webhook-token \
    --replication-policy="automatic" \
    --data-file=-

# Grant service account access to secret
gcloud secrets add-iam-policy-binding buildkite-webhook-token \
    --member="serviceAccount:${SERVICE_ACCOUNT_NAME}@${PROJECT_ID}.iam.gserviceaccount.com" \
    --role="roles/secretmanager.secretAccessor"
```

## Container Build and Push

### 1. Build Container

```bash
# Configure Docker to use Google Cloud Registry
gcloud auth configure-docker

# Build the container
docker build -t gcr.io/$PROJECT_ID/buildkite-webhook .

# Push to Google Container Registry
docker push gcr.io/$PROJECT_ID/buildkite-webhook
```

## Cloud Run Deployment

### 1. Deploy to Cloud Run

```bash
# Set deployment region
export REGION="your-preferred-region"  # e.g., us-central1, australia-southeast2

# Deploy the service
gcloud run deploy buildkite-webhook \
  --image gcr.io/$PROJECT_ID/buildkite-webhook \
  --platform managed \
  --region $REGION \
  --allow-unauthenticated \
  --service-account=${SERVICE_ACCOUNT_NAME}@${PROJECT_ID}.iam.gserviceaccount.com \
  --set-env-vars="PROJECT_ID=$PROJECT_ID,TOPIC_ID=$TOPIC_ID,ENABLE_METRICS=true" \
  --set-secrets="BUILDKITE_WEBHOOK_TOKEN=buildkite-webhook-token:latest"
```

### 2. Verify Deployment

```bash
# List deployed services
gcloud run services list

# Get service URL
gcloud run services describe buildkite-webhook --region $REGION --format 'value(status.url)'
```

## Post-Deployment Checklist

1. Configure Buildkite webhook with the generated Cloud Run URL
2. Test webhook functionality
3. Review Cloud Run service logs
4. Set up monitoring and alerting

## Cleanup and Maintenance

Refer to the project's cleanup script for removing resources when no longer needed.

## Troubleshooting

- Verify all steps are completed in order
- Check IAM permissions
- Ensure APIs are enabled
- Review service account roles
- Check secret and environment variable configurations
## Cloud Run Deployment

### Build and Push Container

```bash
# Set project-specific variables
export PROJECT_ID="your-project-id"
export REGION="your-preferred-region"  # e.g., australia-southeast2, us-central1, europe-west1

# Configure Docker to use Google Cloud Registry
gcloud auth configure-docker

# Build the container
docker build -t gcr.io/$PROJECT_ID/buildkite-webhook .

# Push to Google Container Registry
docker push gcr.io/$PROJECT_ID/buildkite-webhook
```

### Deployment Considerations
- Choose a region close to your primary infrastructure
- Common regions include:
  * `us-central1` (Iowa, USA)
  * `us-east1` (South Carolina, USA)
  * `europe-west1` (Belgium)
  * `asia-southeast1` (Singapore)
  * `australia-southeast2` (Melbourne, Australia)

### Deploy to Cloud Run

```bash
# Deploy the service
gcloud run deploy buildkite-webhook \
  --image gcr.io/$PROJECT_ID/buildkite-webhook \
  --platform managed \
  --region $REGION \
  --allow-unauthenticated \
  --service-account buildkite-webhook-v2@$PROJECT_ID.iam.gserviceaccount.com \
  --set-env-vars="PROJECT_ID=$PROJECT_ID,TOPIC_ID=buildkite-events,ENABLE_METRICS=true,PROMETHEUS_NAMESPACE=buildkite" \
  --set-secrets="BUILDKITE_WEBHOOK_TOKEN=buildkite-webhook-token:latest"
```

### Verify Deployment

```bash
# List deployed services
gcloud run services list

# Get service URL
gcloud run services describe buildkite-webhook --region $REGION --format 'value(status.url)'
```

## Post-Deployment Steps

1. Configure Buildkite webhook to use the generated Cloud Run URL
2. Verify webhook functionality
3. Set up monitoring and logging

### Troubleshooting Deployment

- Ensure service account has correct permissions
- Check container build logs
- Verify environment variables
- Review Cloud Run service logs
# Google Cloud Setup Guide

## Prerequisites

Before beginning, ensure you have:
- [Google Cloud SDK](https://cloud.google.com/sdk/docs/install) installed
- An active Google Cloud account with project creation/management permissions
- `gcloud` CLI configured

## Initial Setup and Authentication

### 1. Authenticate and Set Up gcloud

```bash
# Log in to Google Cloud
gcloud auth login

# List available projects
gcloud projects list

# Set the project you want to work with
gcloud config set project YOUR_PROJECT_ID
```

### 2. Verify and Configure IAM Permissions

Before creating resources, ensure you have the necessary IAM roles:

```bash
# Check current user's roles
gcloud auth list
gcloud projects get-iam-policy YOUR_PROJECT_ID \
    --flatten="bindings[].members" \
    --format='table(bindings.role)' \
    --filter="bindings.members:$(gcloud config get-value account)"
```

### 3. Assign Required IAM Roles

If you lack the necessary permissions, have a project administrator assign these roles:

```bash
# Commands to be run by a project administrator
gcloud projects add-iam-policy-binding YOUR_PROJECT_ID \
    --member="user:your_email@example.com" \
    --role="roles/owner"

# For a more restricted approach, assign these roles:
gcloud projects add-iam-policy-binding YOUR_PROJECT_ID \
    --member="user:your_email@example.com" \
    --role="roles/pubsub.admin"

gcloud projects add-iam-policy-binding YOUR_PROJECT_ID \
    --member="user:your_email@example.com" \
    --role="roles/iam.serviceAccountAdmin"
```

## Service Account Setup

### 1. Create Service Account

```bash
# Set environment variables
export PROJECT_ID="your-project-id"
export SERVICE_ACCOUNT_NAME="buildkite-webhook"

# Create service account
gcloud iam service-accounts create $SERVICE_ACCOUNT_NAME \
    --description="Service account for Buildkite webhook" \
    --display-name="Buildkite Webhook"
```

### 2. Assign Permissions

```bash
# Pub/Sub Publisher role (minimal permissions)
gcloud projects add-iam-policy-binding $PROJECT_ID \
    --member="serviceAccount:${SERVICE_ACCOUNT_NAME}@${PROJECT_ID}.iam.gserviceaccount.com" \
    --role="roles/pubsub.publisher"

# Optional: For full Pub/Sub management
gcloud projects add-iam-policy-binding $PROJECT_ID \
    --member="serviceAccount:${SERVICE_ACCOUNT_NAME}@${PROJECT_ID}.iam.gserviceaccount.com" \
    --role="roles/pubsub.admin"
```

### 3. Create and Download Service Account Key

```bash
# Create service account key
gcloud iam service-accounts keys create credentials.json \
    --iam-account=${SERVICE_ACCOUNT_NAME}@${PROJECT_ID}.iam.gserviceaccount.com
```

## Pub/Sub Configuration

### 1. Enable Pub/Sub API

```bash
# Enable Pub/Sub API
gcloud services enable pubsub.googleapis.com
```

### 2. Create Pub/Sub Topic

```bash
# Set topic name
export TOPIC_ID="buildkite-events"

# Create the topic
gcloud pubsub topics create $TOPIC_ID
```

### 3. Create Pub/Sub Subscription (Optional)

```bash
# Create a subscription
gcloud pubsub subscriptions create buildkite-webhook-sub \
    --topic=$TOPIC_ID
```

## Verification

```bash
# Verify service account
gcloud iam service-accounts list | grep $SERVICE_ACCOUNT_NAME

# Verify Pub/Sub topic
gcloud pubsub topics list | grep $TOPIC_ID
```

## Security Considerations

- Keep the `credentials.json` file secure
- Never commit this file to version control
- Rotate service account keys periodically
- Use the principle of least privilege

## Cleanup

If you need to remove resources:

```bash
# Delete service account
gcloud iam service-accounts delete ${SERVICE_ACCOUNT_NAME}@${PROJECT_ID}.iam.gserviceaccount.com

# Delete Pub/Sub topic
gcloud pubsub topics delete $TOPIC_ID
```

## Troubleshooting

- Ensure you're using the correct project
- Verify API is enabled
- Check IAM permissions
- Confirm service account exists
