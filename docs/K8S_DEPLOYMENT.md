# Kubernetes Deployment Guide

This guide walks through deploying the Buildkite PubSub webhook service to Kubernetes. It assumes you've already completed the [GCP Setup](GCP_SETUP.md) and have your Google Cloud credentials ready.

## Prerequisites

- Kubernetes cluster access (this guide uses local development with Orbstack)
- `kubectl` CLI tool installed
- Docker for building container images
- Completed [GCP Setup](GCP_SETUP.md) with credentials
- Basic understanding of Kubernetes concepts
- `gcloud` CLI tool installed and configured

### GCP Authentication

If you're connecting to a GKE (Google Kubernetes Engine) cluster, you'll need to authenticate:

```bash
# Log in to Google Cloud
gcloud auth login

# Set your project ID
gcloud config set project YOUR_PROJECT_ID

# Get credentials for your cluster
# Replace CLUSTER_NAME and COMPUTE_ZONE with your cluster details
gcloud container clusters get-credentials CLUSTER_NAME --zone COMPUTE_ZONE --project YOUR_PROJECT_ID

# Verify connection
kubectl cluster-info
```

For local development with Orbstack, you can skip the GCP authentication steps and proceed with the local setup.

## Local Development Setup

### 1. Create Kubernetes Namespaces

```bash
# Create required namespaces
kubectl create namespace buildkite-webhook
kubectl create namespace monitoring
```

### 2. Configure Secrets and ConfigMaps

```bash
# Create webhook token secret
kubectl create secret generic buildkite-webhook-secrets \
  --namespace buildkite-webhook \
  --from-literal=buildkite-token="your-webhook-token"

# Create GCP credentials secret
kubectl create secret generic gcp-credentials \
  --namespace buildkite-webhook \
  --from-file=credentials.json

# Create ConfigMap for project configuration
kubectl create configmap buildkite-webhook-config \
  --namespace buildkite-webhook \
  --from-literal=project_id="your-project-id" \
  --from-literal=topic_id="buildkite-events"
```

### 3. Build and Deploy Application

```bash
# Start local Docker registry
docker run -d -p 5000:5000 --restart always --name registry registry:2

# Build and push image
docker build -t buildkite-webhook .
docker tag buildkite-webhook localhost:5000/buildkite-webhook:latest
docker push localhost:5000/buildkite-webhook:latest

# Deploy the application
kubectl apply -f k8s/buildkite-webhook/
```

### 4. Deploy Monitoring Stack

```bash
# Deploy Prometheus
kubectl apply -f k8s/monitoring/prometheus/

# Deploy Grafana
kubectl apply -f k8s/monitoring/grafana/

# Verify deployments
kubectl get pods -n monitoring
```

## Production Deployment

### 1. Configure TLS

```bash
# Generate TLS certificate (production should use proper CA)
./scripts/generate-certs

# Create TLS secret
kubectl create secret tls buildkite-webhook-tls \
  --namespace buildkite-webhook \
  --key path/to/tls.key \
  --cert path/to/tls.crt
```

### 2. Configure Ingress

```bash
# Update ingress.yaml with your domain
vim k8s/buildkite-webhook/ingress.yaml

# Apply ingress configuration
kubectl apply -f k8s/buildkite-webhook/ingress.yaml
```

### 3. Configure HPA (Horizontal Pod Autoscaling)

```bash
# Apply HPA configuration
kubectl apply -f k8s/buildkite-webhook/hpa.yaml
```

### 4. Verify Deployment

```bash
# Check deployment status
kubectl get pods -n buildkite-webhook

# Check service endpoints
kubectl get svc -n buildkite-webhook

# Check ingress status
kubectl get ingress -n buildkite-webhook
```

## Monitoring Setup

### 1. Access Grafana

```bash
# Port forward Grafana service
kubectl port-forward -n monitoring svc/grafana 3000:3000
```

Visit http://localhost:3000 (default credentials: admin/admin)

### 2. Access Prometheus

```bash
# Port forward Prometheus service
kubectl port-forward -n monitoring svc/prometheus 9090:9090
```

Visit http://localhost:9090

## Testing the Deployment

### 1. Local Testing

```bash
# Port forward the webhook service
kubectl port-forward -n buildkite-webhook svc/buildkite-webhook 8080:80

# Test the webhook endpoint
curl -X POST \
  -H "Content-Type: application/json" \
  -H "X-Buildkite-Token: your-token" \
  http://localhost:8080/webhook \
  -d '{"event":"ping"}'
```

### 2. Production Testing

```bash
# Test using your ingress domain
curl -X POST \
  -H "Content-Type: application/json" \
  -H "X-Buildkite-Token: your-token" \
  https://your-domain.com/webhook \
  -d '{"event":"ping"}'
```

### Health Checks

```bash
# Check readiness probe
kubectl exec -n buildkite-webhook <pod-name> -- curl localhost:8080/ready

# Check liveness probe
kubectl exec -n buildkite-webhook <pod-name> -- curl localhost:8080/health
```

## Cleanup

To remove all deployed resources:

```bash
# Use provided cleanup script
./cleanup.sh

# Or manually:
kubectl delete namespace buildkite-webhook
kubectl delete namespace monitoring
```
