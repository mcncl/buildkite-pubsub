# Testing Guide

Complete guide for testing the Buildkite PubSub webhook service from local development through production deployment.

## Table of Contents

1. [Quick Commands](#quick-commands) - Copy-paste commands for common scenarios
2. [Unit Tests](#unit-tests)
3. [Local Development Testing](#local-development-testing)
4. [Local Kubernetes Testing](#local-kubernetes-testing)
5. [GCP Deployment Testing](#gcp-deployment-testing)
6. [Retry Logic Testing](#retry-logic-testing)
7. [Load Testing](#load-testing)
8. [Troubleshooting](#troubleshooting)

---

## Quick Commands

**Already know what you're doing?** Here are the essential commands:

```bash
# Run tests
go test ./... -cover

# Local dev (with Pub/Sub emulator)
docker run -d --name pubsub-emulator -p 8085:8085 \
  gcr.io/google.com/cloudsdktool/google-cloud-cli:emulators \
  gcloud beta emulators pubsub start --host-port=0.0.0.0:8085
export PUBSUB_EMULATOR_HOST=localhost:8085 PROJECT_ID=test-project \
  TOPIC_ID=buildkite-events BUILDKITE_WEBHOOK_TOKEN=test-token PORT=8888
./scripts/init-emulator.sh
go run cmd/webhook/main.go --log-level=debug

# Test webhook
curl -X POST http://localhost:8888/webhook \
  -H "X-Buildkite-Token: test-token" \
  -H "Content-Type: application/json" \
  -d '{"event":"ping","organization":{"slug":"test"}}'

# Kubernetes (local)
docker build -t buildkite-webhook:local . && kubectl create ns buildkite-webhook
kubectl apply -f k8s/ && kubectl port-forward svc/buildkite-webhook 8080:80 -n buildkite-webhook

# GCP deployment
docker build -t gcr.io/$PROJECT_ID/buildkite-webhook:latest . && docker push gcr.io/$PROJECT_ID/buildkite-webhook:latest
kubectl apply -f k8s/ && kubectl get svc buildkite-webhook -n buildkite-webhook

# Monitoring
kubectl logs -n buildkite-webhook -l app=buildkite-webhook --tail=50 -f
curl http://localhost:8888/metrics
```

**Need more context?** Read the detailed sections below.

---

## Unit Tests

Run the test suite to verify everything works:

```bash
# Run all tests
go test ./...

# Run with coverage
go test ./... -cover

# Run specific package
go test ./pkg/webhook -v

# Run retry tests specifically
go test ./pkg/webhook -run TestPublishWithRetry -v
```

**Expected Output:**
```
ok  	github.com/mcncl/buildkite-pubsub/pkg/webhook	26.558s	coverage: 87.8% of statements
```

---

## Local Development Testing

### 1. Setup Local Environment

#### Prerequisites
- Go 1.20+
- Docker Desktop or OrbStack
- ngrok (for webhook testing)

#### Start Pub/Sub Emulator

```bash
# Using Docker
docker run -d --name pubsub-emulator \
  -p 8085:8085 \
  gcr.io/google.com/cloudsdktool/google-cloud-cli:emulators \
  gcloud beta emulators pubsub start --host-port=0.0.0.0:8085

# Verify it's running
curl http://localhost:8085/
```

#### Configure Environment

Create `.env` file:

```bash
cat > .env << 'EOF'
# Pub/Sub Emulator
PUBSUB_EMULATOR_HOST=localhost:8085
PROJECT_ID=test-project
TOPIC_ID=buildkite-events

# Webhook Auth
BUILDKITE_WEBHOOK_TOKEN=test-token-local-dev

# Server
PORT=8888
LOG_LEVEL=debug
LOG_FORMAT=dev

# Tracing (optional)
ENABLE_TRACING=false
EOF
```

### 2. Initialize Pub/Sub Topic

Create a script to initialize the emulator:

```bash
cat > scripts/init-emulator.sh << 'EOF'
#!/bin/bash
set -e

export PUBSUB_EMULATOR_HOST=localhost:8085
PROJECT_ID=${PROJECT_ID:-test-project}
TOPIC_ID=${TOPIC_ID:-buildkite-events}

echo "Creating topic: $TOPIC_ID"
curl -X PUT "http://localhost:8085/v1/projects/$PROJECT_ID/topics/$TOPIC_ID"

echo ""
echo "Creating test subscription"
curl -X PUT "http://localhost:8085/v1/projects/$PROJECT_ID/subscriptions/test-sub" \
  -H "Content-Type: application/json" \
  -d "{
    \"topic\": \"projects/$PROJECT_ID/topics/$TOPIC_ID\"
  }"

echo ""
echo "✅ Emulator initialized!"
EOF

chmod +x scripts/init-emulator.sh
./scripts/init-emulator.sh
```

### 3. Run the Service Locally

```bash
# Load environment variables
export $(grep -v '^#' .env | xargs)

# Run the service
go run cmd/webhook/main.go --log-level=debug --log-format=dev
```

**Expected Output:**
```json
{
  "level": "info",
  "msg": "Configuration loaded",
  "config": {
    "gcp": {
      "project_id": "test-project",
      "topic_id": "buildkite-events"
    },
    "webhook": {
      "token": "********",
      "path": "/webhook"
    },
    "server": {
      "port": 8888
    }
  }
}
{
  "level": "info",
  "msg": "Server starting",
  "port": 8888
}
```

### 4. Test the Service

#### Test Health Endpoints

```bash
# Health check
curl http://localhost:8888/health
# {"status":"healthy"}

# Readiness check
curl http://localhost:8888/ready
# {"status":"ready"}

# Metrics
curl http://localhost:8888/metrics | grep buildkite
```

#### Send Test Webhook

```bash
# Simple ping event
curl -X POST http://localhost:8888/webhook \
  -H "X-Buildkite-Token: test-token-local-dev" \
  -H "Content-Type: application/json" \
  -d '{
    "event": "ping",
    "organization": {
      "slug": "test-org"
    }
  }'

# Expected: {"status":"success","message":"Pong! Webhook received successfully"}
```

#### Send Build Event

```bash
curl -X POST http://localhost:8888/webhook \
  -H "X-Buildkite-Token: test-token-local-dev" \
  -H "Content-Type: application/json" \
  -d '{
    "event": "build.started",
    "build": {
      "id": "test-build-123",
      "number": 42,
      "state": "running",
      "branch": "main",
      "commit": "abc123",
      "created_at": "2024-01-09T10:00:00Z",
      "started_at": "2024-01-09T10:01:00Z"
    },
    "pipeline": {
      "slug": "test-pipeline",
      "name": "Test Pipeline"
    },
    "organization": {
      "slug": "test-org"
    }
  }'

# Expected: {"status":"success","message":"Event published successfully","message_id":"1","event_type":"build.started"}
```

#### Verify Message in Pub/Sub

```bash
# Pull messages from subscription
curl -X POST "http://localhost:8085/v1/projects/test-project/subscriptions/test-sub:pull" \
  -H "Content-Type: application/json" \
  -d '{"maxMessages": 10}' | jq
```

### 5. Test with ngrok (Real Buildkite Webhooks)

```bash
# Start ngrok
ngrok http 8888

# Copy the HTTPS URL (e.g., https://abc123.ngrok.io)
# Go to Buildkite → Settings → Webhooks → Add Webhook
# URL: https://abc123.ngrok.io/webhook
# Token: test-token-local-dev
# Events: All Events

# Trigger a build in Buildkite and watch the logs!
```

---

## Local Kubernetes Testing

Test the full deployment stack locally with OrbStack or Docker Desktop.

### 1. Setup Local Kubernetes

```bash
# OrbStack (recommended)
orbstack

# Or Docker Desktop - enable Kubernetes in settings
```

### 2. Build Local Image

```bash
# Build the image
docker build -t buildkite-webhook:local .

# Verify
docker images | grep buildkite-webhook
```

### 3. Create Namespace

```bash
kubectl create namespace buildkite-webhook

# Set as default
kubectl config set-context --current --namespace=buildkite-webhook
```

### 4. Create Secrets

```bash
# Create secret for webhook token
kubectl create secret generic buildkite-webhook-token \
  --from-literal=token=test-token-local-k8s

# Create secret for Pub/Sub emulator
kubectl create secret generic pubsub-credentials \
  --from-literal=project-id=test-project
```

### 5. Deploy Pub/Sub Emulator

```bash
cat > k8s-local/pubsub-emulator.yaml << 'EOF'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: pubsub-emulator
  namespace: buildkite-webhook
spec:
  replicas: 1
  selector:
    matchLabels:
      app: pubsub-emulator
  template:
    metadata:
      labels:
        app: pubsub-emulator
    spec:
      containers:
      - name: emulator
        image: gcr.io/google.com/cloudsdktool/google-cloud-cli:emulators
        command:
        - gcloud
        - beta
        - emulators
        - pubsub
        - start
        - --host-port=0.0.0.0:8085
        ports:
        - containerPort: 8085
---
apiVersion: v1
kind: Service
metadata:
  name: pubsub-emulator
  namespace: buildkite-webhook
spec:
  selector:
    app: pubsub-emulator
  ports:
  - port: 8085
    targetPort: 8085
EOF

kubectl apply -f k8s-local/pubsub-emulator.yaml
```

### 6. Deploy Webhook Service

```bash
cat > k8s-local/webhook-local.yaml << 'EOF'
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
        image: buildkite-webhook:local
        imagePullPolicy: Never  # Use local image
        ports:
        - containerPort: 8080
        env:
        - name: PUBSUB_EMULATOR_HOST
          value: "pubsub-emulator:8085"
        - name: PROJECT_ID
          valueFrom:
            secretKeyRef:
              name: pubsub-credentials
              key: project-id
        - name: TOPIC_ID
          value: "buildkite-events"
        - name: BUILDKITE_WEBHOOK_TOKEN
          valueFrom:
            secretKeyRef:
              name: buildkite-webhook-token
              key: token
        - name: LOG_LEVEL
          value: "debug"
        - name: LOG_FORMAT
          value: "json"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 3
          periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: buildkite-webhook
  namespace: buildkite-webhook
spec:
  type: LoadBalancer
  selector:
    app: buildkite-webhook
  ports:
  - port: 80
    targetPort: 8080
EOF

kubectl apply -f k8s-local/webhook-local.yaml
```

### 7. Initialize Topic in Emulator

```bash
# Port-forward to emulator
kubectl port-forward svc/pubsub-emulator 8085:8085 &

# Initialize topic
./scripts/init-emulator.sh

# Stop port-forward
pkill -f "kubectl port-forward"
```

### 8. Test the Deployment

```bash
# Check pod status
kubectl get pods

# Check logs
kubectl logs -l app=buildkite-webhook --tail=50 -f

# Port-forward to service
kubectl port-forward svc/buildkite-webhook 8080:80

# Test in another terminal
curl -X POST http://localhost:8080/webhook \
  -H "X-Buildkite-Token: test-token-local-k8s" \
  -H "Content-Type: application/json" \
  -d '{
    "event": "build.started",
    "build": {"id": "k8s-test", "number": 1, "state": "running"},
    "pipeline": {"slug": "test", "name": "Test"},
    "organization": {"slug": "test"}
  }'
```

### 9. Test Load Balancing

```bash
# Send multiple requests
for i in {1..10}; do
  curl -X POST http://localhost:8080/webhook \
    -H "X-Buildkite-Token: test-token-local-k8s" \
    -H "Content-Type: application/json" \
    -d "{\"event\":\"build.started\",\"build\":{\"id\":\"test-$i\"}}"
  echo ""
done

# Check logs to see load distribution
kubectl logs -l app=buildkite-webhook --tail=20
```

---

## GCP Deployment Testing

Deploy to Google Cloud Platform for production-like testing.

### 1. GCP Setup

Follow the [GCP Setup Guide](GCP_SETUP.md) first, then:

```bash
# Set your project
export PROJECT_ID=your-gcp-project-id
export REGION=us-central1
gcloud config set project $PROJECT_ID
```

### 2. Build and Push Image

```bash
# Configure Docker for GCR
gcloud auth configure-docker

# Build with proper tagging
docker build -t gcr.io/$PROJECT_ID/buildkite-webhook:v1.0.0 .
docker push gcr.io/$PROJECT_ID/buildkite-webhook:v1.0.0

# Tag as latest
docker tag gcr.io/$PROJECT_ID/buildkite-webhook:v1.0.0 \
  gcr.io/$PROJECT_ID/buildkite-webhook:latest
docker push gcr.io/$PROJECT_ID/buildkite-webhook:latest
```

### 3. Create GKE Cluster (if not exists)

```bash
gcloud container clusters create buildkite-webhook-cluster \
  --region=$REGION \
  --num-nodes=2 \
  --machine-type=e2-small \
  --enable-autoscaling \
  --min-nodes=2 \
  --max-nodes=5 \
  --enable-autorepair \
  --enable-autoupgrade

# Get credentials
gcloud container clusters get-credentials buildkite-webhook-cluster --region=$REGION
```

### 4. Create Kubernetes Secrets

```bash
# Create namespace
kubectl create namespace buildkite-webhook

# Create webhook token secret
kubectl create secret generic buildkite-webhook-token \
  --namespace=buildkite-webhook \
  --from-literal=token=$(openssl rand -hex 32)

# Store the token for later use
kubectl get secret buildkite-webhook-token \
  --namespace=buildkite-webhook \
  -o jsonpath='{.data.token}' | base64 -d > webhook-token.txt
echo ""
echo "Webhook token saved to webhook-token.txt"

# Create service account key (already done in GCP_SETUP.md)
kubectl create secret generic gcp-credentials \
  --namespace=buildkite-webhook \
  --from-file=credentials.json=./credentials.json
```

### 5. Update Kubernetes Manifests

```bash
# Update deployment with your image
sed -i '' "s|IMAGE_PLACEHOLDER|gcr.io/$PROJECT_ID/buildkite-webhook:v1.0.0|" k8s/deployment.yaml

# Update configmap with your project
sed -i '' "s|PROJECT_ID_PLACEHOLDER|$PROJECT_ID|" k8s/configmap.yaml
```

### 6. Deploy to GKE

```bash
# Apply all manifests
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/secret.yaml  # If not using existing secrets
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/service.yaml
kubectl apply -f k8s/hpa.yaml
kubectl apply -f k8s/ingress.yaml

# Check rollout status
kubectl rollout status deployment/buildkite-webhook -n buildkite-webhook

# Check pods
kubectl get pods -n buildkite-webhook
```

### 7. Get External IP

```bash
# Get service external IP
kubectl get service buildkite-webhook -n buildkite-webhook

# Wait for EXTERNAL-IP (may take a few minutes)
# Or get ingress IP
kubectl get ingress -n buildkite-webhook
```

### 8. Test the Deployment

```bash
# Get the external IP
WEBHOOK_URL=$(kubectl get service buildkite-webhook -n buildkite-webhook -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
WEBHOOK_TOKEN=$(cat webhook-token.txt)

# Test health endpoint
curl http://$WEBHOOK_URL/health

# Test webhook
curl -X POST http://$WEBHOOK_URL/webhook \
  -H "X-Buildkite-Token: $WEBHOOK_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "event": "build.started",
    "build": {
      "id": "gcp-test-build",
      "number": 1,
      "state": "running",
      "branch": "main",
      "created_at": "2024-01-09T10:00:00Z",
      "started_at": "2024-01-09T10:01:00Z"
    },
    "pipeline": {"slug": "test", "name": "Test Pipeline"},
    "organization": {"slug": "test-org"}
  }'
```

### 9. Verify in Google Cloud Console

```bash
# View Pub/Sub messages
gcloud pubsub subscriptions pull buildkite-events-all \
  --limit=5 \
  --format=json | jq

# View pod logs
kubectl logs -n buildkite-webhook -l app=buildkite-webhook --tail=50

# View metrics in GCP Console
# Go to: Kubernetes Engine → Workloads → buildkite-webhook → Logs
```

### 10. Configure Buildkite

```bash
echo "Configure your Buildkite webhook with:"
echo "URL: http://$WEBHOOK_URL/webhook"
echo "Token: $WEBHOOK_TOKEN"
```

Go to Buildkite → Organization Settings → Webhooks → Add Webhook:
- **URL**: `http://YOUR_EXTERNAL_IP/webhook`
- **Token**: Copy from `webhook-token.txt`
- **Events**: All Events (or select specific ones)

---

## Retry Logic Testing

Test the retry functionality with various failure scenarios.

### 1. Test Retry on Transient Failure

Create a test that simulates Pub/Sub being temporarily unavailable:

```bash
# Create a test script
cat > test-retry.sh << 'EOF'
#!/bin/bash
set -e

WEBHOOK_URL=${WEBHOOK_URL:-http://localhost:8888}
TOKEN=${TOKEN:-test-token-local-dev}

echo "Testing retry logic..."

# Stop Pub/Sub emulator to simulate failure
echo "1. Stopping Pub/Sub emulator..."
docker stop pubsub-emulator

# Send webhook (should fail and retry)
echo "2. Sending webhook (will retry)..."
curl -X POST $WEBHOOK_URL/webhook \
  -H "X-Buildkite-Token: $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "event": "build.started",
    "build": {"id": "retry-test", "number": 1, "state": "running"},
    "pipeline": {"slug": "test", "name": "Test"},
    "organization": {"slug": "test"}
  }' &

CURL_PID=$!

# Wait 2 seconds then restart emulator
sleep 2
echo "3. Restarting Pub/Sub emulator..."
docker start pubsub-emulator
sleep 2

# Wait for curl to finish
wait $CURL_PID

echo "4. ✅ Test complete!"
echo ""
echo "Check logs for retry attempts:"
echo "  grep 'retry' logs.txt"
EOF

chmod +x test-retry.sh
./test-retry.sh
```

### 2. Monitor Retry Metrics

```bash
# Query Prometheus metrics
curl http://localhost:8888/metrics | grep buildkite_pubsub_retries_total

# Expected output:
# buildkite_pubsub_retries_total{event_type="build.started"} 2
```

### 3. Test Non-Retryable Error (Auth Failure)

```bash
# Send with wrong token (should fail immediately, no retries)
curl -X POST http://localhost:8888/webhook \
  -H "X-Buildkite-Token: wrong-token" \
  -H "Content-Type: application/json" \
  -d '{"event":"ping"}' \
  -v

# Expected: 401 Unauthorized with no retries
```

### 4. Test Context Cancellation

```bash
# Send with timeout (context will cancel during retry)
curl -X POST http://localhost:8888/webhook \
  -H "X-Buildkite-Token: test-token-local-dev" \
  -H "Content-Type: application/json" \
  -d '{"event":"build.started","build":{"id":"timeout-test"}}' \
  --max-time 1

# Should fail with timeout, check logs for context cancellation
```

### 5. Load Test with Retries

Create a load test that triggers retries:

```bash
cat > load-test-retries.sh << 'EOF'
#!/bin/bash
WEBHOOK_URL=${WEBHOOK_URL:-http://localhost:8888}
TOKEN=${TOKEN:-test-token-local-dev}
REQUESTS=${REQUESTS:-100}

echo "Load testing with $REQUESTS requests..."

# Function to send webhook
send_webhook() {
  local id=$1
  curl -s -X POST $WEBHOOK_URL/webhook \
    -H "X-Buildkite-Token: $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"event\":\"build.started\",\"build\":{\"id\":\"load-$id\"}}" \
    > /dev/null
}

# Send requests in parallel
for i in $(seq 1 $REQUESTS); do
  send_webhook $i &
  
  # Limit concurrent requests
  if (( i % 10 == 0 )); then
    wait
  fi
done

wait

echo "✅ Load test complete!"
echo ""
echo "Check metrics:"
curl -s http://localhost:8888/metrics | grep -E "(buildkite_webhook_requests_total|buildkite_pubsub_retries_total)"
EOF

chmod +x load-test-retries.sh
./load-test-retries.sh
```

### 6. Verify Exponential Backoff

Enable debug logging and check backoff timing:

```bash
# Run with debug logging
LOG_LEVEL=debug go run cmd/webhook/main.go 2>&1 | tee logs.txt

# In another terminal, cause a failure
docker stop pubsub-emulator
curl -X POST http://localhost:8888/webhook \
  -H "X-Buildkite-Token: test-token-local-dev" \
  -H "Content-Type: application/json" \
  -d '{"event":"build.started","build":{"id":"backoff-test"}}'

# Check logs for backoff timing
grep -A 5 "retry" logs.txt

# Expected pattern: ~100ms, ~500ms, ~1s delays between attempts
```

---

## Load Testing

### Basic Load Test with Apache Bench

```bash
# Install Apache Bench (if not installed)
# macOS: brew install apache-bench
# Ubuntu: apt-get install apache2-utils

# Create test payload
cat > payload.json << 'EOF'
{
  "event": "build.started",
  "build": {
    "id": "load-test",
    "number": 1,
    "state": "running"
  },
  "pipeline": {"slug": "test", "name": "Test"},
  "organization": {"slug": "test"}
}
EOF

# Run load test (100 requests, 10 concurrent)
ab -n 100 -c 10 \
  -H "X-Buildkite-Token: test-token-local-dev" \
  -H "Content-Type: application/json" \
  -p payload.json \
  http://localhost:8888/webhook

# Check results and metrics
curl http://localhost:8888/metrics | grep buildkite_webhook_requests_total
```

### Advanced Load Test with k6

```bash
# Install k6
# macOS: brew install k6
# Ubuntu: snap install k6

# Create k6 test script
cat > load-test.js << 'EOF'
import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  vus: 10,  // Virtual users
  duration: '30s',
  thresholds: {
    http_req_duration: ['p(95)<500'],  // 95% of requests should be below 500ms
    http_req_failed: ['rate<0.01'],     // Less than 1% failure rate
  },
};

export default function () {
  const url = __ENV.WEBHOOK_URL || 'http://localhost:8888/webhook';
  const payload = JSON.stringify({
    event: 'build.started',
    build: {
      id: `load-test-${__VU}-${__ITER}`,
      number: __ITER,
      state: 'running',
    },
    pipeline: { slug: 'test', name: 'Test Pipeline' },
    organization: { slug: 'test' },
  });

  const params = {
    headers: {
      'Content-Type': 'application/json',
      'X-Buildkite-Token': __ENV.WEBHOOK_TOKEN || 'test-token-local-dev',
    },
  };

  const res = http.post(url, payload, params);
  
  check(res, {
    'status is 200': (r) => r.status === 200,
    'response time < 500ms': (r) => r.timings.duration < 500,
  });

  sleep(0.1);  // 100ms between requests per VU
}
EOF

# Run k6 load test
k6 run load-test.js

# Run with custom parameters
k6 run load-test.js \
  --env WEBHOOK_URL=http://your-service-ip/webhook \
  --env WEBHOOK_TOKEN=your-token
```

---

## Troubleshooting

### Service won't start

```bash
# Check pod status
kubectl get pods -n buildkite-webhook

# Check pod logs
kubectl logs -n buildkite-webhook -l app=buildkite-webhook

# Verify configuration:
# 1. Confirm Pub/Sub topic exists
gcloud pubsub topics describe buildkite-events --project=$PROJECT_ID

# 2. Verify credentials are present
kubectl get secrets -n buildkite-webhook

# 3. Review configuration
kubectl get configmap buildkite-webhook-config -n buildkite-webhook -o yaml
```

### Webhooks not being published

```bash
# Check if topic exists
gcloud pubsub topics list --project=$PROJECT_ID

# Check service logs for errors
kubectl logs -n buildkite-webhook -l app=buildkite-webhook --tail=100 | grep -i error

# Test with direct curl
curl -X POST http://localhost:8888/webhook \
  -H "X-Buildkite-Token: $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"event":"ping"}' \
  -v
```

### Verifying retry behavior

```bash
# Check retry metrics
curl http://localhost:8888/metrics | grep retry

# Enable debug logging for detailed retry information
kubectl set env deployment/buildkite-webhook LOG_LEVEL=debug -n buildkite-webhook

# View retry logs
kubectl logs -n buildkite-webhook -l app=buildkite-webhook | grep -i retry
```

### Investigating high latency

```bash
# Check Pub/Sub publish latency
curl http://localhost:8888/metrics | grep pubsub_publish_duration

# Check request duration
curl http://localhost:8888/metrics | grep webhook_request_duration

# Check if rate limited
curl http://localhost:8888/metrics | grep rate_limit
```

### Debug Commands

```bash
# Port-forward for direct access
kubectl port-forward svc/buildkite-webhook 8080:80 -n buildkite-webhook

# Execute shell in pod
kubectl exec -it deployment/buildkite-webhook -n buildkite-webhook -- /bin/sh

# Check resource usage
kubectl top pods -n buildkite-webhook

# Check events
kubectl get events -n buildkite-webhook --sort-by='.lastTimestamp'

# Restart deployment
kubectl rollout restart deployment/buildkite-webhook -n buildkite-webhook
```

---

## Test Checklist

Use this checklist to verify your deployment:

### Local Testing
- [ ] Unit tests pass (`go test ./...`)
- [ ] Retry tests pass specifically
- [ ] Service starts without errors
- [ ] Health endpoints respond
- [ ] Metrics endpoint returns data
- [ ] Ping event succeeds
- [ ] Build event publishes to Pub/Sub
- [ ] Messages appear in subscription

### Kubernetes Testing (Local)
- [ ] Pods are running
- [ ] Health checks pass
- [ ] Service accessible via LoadBalancer
- [ ] Multiple pods load balance requests
- [ ] Logs show successful publishes
- [ ] Pub/Sub emulator receives messages

### GCP Testing
- [ ] GKE cluster created
- [ ] Image built and pushed to GCR
- [ ] Secrets created successfully
- [ ] Deployment rolled out successfully
- [ ] External IP assigned
- [ ] Webhook responds to requests
- [ ] Messages publish to real Pub/Sub
- [ ] Buildkite webhooks work end-to-end

### Retry Testing
- [ ] Transient failures retry successfully
- [ ] Non-retryable errors fail immediately
- [ ] Exponential backoff observed in logs
- [ ] Retry metrics increment correctly
- [ ] Context cancellation stops retries
- [ ] Load test with retries succeeds

---

## Next Steps

After successful testing:

1. **Production Deployment**
   - Follow [K8S_DEPLOYMENT.md](K8S_DEPLOYMENT.md) for production setup
   - Configure TLS/Ingress
   - Set up monitoring and alerting

2. **Monitoring**
   - Deploy Prometheus and Grafana
   - Set up dashboards from [MONITORING.md](MONITORING.md)
   - Configure alerts for failures

3. **Advanced Features**
   - Implement Dead Letter Queue (TASK-005)
   - Add Circuit Breaker (TASK-006)
   - Enable distributed tracing

---

For more information:
- [Quick Start Guide](QUICK_START.md)
- [GCP Setup Guide](GCP_SETUP.md)
- [Monitoring Guide](MONITORING.md)
- [Architecture Documentation](ARCHITECTURE.md)
