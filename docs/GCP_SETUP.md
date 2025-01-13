# Google Cloud Setup Guide

Deploy the Buildkite PubSub Webhook service to Google Cloud Platform.

## Prerequisites

- [Google Cloud SDK](https://cloud.google.com/sdk/docs/install)
- A Google Cloud Project with billing enabled
- Project Owner or Editor role

## Part 1: Basic GCP Setup

### 1. Configure Project

```bash
# Set project ID
export PROJECT_ID="your-project-id"
gcloud config set project $PROJECT_ID

# Enable required APIs
gcloud services enable \
  pubsub.googleapis.com \
  container.googleapis.com \
  monitoring.googleapis.com \
  run.googleapis.com \
  cloudbuild.googleapis.com
```

### 2. Create Service Account

```bash
# Create service account
gcloud iam service-accounts create buildkite-webhook \
  --description="Service account for Buildkite webhook" \
  --display-name="Buildkite Webhook"

# Grant permissions
gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="serviceAccount:buildkite-webhook@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/pubsub.publisher"

gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="serviceAccount:buildkite-webhook@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/monitoring.metricWriter"

# Download credentials (for local testing)
gcloud iam service-accounts keys create credentials.json \
  --iam-account=buildkite-webhook@$PROJECT_ID.iam.gserviceaccount.com
```

### 3. Set Up Pub/Sub

```bash
# Create topic
export TOPIC_ID="buildkite-events"
gcloud pubsub topics create $TOPIC_ID

# Create subscription for testing
gcloud pubsub subscriptions create buildkite-events-sub \
  --topic $TOPIC_ID \
  --message-retention-duration="7d"
```

### 4. Store Webhook Token

```bash
# Create secret for webhook token
echo -n "your-webhook-token" | \
  gcloud secrets create buildkite-webhook-token \
  --data-file=- \
  --replication-policy="automatic"

# Grant access to the service account
gcloud secrets add-iam-policy-binding buildkite-webhook-token \
  --member="serviceAccount:buildkite-webhook@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/secretmanager.secretAccessor"
```

## Part 2: Cloud Run Deployment

### 1. Build and Push Image

```bash
# Build using Cloud Build
gcloud builds submit --tag gcr.io/$PROJECT_ID/buildkite-webhook

# Verify image
gcloud container images list-tags gcr.io/$PROJECT_ID/buildkite-webhook
```

### 2. Deploy to Cloud Run

```bash
# Deploy the service
gcloud run deploy buildkite-webhook \
  --image gcr.io/$PROJECT_ID/buildkite-webhook \
  --platform managed \
  --region australia-southeast2 \
  --allow-unauthenticated \
  --service-account buildkite-webhook@$PROJECT_ID.iam.gserviceaccount.com \
  --set-env-vars="PROJECT_ID=$PROJECT_ID,TOPIC_ID=buildkite-events,ENABLE_METRICS=true,PROMETHEUS_NAMESPACE=buildkite" \
  --set-secrets="BUILDKITE_WEBHOOK_TOKEN=buildkite-webhook-token:latest"

# Get the service URL
export SERVICE_URL=$(gcloud run services describe buildkite-webhook \
  --platform managed \
  --region australia-southeast2 \
  --format='get(status.url)')
```

### 3. Test Cloud Run Deployment

```bash
# Send test webhook
curl -X POST \
  -H "Content-Type: application/json" \
  -H "X-Buildkite-Token: your-webhook-token" \
  "$SERVICE_URL/webhook" \
  -d '{"event":"ping"}'

# Check messages
gcloud pubsub subscriptions pull buildkite-events-sub --auto-ack
```

## Part 3: GKE Deployment (Optional)

If you need more control over your deployment or want to run alongside other services, you can deploy to GKE:

### 1. Create GKE Cluster

```bash
# Create cluster
gcloud container clusters create buildkite-webhook \
  --machine-type=e2-standard-2 \
  --num-nodes=2 \
  --zone=us-central1-a \
  --workload-pool=$PROJECT_ID.svc.id.goog

# Get credentials
gcloud container clusters get-credentials buildkite-webhook \
  --zone=us-central1-a
```

### 2. Configure Workload Identity

```bash
# Create namespace
kubectl create namespace buildkite-webhook

# Create Kubernetes service account
kubectl create serviceaccount buildkite-webhook \
  --namespace buildkite-webhook

# Configure workload identity binding
gcloud iam service-accounts add-iam-policy-binding \
  buildkite-webhook@$PROJECT_ID.iam.gserviceaccount.com \
  --role roles/iam.workloadIdentityUser \
  --member "serviceAccount:$PROJECT_ID.svc.id.goog[buildkite-webhook/buildkite-webhook]"

kubectl annotate serviceaccount buildkite-webhook \
  --namespace buildkite-webhook \
  iam.gke.io/gcp-service-account=buildkite-webhook@$PROJECT_ID.iam.gserviceaccount.com
```

### 3. Deploy to GKE

```bash
# Create ConfigMap
kubectl create configmap buildkite-webhook-config \
  --namespace buildkite-webhook \
  --from-literal=project_id="$PROJECT_ID" \
  --from-literal=topic_id="$TOPIC_ID"

# Create secret for webhook token
kubectl create secret generic buildkite-webhook-secrets \
  --namespace buildkite-webhook \
  --from-literal=buildkite-token="your-webhook-token"

# Deploy application
kubectl apply -f k8s/webhook/

# Create static IP for ingress
gcloud compute addresses create buildkite-webhook \
  --global

# Note the IP address
export WEBHOOK_IP=$(gcloud compute addresses describe buildkite-webhook \
  --global --format='get(address)')

# Deploy ingress (after configuring DNS)
kubectl apply -f k8s/ingress/
```

## Cleaning Up

### Cloud Run Cleanup
```bash
# Delete Cloud Run service
gcloud run services delete buildkite-webhook \
  --platform managed \
  --region australia-southeast2

# Delete container image
gcloud container images delete gcr.io/$PROJECT_ID/buildkite-webhook --force-delete-tags
```

### GKE Cleanup
```bash
# Delete GKE resources
kubectl delete namespace buildkite-webhook

# Delete cluster
gcloud container clusters delete buildkite-webhook \
  --zone=us-central1-a

# Delete static IP
gcloud compute addresses delete buildkite-webhook --global
```

### Common Resources Cleanup
```bash
# Delete Pub/Sub resources
gcloud pubsub subscriptions delete buildkite-events-sub
gcloud pubsub topics delete $TOPIC_ID

# Delete secrets
gcloud secrets delete buildkite-webhook-token

# Delete service account
gcloud iam service-accounts delete \
  buildkite-webhook@$PROJECT_ID.iam.gserviceaccount.com
```
