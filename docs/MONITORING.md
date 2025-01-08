# Monitoring Guide

This guide covers monitoring, metrics, and alerting for the Buildkite PubSub Webhook service.

## Available Metrics

### Webhook Metrics

| Metric | Type | Description | Labels |
|--------|------|-------------|---------|
| `buildkite_webhook_requests_total` | Counter | Total number of webhook requests | `status`, `event_type` |
| `buildkite_webhook_request_duration_seconds` | Histogram | Request processing time | `event_type` |
| `buildkite_webhook_auth_failures_total` | Counter | Authentication failures | - |

### Pub/Sub Metrics

| Metric | Type | Description | Labels |
|--------|------|-------------|---------|
| `buildkite_pubsub_publish_requests_total` | Counter | Pub/Sub publish attempts | `status` |
| `buildkite_pubsub_publish_duration_seconds` | Histogram | Pub/Sub publish latency | - |

### System Metrics

| Metric | Type | Description | Labels |
|--------|------|-------------|---------|
| `buildkite_errors_total` | Counter | Error count by type | `type` |
| `buildkite_rate_limit_exceeded_total` | Counter | Rate limit triggers | `type` |

## Grafana Dashboards

### Overview Dashboard

Main service health dashboard showing:
- Request rates and latencies
- Error rates
- Authentication failures
- Pub/Sub publish success rate

```yaml
# k8s/monitoring/grafana/dashboards/overview.json
{
  "panels": [
    {
      "title": "Webhook Request Rate",
      "type": "timeseries",
      "datasource": "prometheus",
      "targets": [
        {
          "expr": "rate(buildkite_webhook_requests_total[5m])",
          "legendFormat": "{{status}}"
        }
      ]
    },
    {
      "title": "Request Latency (p95)",
      "type": "gauge",
      "datasource": "prometheus",
      "targets": [
        {
          "expr": "histogram_quantile(0.95, rate(buildkite_webhook_request_duration_seconds_bucket[5m]))"
        }
      ]
    }
  ]
}
```

### Operational Dashboard

Detailed operational metrics showing:
- Rate limiting
- Resource usage
- Pub/Sub performance
- Detailed error breakdown

```yaml
# k8s/monitoring/grafana/dashboards/operational.json
{
  "panels": [
    {
      "title": "Rate Limit Triggers",
      "type": "timeseries",
      "datasource": "prometheus",
      "targets": [
        {
          "expr": "rate(buildkite_rate_limit_exceeded_total[5m])",
          "legendFormat": "{{type}}"
        }
      ]
    }
  ]
}
```

## Alerting

### Critical Alerts

1. **High Error Rate**
```yaml
alert: HighErrorRate
expr: rate(buildkite_errors_total[5m]) > 0.1
for: 5m
labels:
  severity: critical
annotations:
  summary: High error rate in webhook service
```

2. **Authentication Failures**
```yaml
alert: HighAuthFailures
expr: rate(buildkite_webhook_auth_failures_total[5m]) > 0.05
for: 5m
labels:
  severity: warning
```

3. **Publish Failures**
```yaml
alert: PubSubPublishFailures
expr: rate(buildkite_pubsub_publish_requests_total{status="error"}[5m]) > 0.1
for: 5m
labels:
  severity: critical
```

### Warning Alerts

1. **High Latency**
```yaml
alert: HighLatency
expr: histogram_quantile(0.95, rate(buildkite_webhook_request_duration_seconds_bucket[5m])) > 1
for: 5m
labels:
  severity: warning
```

2. **Rate Limiting**
```yaml
alert: RateLimitingTriggered
expr: rate(buildkite_rate_limit_exceeded_total[5m]) > 0.1
for: 5m
labels:
  severity: warning
```

## Monitoring Best Practices

### Dashboard Organization

1. **Overview Dashboard**
   - High-level service health
   - Key performance indicators
   - Quick problem identification

2. **Operational Dashboard**
   - Detailed metrics
   - Debugging information
   - Resource usage

3. **SLO Dashboard**
   - Availability metrics
   - Latency metrics
   - Error budget tracking

### Alert Configuration

1. **Severity Levels**
   - Critical: Immediate action required
   - Warning: Investigation needed
   - Info: Awareness only

2. **Alert Grouping**
   - Group related alerts
   - Prevent alert storms
   - Reduce noise

3. **Runbooks**
   - Link alerts to runbooks
   - Include troubleshooting steps
   - Document escalation paths

## Health Checks

### Liveness Probe
Configured in Kubernetes to check basic service health:
```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 15
  periodSeconds: 20
```

### Readiness Probe
Verifies service is ready to handle requests:
```yaml
readinessProbe:
  httpGet:
    path: /ready
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10
```

## Debugging

### Metric Queries

1. **Error Investigation**
```promql
# Error rate by type
rate(buildkite_errors_total[5m])

# Error ratio
sum(rate(buildkite_errors_total[5m])) / sum(rate(buildkite_webhook_requests_total[5m]))
```

2. **Performance Analysis**
```promql
# 95th percentile latency
histogram_quantile(0.95, sum(rate(buildkite_webhook_request_duration_seconds_bucket[5m])) by (le))

# Request rate by status
sum(rate(buildkite_webhook_requests_total[5m])) by (status)
```

### Common Issues

1. **High Error Rates**
   - Check logs for error patterns
   - Verify Pub/Sub connectivity
   - Check rate limiting status

2. **High Latency**
   - Monitor resource usage
   - Check Pub/Sub performance
   - Verify network connectivity

3. **Authentication Issues**
   - Check token configuration
   - Verify secret mounting
   - Monitor auth failure patterns

## Resource Usage

### Memory Profiling
```bash
# Get heap profile
kubectl exec -n buildkite-webhook <pod-name> -- curl -sK http://localhost:8080/debug/pprof/heap > heap.prof

# Analyze with pprof
go tool pprof heap.prof
```

### CPU Profiling
```bash
# Get 30-second CPU profile
kubectl exec -n buildkite-webhook <pod-name> -- curl -sK http://localhost:8080/debug/pprof/profile > cpu.prof

# Analyze with pprof
go tool pprof cpu.prof
```
