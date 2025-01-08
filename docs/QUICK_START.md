# Quick Start Guide

This guide walks through setting up the Buildkite PubSub Webhook service for local development and testing.

## Prerequisites

Before starting, ensure you have:

1. **Local Development Tools**
   - [Docker](https://docs.docker.com/get-docker/)
   - [Go 1.20+](https://golang.org/dl/)
   - [kubectl](https://kubernetes.io/docs/tasks/tools/)
   - [Orbstack](https://orbstack.dev/)
   - [ngrok](https://ngrok.com/)

2. **Google Cloud Setup**
   - A Google Cloud Project with Pub/Sub enabled
   - Service account with Pub/Sub permissions
   - Service account key file (see [GCP Setup Guide](GCP_SETUP.md))

3. **Buildkite Access**
   - Organization admin access to create webhooks
   - A webhook token (you can generate this yourself)

## 1. Initial Setup

```bash
# Clone the repository
git clone <your-repo-url>
cd buildkite-pubsub

# Create Kubernetes namespaces
kubectl create namespace buildkite-webhook
kubectl create namespace monitoring

# Set environment variables
export PROJECT_ID="your-gcp-project"
export TOPIC_ID="buildkite-events"
export BUILDKITE_WEBHOOK_TOKEN="your-webhook-token"
```

## 2. Configure Google Cloud

1. Set up your service account by following the [GCP Setup Guide](GCP_SETUP.md)
2. Create the Pub/Sub topic:
```bash
gcloud pubsub topics create $TOPIC_ID
```
3. Ensure your credentials.json file is in your working directory

## 3. Create Kubernetes Secrets and Config

```bash
# Create webhook token secret
kubectl create secret generic buildkite-webhook-secrets \
  --namespace buildkite-webhook \
  --from-literal=buildkite-token="$BUILDKITE_WEBHOOK_TOKEN"

# Create GCP credentials secret
kubectl create secret generic gcp-credentials \
  --namespace buildkite-webhook \
  --from-file=credentials.json

# Create ConfigMap for project configuration
kubectl create configmap buildkite-webhook-config \
  --namespace buildkite-webhook \
  --from-literal=project_id="$PROJECT_ID" \
  --from-literal=topic_id="$TOPIC_ID"
```

## 4. Deploy Monitoring Stack

```bash
# Deploy Prometheus
kubectl apply -f k8s/monitoring/prometheus/configmap.yaml
kubectl apply -f k8s/monitoring/prometheus/alerts.yaml
kubectl apply -f k8s/monitoring/prometheus/deployment.yaml
kubectl apply -f k8s/monitoring/prometheus/service.yaml

# Deploy Grafana
kubectl apply -f k8s/monitoring/grafana/secret.yaml
kubectl apply -f k8s/monitoring/grafana/dashboardconfig.yaml
kubectl apply -f k8s/monitoring/grafana/dashboard.yaml
kubectl apply -f k8s/monitoring/grafana/datasourceconfig.yaml
kubectl apply -f k8s/monitoring/grafana/deployment.yaml
kubectl apply -f k8s/monitoring/grafana/service.yaml

# Verify monitoring deployments
kubectl get pods -n monitoring
```

## 5. Build and Deploy Webhook Service

```bash
# Start local Docker registry
docker run -d -p 5000:5000 --restart always --name registry registry:2

# Build and push image
docker build -t buildkite-webhook .
docker tag buildkite-webhook localhost:5000/buildkite-webhook:latest
docker push localhost:5000/buildkite-webhook:latest

# Deploy the webhook service
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: buildkite-webhook
  namespace: buildkite-webhook
spec:
  replicas: 2
  selector:
    matchLabels:
      app: buildkite-webhook
  template:
    metadata:
      labels:
        app: buildkite-webhook
    spec:
      containers:
        - name: webhook
          image: localhost:5000/buildkite-webhook:latest
          imagePullPolicy: Always
          ports:
            - containerPort: 8080
          env:
            - name: PROJECT_ID
              valueFrom:
                configMapKeyRef:
                  name: buildkite-webhook-config
                  key: project_id
            - name: TOPIC_ID
              valueFrom:
                configMapKeyRef:
                  name: buildkite-webhook-config
                  key: topic_id
            - name: BUILDKITE_WEBHOOK_TOKEN
              valueFrom:
                secretKeyRef:
                  name: buildkite-webhook-secrets
                  key: buildkite-token
            - name: GOOGLE_APPLICATION_CREDENTIALS
              value: /var/secrets/google/credentials.json
          volumeMounts:
            - name: google-cloud-key
              mountPath: /var/secrets/google
      volumes:
        - name: google-cloud-key
          secret:
            secretName: gcp-credentials
EOF

# Create the service
kubectl apply -f - <<EOF
apiVersion: v1
kind: Service
metadata:
  name: buildkite-webhook
  namespace: buildkite-webhook
  labels:
    app: buildkite-webhook
spec:
  ports:
    - port: 80
      targetPort: 8080
      protocol: TCP
      name: http
  selector:
    app: buildkite-webhook
EOF

# Verify deployment
kubectl get pods -n buildkite-webhook
```

## 6. Set Up External Access

```bash
# Port forward the webhook service locally
kubectl port-forward -n buildkite-webhook svc/buildkite-webhook 8081:80 &

# Start ngrok in a separate terminal
ngrok http 8081
```

Note the ngrok URL (e.g., `https://random-string.ngrok-free.app`) for use in Buildkite settings.

## 7. Configure Buildkite Webhook

1. Go to your Buildkite organization settings
2. Navigate to Webhooks
3. Create a new webhook:
   - URL: Your ngrok URL + `/webhook` (e.g., `https://random-string.ngrok-free.app/webhook`)
   - Events: Select 'All Events' or specific events
   - Token: Use the same token from `BUILDKITE_WEBHOOK_TOKEN`
   - SSL Verification: Enabled (ngrok provides SSL)

## 8. Test the Webhook

```bash
# Send a test ping
curl -X POST \
  -H "Content-Type: application/json" \
  -H "X-Buildkite-Token: $BUILDKITE_WEBHOOK_TOKEN" \
  "https://your-ngrok-url/webhook" \
  -d '{"event":"ping"}'

# Expected response:
# {"message":"Pong! Webhook received successfully"}

# View webhook service logs
kubectl logs -f -n buildkite-webhook -l app=buildkite-webhook
```

## 9. Set Up Monitoring (Optional)

```bash
# Access Grafana
kubectl port-forward -n monitoring svc/grafana 3001:3000 &
# Visit http://localhost:3001 (admin/admin)

# Access Prometheus
kubectl port-forward -n monitoring svc/prometheus 9091:9090 &
# Visit http://localhost:9091
```

## Cleanup

Use our cleanup script to remove all resources:

```bash
./cleanup.sh

# Or run with dry-run to preview changes
./cleanup.sh --dry-run
```

## Next Steps

1. Set up event subscribers (see [Usage Guide](USAGE.md))
2. Configure alerts for failed webhooks
3. Create custom dashboards for your metrics
4. Set up production deployment with proper ingress

## Troubleshooting

- **Pod not starting**: Check logs with `kubectl logs -n buildkite-webhook -l app=buildkite-webhook`
- **Connection refused**: Ensure port-forwards are running
- **Token invalid**: Verify token in secret matches Buildkite configuration
- **Permission denied**: Check GCP credentials and permissions
