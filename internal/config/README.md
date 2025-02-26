# Configuration Package

This package provides a standardized way to manage the application's configuration. It supports loading configuration from multiple sources with clear precedence, validation, and sensible defaults.

## Features

- Configuration from multiple sources (environment variables, files, and explicit overrides)
- Clear precedence rules (overrides > environment variables > config file > defaults)
- Validation of configuration values
- Sensible defaults for optional settings
- Support for both JSON and YAML configuration files
- Masking of sensitive information in logs

## Usage

### Basic Usage

The simplest way to load configuration is:

```go
import "github.com/mcncl/buildkite-pubsub/internal/config"

func main() {
    // Load configuration with defaults and environment variables
    cfg, err := config.Load("", nil)
    if err != nil {
        log.Fatalf("Failed to load configuration: %v", err)
    }

    // Use the configuration
    log.Printf("Starting server on port %d", cfg.Server.Port)
    // ...
}
```

### Loading from a Configuration File

You can load configuration from a JSON or YAML file:

```go
cfg, err := config.Load("config.json", nil)
if err != nil {
    log.Fatalf("Failed to load configuration: %v", err)
}
```

Configuration files can be in JSON format:

```json
{
  "gcp": {
    "project_id": "my-project",
    "topic_id": "buildkite-events"
  },
  "webhook": {
    "token": "my-webhook-token",
    "path": "/webhook"
  },
  "server": {
    "port": 8080,
    "log_level": "info"
  }
}
```

Or in YAML format:

```yaml
gcp:
  project_id: my-project
  topic_id: buildkite-events
webhook:
  token: my-webhook-token
  path: /webhook
server:
  port: 8080
  log_level: info
```

### Using Environment Variables

Environment variables take precedence over configuration files and default values. The package maps environment variables to configuration fields as follows:

| Environment Variable | Configuration Field |
|----------------------|---------------------|
| `PROJECT_ID` | `GCP.ProjectID` |
| `TOPIC_ID` | `GCP.TopicID` |
| `BUILDKITE_WEBHOOK_TOKEN` | `Webhook.Token` |
| `PORT` | `Server.Port` |
| `LOG_LEVEL` | `Server.LogLevel` |
| ... | ... |

See `LoadFromEnv()` in the code for the complete mapping.

### Using Explicit Overrides

You can provide explicit overrides that take highest precedence:

```go
override := &config.Config{
    Server: config.ServerConfig{
        Port: 9090,
    },
}

cfg, err := config.Load("config.json", override)
if err != nil {
    log.Fatalf("Failed to load configuration: %v", err)
}

// Port will be 9090 regardless of what's in the config file or environment
```

### Command-Line Flags

A common pattern is to use command-line flags for the configuration file path:

```go
func main() {
    configFile := flag.String("config", "", "Path to configuration file")
    flag.Parse()

    cfg, err := config.Load(*configFile, nil)
    if err != nil {
        log.Fatalf("Failed to load configuration: %v", err)
    }

    // Use configuration...
}
```

## Configuration Structure

The configuration is divided into logical sections:

- `GCP`: Google Cloud Platform settings
- `Webhook`: Buildkite webhook settings
- `Server`: HTTP server settings
- `Security`: Security-related settings

See the `Config` struct in the code for the complete structure.

## Default Values

The package provides sensible defaults for many values. See `DefaultConfig()` in the code for the complete list of defaults.

## Validation

The package validates the configuration before returning it. Validation includes:

- Required fields (e.g., `GCP.ProjectID`, `GCP.TopicID`, `Webhook.Token`)
- Value ranges (e.g., port numbers, rate limits)
- Enumerated values (e.g., log levels)

## Development and Testing

When developing or testing with this package, you can create a test configuration:

```go
// For tests
func createTestConfig() *config.Config {
    return &config.Config{
        GCP: config.GCPConfig{
            ProjectID: "test-project",
            TopicID: "test-topic",
        },
        Webhook: config.WebhookConfig{
            Token: "test-token",
        },
        // Set other required fields...
    }
}
```

You can also use environment variables in your tests:

```go
func TestWithEnvironment(t *testing.T) {
    // Save original environment
    origEnv := os.Getenv("PROJECT_ID")
    defer os.Setenv("PROJECT_ID", origEnv)

    // Set test environment
    os.Setenv("PROJECT_ID", "test-project")

    // Run test...
}
```
