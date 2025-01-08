# Monitoring Guide

This guide covers monitoring, metrics, and alerting for the Buildkite PubSub Webhook service.

## Setup Options

### Option 1: UI-Based Setup (Recommended for Development)

1. Deploy the monitoring stack:
```bash
# Deploy Prometheus
kubectl apply -f k8s/monitoring/prometheus/configmap.yaml
kubectl apply -f k8s/monitoring/prometheus/deployment.yaml
kubectl apply -f k8s/monitoring/prometheus/service.yaml

# Deploy Grafana
kubectl apply -f k8s/monitoring/grafana/secret.yaml
kubectl apply -f k8s/monitoring/grafana/deployment.yaml
kubectl apply -f k8s/monitoring/grafana/service.yaml
```

2. Access Grafana:
```bash
kubectl port-forward -n monitoring svc/grafana 3000:3000
```

3. Create the dashboard:
   - Log into Grafana (default credentials: admin/admin)
   - Go to Dashboards → New Dashboard
   - Add these panels:

     **Webhook Events by Type**
     ```promql
     sum(buildkite_webhook_request_duration_seconds_count) by (event_type)
     ```

     **Request Duration by Event Type**
     ```promql
     rate(buildkite_webhook_request_duration_seconds_sum[5m]) / rate(buildkite_webhook_request_duration_seconds_count[5m])
     ```

### Option 2: ConfigMap-Based Setup (For Production/GitOps)

⚠️ **Note:** This approach requires additional setup and careful ordering of resources. Consider using a Grafana Operator or similar tool for production environments.

If you need to use ConfigMaps for dashboard provisioning, please refer to the [Grafana Provisioning Documentation](https://grafana.com/docs/grafana/latest/administration/provisioning/) or consider using the [Grafana Operator](https://github.com/grafana-operator/grafana-operator).

## Available Metrics

| Metric | Type | Description | Labels |
|--------|------|-------------|---------|
| `buildkite_webhook_request_duration_seconds` | Histogram | Request processing time | `event_type` |
| `buildkite_webhook_requests_total` | Counter | Total number of webhook requests | `status`, `event_type` |
| `buildkite_webhook_auth_failures_total` | Counter | Authentication failures | - |
| `buildkite_pubsub_publish_requests_total` | Counter | Pub/Sub publish attempts | `status` |
| `buildkite_pubsub_publish_duration_seconds` | Histogram | Pub/Sub publish latency | - |

## Verifying Metrics

1. Check Prometheus metrics endpoint:
```bash
kubectl port-forward -n buildkite-webhook svc/buildkite-webhook 8080:80
curl http://localhost:8080/metrics
```

2. Check Prometheus is scraping:
```bash
kubectl port-forward -n monitoring svc/prometheus 9090:9090
```
Visit http://localhost:9090/targets

## Troubleshooting

1. **No metrics in Prometheus**
   - Check the webhook /metrics endpoint
   - Verify Prometheus scrape configs
   - Check Prometheus logs

2. **No data in Grafana**
   - Verify Prometheus data source configuration
   - Check queries directly in Prometheus
   - Ensure time range matches when data started flowing

## Alerting

Alert configurations remain in Prometheus AlertManager as before.
