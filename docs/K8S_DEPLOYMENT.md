# Kubernetes Deployment

Deploy the webhook service to Kubernetes. Assumes you've completed [GCP Setup](GCP_SETUP.md).

## Prerequisites

- Kubernetes cluster (local: Orbstack/Docker Desktop, or GKE)
- `kubectl` configured
- Docker for building images

## Setup

### 1. Create Namespace and Secrets

```bash
kubectl create namespace buildkite-webhook

# Webhook token
kubectl create secret generic buildkite-webhook-secrets \
  --namespace buildkite-webhook \
  --from-literal=buildkite-token="your-token"

# GCP credentials
kubectl create secret generic gcp-credentials \
  --namespace buildkite-webhook \
  --from-file=credentials.json

# Configuration
kubectl create configmap buildkite-webhook-config \
  --namespace buildkite-webhook \
  --from-literal=project_id="your-project-id" \
  --from-literal=topic_id="buildkite-events"
```

### 2. Build and Deploy

```bash
# Build image
docker build -t buildkite-webhook .

# For local cluster
docker tag buildkite-webhook localhost:5000/buildkite-webhook:latest
docker push localhost:5000/buildkite-webhook:latest

# For GKE
docker tag buildkite-webhook gcr.io/$PROJECT_ID/buildkite-webhook:latest
docker push gcr.io/$PROJECT_ID/buildkite-webhook:latest

# Deploy
kubectl apply -f k8s/
```

### 3. Verify

```bash
kubectl get pods -n buildkite-webhook
kubectl get svc -n buildkite-webhook
```

## Testing

```bash
# Port forward
kubectl port-forward -n buildkite-webhook svc/buildkite-webhook 8080:80

# Test webhook
curl -X POST http://localhost:8080/webhook \
  -H "X-Buildkite-Token: your-token" \
  -H "Content-Type: application/json" \
  -d '{"event":"ping"}'
```

## Cleanup

```bash
kubectl delete namespace buildkite-webhook
```
