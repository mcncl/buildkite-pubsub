# Contributing to Buildkite PubSub

Thank you for your interest in contributing to Buildkite PubSub! This document provides guidelines and information for contributors.

## Code of Conduct

Please be respectful and constructive in all interactions. We're building software together and aim to maintain a welcoming environment for all contributors.

## Getting Started

### Prerequisites

- Go 1.20+
- Docker (for running tests and local development)
- Access to a Google Cloud project (for integration testing)

### Development Setup

1. Clone the repository:
   ```bash
   git clone https://github.com/mcncl/buildkite-pubsub.git
   cd buildkite-pubsub
   ```

2. Install dependencies:
   ```bash
   go mod download
   ```

3. Run tests:
   ```bash
   go test ./...
   ```

4. Run linting:
   ```bash
   golangci-lint run
   ```

## Making Changes

### Branch Naming

Use descriptive branch names:
- `feature/add-dlq-support`
- `fix/retry-logic-timeout`
- `docs/update-readme`

### Code Style

- Follow standard Go conventions (use `gofmt` and `goimports`)
- Use meaningful variable and function names
- Add comments for exported functions and complex logic
- Keep functions focused and reasonably sized

### Testing

- Write tests for new functionality
- Ensure existing tests pass before submitting
- Aim for good test coverage on critical paths
- Use table-driven tests where appropriate

Run tests with coverage:
```bash
go test -cover ./...
```

### Commit Messages

Follow conventional commit format:
```
type(scope): description

[optional body]

[optional footer]
```

Types:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `test`: Adding or updating tests
- `refactor`: Code refactoring
- `ci`: CI/CD changes
- `chore`: Maintenance tasks

Examples:
```
feat(publisher): add circuit breaker support

Implement circuit breaker pattern to prevent cascading failures
when Pub/Sub is unavailable.

Closes #123
```

## Pull Request Process

1. Create a branch from `main`
2. Make your changes with appropriate tests
3. Ensure all tests pass: `go test ./...`
4. Ensure code passes linting: `golangci-lint run`
5. Update documentation if needed
6. Submit a pull request with a clear description

### PR Description Template

```markdown
## Summary
Brief description of changes

## Changes
- Change 1
- Change 2

## Testing
How the changes were tested

## Related Issues
Closes #XXX
```

## Testing Guidelines

### Unit Tests

Place tests in `*_test.go` files alongside the code:
```
pkg/webhook/handler.go
pkg/webhook/handler_test.go
```

### Integration Tests

For tests that require external services, use build tags:
```go
//go:build integration

package publisher
```

Run integration tests:
```bash
go test -tags=integration ./...
```

### Using the Pub/Sub Emulator

For local development and testing:
```bash
# Start emulator
docker run -d --name pubsub-emulator \
  -p 8085:8085 \
  gcr.io/google.com/cloudsdktool/google-cloud-cli:emulators \
  gcloud beta emulators pubsub start --host-port=0.0.0.0:8085

# Set environment variable
export PUBSUB_EMULATOR_HOST=localhost:8085
```

## Documentation

- Update README.md for user-facing changes
- Update docs/ for detailed documentation
- Add code comments for complex logic
- Keep documentation in sync with code

## Questions?

- Open an issue for bugs or feature requests
- Check existing issues before creating new ones
- Provide as much context as possible in issues

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
