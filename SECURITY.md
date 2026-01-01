# Security Policy

## Supported Versions

We provide security updates for the following versions:

| Version | Supported          |
| ------- | ------------------ |
| 1.x.x   | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

We take security seriously. If you discover a security vulnerability, please report it responsibly.

### How to Report

**Please do NOT report security vulnerabilities through public GitHub issues.**

Instead, please report them via one of the following methods:

1. **Email**: Send details to the repository maintainer
2. **GitHub Security Advisory**: Use the "Report a vulnerability" button in the Security tab

### What to Include

When reporting a vulnerability, please include:

- Description of the vulnerability
- Steps to reproduce the issue
- Potential impact assessment
- Any suggested fixes (if available)

### Response Timeline

- **Initial Response**: Within 48 hours
- **Status Update**: Within 7 days
- **Resolution**: Depends on severity and complexity

### Disclosure Policy

- We will acknowledge receipt of your report
- We will investigate and validate the issue
- We will work on a fix and coordinate disclosure
- We will credit reporters in security advisories (unless anonymity is requested)

## Security Best Practices

When deploying this service, follow these security recommendations:

### Authentication

- Always use either a strong webhook token or HMAC-SHA256 signature verification
- Rotate tokens periodically
- Store tokens in secure secret management systems (e.g., Kubernetes Secrets, HashiCorp Vault)

### Network Security

- Deploy behind a load balancer with TLS termination
- Use network policies to restrict traffic
- Consider IP allowlisting for Buildkite webhook IPs

### GCP Security

- Use dedicated service accounts with minimal permissions
- Enable audit logging for Pub/Sub operations
- Use VPC Service Controls if available

### Rate Limiting

- Configure appropriate rate limits to prevent abuse
- Monitor rate limit metrics for anomalies

### Monitoring

- Enable alerting for authentication failures
- Monitor for unusual traffic patterns
- Set up alerts for error rate spikes

## Security Features

This service includes several security features:

- **Token Authentication**: Constant-time comparison to prevent timing attacks
- **HMAC-SHA256 Signatures**: Request payload verification with replay protection
- **Rate Limiting**: Per-IP and global rate limiting
- **Request Validation**: Input validation and size limits
- **Structured Logging**: Security event logging for audit trails

## Dependency Security

We use the following tools to maintain dependency security:

- **govulncheck**: Checks for known vulnerabilities in Go dependencies
- **gosec**: Static analysis for security issues in Go code
- **Renovate/Dependabot**: Automated dependency updates

To check for vulnerabilities locally:
```bash
govulncheck ./...
gosec ./...
```

## Contact

For any security-related questions that don't involve vulnerability reports, please open a GitHub issue.
