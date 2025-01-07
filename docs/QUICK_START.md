# Quick Start Guide for Buildkite PubSub Webhook

This guide will walk you through setting up the Buildkite PubSub Webhook system from scratch, including monitoring and visualization.

## Prerequisites

- [Orbstack](https://orbstack.dev/) installed and running
- [kubectl](https://kubernetes.io/docs/tasks/tools/) configured to work with Orbstack
- [Go 1.20+](https://golang.org/dl/) installed
- A Buildkite account with webhook access
- Google Cloud Project access (for Pub/Sub)

## 1. Initial Setup

First, clone the repository and set up your environment:

```bash
git clone <your-repo-url>
cd buildkite-pubsub

# Set up environment variables
export PROJECT_ID="your-gcp-project"
export TOPIC_ID="buildkite-events"
export BUILDKITE_WEBHOOK_TOKEN="your-buildkite-webhook-token"
```

## 2. Create Kubernetes Namespace and Resources

```bash
# Create namespace
kubectl create namespace buildkite-webhook

# Create secrets (use base64 encoded values)
echo -n "$BUILDKITE_WEBHOOK_TOKEN" | base64 > token.b64
echo -n "$PROJECT_ID" | base64 > project.b64

kubectl create secret generic buildkite-webhook-secrets \
  --namespace buildkite-webhook \
  --from-file=buildkite-token=token.b64 \
  --from-file=project-id=project.b64

# Clean up temporary files
rm token.b64 project.b64

# Generate TLS certificates
./generate-certs
```

## 3. Set Up Monitoring Stack

```bash
# Create monitoring namespace
kubectl create namespace monitoring

# Apply Prometheus configurations
kubectl apply -f k8s/monitoring/prometheus-configmap.yaml
kubectl apply -f k8s/monitoring/prometheus-deployment.yaml
kubectl apply -f k8s/monitoring/prometheus-service.yaml

# Apply Grafana configurations
kubectl apply -f k8s/monitoring/grafana-configmap.yaml
kubectl apply -f k8s/monitoring/grafana-deployment.yaml
kubectl apply -f k8s/monitoring/grafana-service.yaml

# Set up alerts
kubectl apply -f k8s/monitoring/alerts.yaml
```

## 4. Deploy the Webhook Service

```bash
# Apply core configurations
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/service.yaml
kubectl apply -f k8s/ingress.yaml
kubectl apply -f k8s/hpa.yaml

# Verify deployments
kubectl get pods -n buildkite-webhook
kubectl get pods -n monitoring
```

## 5. Configure Local Development

For local development and testing:

```bash
# Run the service locally
go run cmd/webhook/main.go

# Or with hot reload using Air
go install github.com/cosmtrek/air@latest
air
```

## 6. Set Up Buildkite Webhook

1. Go to your Buildkite organization settings
2. Navigate to Webhooks
3. Create a new webhook:
   - URL: `https://webhook.your-domain.com/webhook` (or your local URL)
   - Events: Select the events you want to receive
   - Token: Use the same token you set in `BUILDKITE_WEBHOOK_TOKEN`

## 7. Verify the Setup

Check the following endpoints:

- Health check: `https://webhook.your-domain.com/health`
- Readiness: `https://webhook.your-domain.com/ready`
- Prometheus metrics: `http://localhost:9090`
- Grafana dashboards: `http://localhost:3000`

### Testing the Webhook

```bash
# Send a test webhook
curl -X POST \
  -H "Content-Type: application/json" \
  -H "X-Buildkite-Token: your-token" \
  https://webhook.your-domain.com/webhook \
  -d '{
    "event": "build.started",
    "build": {
      "id": "test-build",
      "url": "https://buildkite.com/test",
      "number": 1,
      "state": "started"
    }
  }'
```

## 8. Monitor Events

1. Access Grafana (default credentials: admin/admin):
   ```bash
   kubectl port-forward -n monitoring svc/grafana 3000:3000
   ```

2. Add Prometheus as a data source:
   - URL: `http://prometheus:9090`
   - Access: Server (default)

3. Import the provided dashboards from `k8s/monitoring/dashboards/`

## 9. Troubleshooting

Common issues and solutions:

1. Check pod status:
   ```bash
   kubectl get pods -n buildkite-webhook
   kubectl describe pod <pod-name> -n buildkite-webhook
   ```

2. View logs:
   ```bash
   kubectl logs -f deployment/buildkite-webhook -n buildkite-webhook
   ```

3. Check Prometheus targets:
   ```bash
   kubectl port-forward -n monitoring svc/prometheus 9090:9090
   # Visit http://localhost:9090/targets
   ```

## 10. Clean Up

To remove everything:

```bash
kubectl delete namespace buildkite-webhook
kubectl delete namespace monitoring
```

## Next Steps

- Set up production-grade TLS certificates
- Configure persistent storage for Prometheus and Grafana
- Set up alerts to your preferred notification channels
- Add custom dashboards for your specific metrics

For more detailed information, check the [Usage Guide](USAGE.md) and individual component documentation.
