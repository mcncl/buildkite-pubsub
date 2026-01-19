// Package config provides a standardized way to load, validate, and access application configuration.
// It supports loading configuration from environment variables, files (JSON/YAML), and explicit overrides.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mcncl/buildkite-pubsub/internal/errors"
	"gopkg.in/yaml.v3"
)

// Config holds all application configuration
type Config struct {
	GCP      GCPConfig      `json:"gcp" yaml:"gcp"`
	Webhook  WebhookConfig  `json:"webhook" yaml:"webhook"`
	Server   ServerConfig   `json:"server" yaml:"server"`
	Security SecurityConfig `json:"security" yaml:"security"`
}

// GCPConfig holds Google Cloud Platform related configuration
type GCPConfig struct {
	ProjectID              string  `json:"project_id" yaml:"project_id"`
	TopicID                string  `json:"topic_id" yaml:"topic_id"`
	CredentialsFile        string  `json:"credentials_file" yaml:"credentials_file"`
	EnableTracing          bool    `json:"enable_tracing" yaml:"enable_tracing"`
	OTLPEndpoint           string  `json:"otlp_endpoint" yaml:"otlp_endpoint"`
	TraceSamplingRatio     float64 `json:"trace_sampling_ratio" yaml:"trace_sampling_ratio"`
	PubSubBatchSize        int     `json:"pubsub_batch_size" yaml:"pubsub_batch_size"`
	PubSubRetryMaxAttempts int     `json:"pubsub_retry_max_attempts" yaml:"pubsub_retry_max_attempts"`
	// Dead Letter Queue configuration
	EnableDLQ  bool   `json:"enable_dlq" yaml:"enable_dlq"`
	DLQTopicID string `json:"dlq_topic_id" yaml:"dlq_topic_id"`
}

// WebhookConfig holds Buildkite webhook related configuration
type WebhookConfig struct {
	Token      string `json:"token" yaml:"token"`
	HMACSecret string `json:"hmac_secret" yaml:"hmac_secret"`
	Path       string `json:"path" yaml:"path"`
}

// ServerConfig holds HTTP server related configuration
type ServerConfig struct {
	Port           int           `json:"port" yaml:"port"`
	LogLevel       string        `json:"log_level" yaml:"log_level"`
	MaxRequestSize int           `json:"max_request_size" yaml:"max_request_size"`
	RequestTimeout time.Duration `json:"request_timeout" yaml:"request_timeout,omitempty"`
	ReadTimeout    time.Duration `json:"read_timeout" yaml:"read_timeout,omitempty"`
	WriteTimeout   time.Duration `json:"write_timeout" yaml:"write_timeout,omitempty"`
	IdleTimeout    time.Duration `json:"idle_timeout" yaml:"idle_timeout,omitempty"`
}

// SecurityConfig holds security related configuration
type SecurityConfig struct {
	RateLimit            int      `json:"rate_limit" yaml:"rate_limit"`
	IPRateLimit          int      `json:"ip_rate_limit" yaml:"ip_rate_limit"`
	AllowedOrigins       []string `json:"allowed_origins" yaml:"allowed_origins"`
	AllowedMethods       []string `json:"allowed_methods" yaml:"allowed_methods"`
	AllowedHeaders       []string `json:"allowed_headers" yaml:"allowed_headers"`
	EnableCSRFProtection bool     `json:"enable_csrf_protection" yaml:"enable_csrf_protection"`
	CSRFCookieName       string   `json:"csrf_cookie_name" yaml:"csrf_cookie_name"`
	CSRFHeaderName       string   `json:"csrf_header_name" yaml:"csrf_header_name"`
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		GCP: GCPConfig{
			CredentialsFile:        "credentials.json",
			EnableTracing:          true,
			OTLPEndpoint:           "localhost:4317",
			TraceSamplingRatio:     0.1,
			PubSubBatchSize:        100,
			PubSubRetryMaxAttempts: 5,
		},
		Webhook: WebhookConfig{
			Path: "/webhook",
		},
		Server: ServerConfig{
			Port:           8888,
			LogLevel:       "info",
			MaxRequestSize: 1 * 1024 * 1024, // 1 MB
			RequestTimeout: 30 * time.Second,
			ReadTimeout:    5 * time.Second,
			WriteTimeout:   10 * time.Second,
			IdleTimeout:    120 * time.Second,
		},
		Security: SecurityConfig{
			RateLimit:      60, // 60 requests per minute
			IPRateLimit:    30, // 30 requests per minute per IP
			AllowedOrigins: []string{"*"},
			AllowedMethods: []string{"POST", "OPTIONS"},
			AllowedHeaders: []string{
				"Accept",
				"Content-Type",
				"Content-Length",
				"Accept-Encoding",
				"Authorization",
				"X-CSRF-Token",
				"X-Buildkite-Token",
				"X-Request-ID",
			},
			EnableCSRFProtection: false,
			CSRFCookieName:       "csrf_token",
			CSRFHeaderName:       "X-CSRF-Token",
		},
	}
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	// Check required GCP fields
	if c.GCP.ProjectID == "" {
		return errors.NewValidationError("GCP.ProjectID cannot be empty")
	}
	if c.GCP.TopicID == "" {
		return errors.NewValidationError("GCP.TopicID cannot be empty")
	}
	// Validate DLQ configuration
	if c.GCP.EnableDLQ && c.GCP.DLQTopicID == "" {
		return errors.NewValidationError("GCP.DLQTopicID is required when DLQ is enabled")
	}

	// Check required Webhook fields - either Token or HMACSecret must be provided
	if c.Webhook.Token == "" && c.Webhook.HMACSecret == "" {
		return errors.NewValidationError("Webhook.Token or Webhook.HMACSecret must be provided")
	}

	// Check Server fields
	if c.Server.Port < 1024 || c.Server.Port > 65535 {
		return errors.NewValidationError("Server.Port must be between 1024 and 65535")
	}

	validLogLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
		"fatal": true,
		"trace": true,
	}
	if _, ok := validLogLevels[strings.ToLower(c.Server.LogLevel)]; !ok {
		return errors.NewValidationError("Server.LogLevel must be one of: debug, info, warn, error, fatal, trace")
	}

	// Check Security fields
	if c.Security.RateLimit < 0 {
		return errors.NewValidationError("Security.RateLimit cannot be negative")
	}
	if c.Security.IPRateLimit < 0 {
		return errors.NewValidationError("Security.IPRateLimit cannot be negative")
	}

	return nil
}

// LoadFromEnv loads configuration from environment variables
func LoadFromEnv() (*Config, error) {
	cfg := DefaultConfig()

	// Load GCP config
	if val := os.Getenv("PROJECT_ID"); val != "" {
		cfg.GCP.ProjectID = val
	}
	if val := os.Getenv("TOPIC_ID"); val != "" {
		cfg.GCP.TopicID = val
	}
	if val := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"); val != "" {
		cfg.GCP.CredentialsFile = val
	}
	if val := os.Getenv("ENABLE_TRACING"); val != "" {
		cfg.GCP.EnableTracing = strings.ToLower(val) == "true"
	}
	if val := os.Getenv("OTLP_ENDPOINT"); val != "" {
		cfg.GCP.OTLPEndpoint = val
	}
	if val := os.Getenv("TRACE_SAMPLING_RATIO"); val != "" {
		if ratio, err := strconv.ParseFloat(val, 64); err == nil && ratio >= 0 && ratio <= 1 {
			cfg.GCP.TraceSamplingRatio = ratio
		}
	}
	if val := os.Getenv("PUBSUB_BATCH_SIZE"); val != "" {
		if size, err := strconv.Atoi(val); err == nil && size > 0 {
			cfg.GCP.PubSubBatchSize = size
		}
	}
	if val := os.Getenv("PUBSUB_RETRY_MAX_ATTEMPTS"); val != "" {
		if attempts, err := strconv.Atoi(val); err == nil && attempts > 0 {
			cfg.GCP.PubSubRetryMaxAttempts = attempts
		}
	}
	// Dead Letter Queue configuration
	if val := os.Getenv("ENABLE_DLQ"); val != "" {
		cfg.GCP.EnableDLQ = strings.ToLower(val) == "true" || val == "1"
	}
	if val := os.Getenv("DLQ_TOPIC_ID"); val != "" {
		cfg.GCP.DLQTopicID = val
	}

	// Load Webhook config
	if val := os.Getenv("BUILDKITE_WEBHOOK_TOKEN"); val != "" {
		cfg.Webhook.Token = val
	}
	if val := os.Getenv("BUILDKITE_WEBHOOK_HMAC_SECRET"); val != "" {
		cfg.Webhook.HMACSecret = val
	}
	if val := os.Getenv("WEBHOOK_PATH"); val != "" {
		cfg.Webhook.Path = val
	}

	// Load Server config
	if val := os.Getenv("PORT"); val != "" {
		if port, err := strconv.Atoi(val); err == nil {
			cfg.Server.Port = port
		}
	}
	if val := os.Getenv("LOG_LEVEL"); val != "" {
		cfg.Server.LogLevel = val
	}
	if val := os.Getenv("MAX_REQUEST_SIZE"); val != "" {
		if size, err := strconv.Atoi(val); err == nil && size > 0 {
			cfg.Server.MaxRequestSize = size
		}
	}
	if val := os.Getenv("REQUEST_TIMEOUT"); val != "" {
		if timeout, err := strconv.Atoi(val); err == nil && timeout > 0 {
			cfg.Server.RequestTimeout = time.Duration(timeout) * time.Second
		}
	}
	if val := os.Getenv("READ_TIMEOUT"); val != "" {
		if timeout, err := strconv.Atoi(val); err == nil && timeout > 0 {
			cfg.Server.ReadTimeout = time.Duration(timeout) * time.Second
		}
	}
	if val := os.Getenv("WRITE_TIMEOUT"); val != "" {
		if timeout, err := strconv.Atoi(val); err == nil && timeout > 0 {
			cfg.Server.WriteTimeout = time.Duration(timeout) * time.Second
		}
	}
	if val := os.Getenv("IDLE_TIMEOUT"); val != "" {
		if timeout, err := strconv.Atoi(val); err == nil && timeout > 0 {
			cfg.Server.IdleTimeout = time.Duration(timeout) * time.Second
		}
	}

	// Load Security config
	if val := os.Getenv("RATE_LIMIT"); val != "" {
		if limit, err := strconv.Atoi(val); err == nil && limit >= 0 {
			cfg.Security.RateLimit = limit
		}
	}
	if val := os.Getenv("IP_RATE_LIMIT"); val != "" {
		if limit, err := strconv.Atoi(val); err == nil && limit >= 0 {
			cfg.Security.IPRateLimit = limit
		}
	}
	if val := os.Getenv("ALLOWED_ORIGINS"); val != "" {
		cfg.Security.AllowedOrigins = strings.Split(val, ",")
	}
	if val := os.Getenv("ALLOWED_METHODS"); val != "" {
		cfg.Security.AllowedMethods = strings.Split(val, ",")
	}
	if val := os.Getenv("ALLOWED_HEADERS"); val != "" {
		cfg.Security.AllowedHeaders = strings.Split(val, ",")
	}
	if val := os.Getenv("ENABLE_CSRF_PROTECTION"); val != "" {
		cfg.Security.EnableCSRFProtection = strings.ToLower(val) == "true"
	}
	if val := os.Getenv("CSRF_COOKIE_NAME"); val != "" {
		cfg.Security.CSRFCookieName = val
	}
	if val := os.Getenv("CSRF_HEADER_NAME"); val != "" {
		cfg.Security.CSRFHeaderName = val
	}

	return cfg, nil
}

// LoadFromFile loads configuration from a JSON or YAML file
func LoadFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read config file")
	}

	cfg := DefaultConfig()

	// Create a temporary struct for parsing that uses string types for durations
	type tempConfig struct {
		GCP struct {
			ProjectID              string  `json:"project_id" yaml:"project_id"`
			TopicID                string  `json:"topic_id" yaml:"topic_id"`
			CredentialsFile        string  `json:"credentials_file" yaml:"credentials_file"`
			EnableTracing          bool    `json:"enable_tracing" yaml:"enable_tracing"`
			OTLPEndpoint           string  `json:"otlp_endpoint" yaml:"otlp_endpoint"`
			TraceSamplingRatio     float64 `json:"trace_sampling_ratio" yaml:"trace_sampling_ratio"`
			PubSubBatchSize        int     `json:"pubsub_batch_size" yaml:"pubsub_batch_size"`
			PubSubRetryMaxAttempts int     `json:"pubsub_retry_max_attempts" yaml:"pubsub_retry_max_attempts"`
		} `json:"gcp" yaml:"gcp"`
		Webhook struct {
			Token      string `json:"token" yaml:"token"`
			HMACSecret string `json:"hmac_secret" yaml:"hmac_secret"`
			Path       string `json:"path" yaml:"path"`
		} `json:"webhook" yaml:"webhook"`
		Server struct {
			Port           int    `json:"port" yaml:"port"`
			LogLevel       string `json:"log_level" yaml:"log_level"`
			MaxRequestSize int    `json:"max_request_size" yaml:"max_request_size"`
			RequestTimeout string `json:"request_timeout" yaml:"request_timeout"`
			ReadTimeout    string `json:"read_timeout" yaml:"read_timeout"`
			WriteTimeout   string `json:"write_timeout" yaml:"write_timeout"`
			IdleTimeout    string `json:"idle_timeout" yaml:"idle_timeout"`
		} `json:"server" yaml:"server"`
		Security struct {
			RateLimit            int      `json:"rate_limit" yaml:"rate_limit"`
			IPRateLimit          int      `json:"ip_rate_limit" yaml:"ip_rate_limit"`
			AllowedOrigins       []string `json:"allowed_origins" yaml:"allowed_origins"`
			AllowedMethods       []string `json:"allowed_methods" yaml:"allowed_methods"`
			AllowedHeaders       []string `json:"allowed_headers" yaml:"allowed_headers"`
			EnableCSRFProtection bool     `json:"enable_csrf_protection" yaml:"enable_csrf_protection"`
			CSRFCookieName       string   `json:"csrf_cookie_name" yaml:"csrf_cookie_name"`
			CSRFHeaderName       string   `json:"csrf_header_name" yaml:"csrf_header_name"`
		} `json:"security" yaml:"security"`
	}

	var tempCfg tempConfig

	// Determine file type from extension
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		// For JSON, we'll try first with the original struct
		// and if that fails, then with the temporary struct
		err := json.Unmarshal(data, cfg)
		if err != nil {
			// Try with tempCfg
			if err := json.Unmarshal(data, &tempCfg); err != nil {
				return nil, errors.Wrap(err, "failed to parse JSON config file")
			}
		} else {
			// Original unmarshal worked, just return the config
			return cfg, nil
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &tempCfg); err != nil {
			return nil, errors.Wrap(err, "failed to parse YAML config file")
		}
	default:
		return nil, errors.NewValidationError("unsupported config file format: " + ext)
	}

	// Copy over the values to our actual config struct
	cfg.GCP.ProjectID = tempCfg.GCP.ProjectID
	cfg.GCP.TopicID = tempCfg.GCP.TopicID
	cfg.GCP.CredentialsFile = tempCfg.GCP.CredentialsFile
	cfg.GCP.EnableTracing = tempCfg.GCP.EnableTracing
	cfg.GCP.OTLPEndpoint = tempCfg.GCP.OTLPEndpoint
	cfg.GCP.TraceSamplingRatio = tempCfg.GCP.TraceSamplingRatio
	cfg.GCP.PubSubBatchSize = tempCfg.GCP.PubSubBatchSize
	cfg.GCP.PubSubRetryMaxAttempts = tempCfg.GCP.PubSubRetryMaxAttempts

	cfg.Webhook.Token = tempCfg.Webhook.Token
	cfg.Webhook.HMACSecret = tempCfg.Webhook.HMACSecret
	cfg.Webhook.Path = tempCfg.Webhook.Path

	cfg.Server.Port = tempCfg.Server.Port
	cfg.Server.LogLevel = tempCfg.Server.LogLevel
	cfg.Server.MaxRequestSize = tempCfg.Server.MaxRequestSize

	// Parse duration values
	if tempCfg.Server.RequestTimeout != "" {
		if secs, err := strconv.Atoi(tempCfg.Server.RequestTimeout); err == nil {
			cfg.Server.RequestTimeout = time.Duration(secs) * time.Second
		} else if d, err := time.ParseDuration(tempCfg.Server.RequestTimeout); err == nil {
			cfg.Server.RequestTimeout = d
		}
	}

	if tempCfg.Server.ReadTimeout != "" {
		if secs, err := strconv.Atoi(tempCfg.Server.ReadTimeout); err == nil {
			cfg.Server.ReadTimeout = time.Duration(secs) * time.Second
		} else if d, err := time.ParseDuration(tempCfg.Server.ReadTimeout); err == nil {
			cfg.Server.ReadTimeout = d
		}
	}

	if tempCfg.Server.WriteTimeout != "" {
		if secs, err := strconv.Atoi(tempCfg.Server.WriteTimeout); err == nil {
			cfg.Server.WriteTimeout = time.Duration(secs) * time.Second
		} else if d, err := time.ParseDuration(tempCfg.Server.WriteTimeout); err == nil {
			cfg.Server.WriteTimeout = d
		}
	}

	if tempCfg.Server.IdleTimeout != "" {
		if secs, err := strconv.Atoi(tempCfg.Server.IdleTimeout); err == nil {
			cfg.Server.IdleTimeout = time.Duration(secs) * time.Second
		} else if d, err := time.ParseDuration(tempCfg.Server.IdleTimeout); err == nil {
			cfg.Server.IdleTimeout = d
		}
	}

	cfg.Security.RateLimit = tempCfg.Security.RateLimit
	cfg.Security.IPRateLimit = tempCfg.Security.IPRateLimit
	cfg.Security.AllowedOrigins = tempCfg.Security.AllowedOrigins
	cfg.Security.AllowedMethods = tempCfg.Security.AllowedMethods
	cfg.Security.AllowedHeaders = tempCfg.Security.AllowedHeaders
	cfg.Security.EnableCSRFProtection = tempCfg.Security.EnableCSRFProtection
	cfg.Security.CSRFCookieName = tempCfg.Security.CSRFCookieName
	cfg.Security.CSRFHeaderName = tempCfg.Security.CSRFHeaderName

	return cfg, nil
}

// MergeConfigs merges two configurations, with the second taking precedence
func MergeConfigs(base, override *Config) *Config {
	result := *base

	// Only override non-zero values
	if override == nil {
		return &result
	}

	// GCP config
	if override.GCP.ProjectID != "" {
		result.GCP.ProjectID = override.GCP.ProjectID
	}
	if override.GCP.TopicID != "" {
		result.GCP.TopicID = override.GCP.TopicID
	}
	if override.GCP.CredentialsFile != "" {
		result.GCP.CredentialsFile = override.GCP.CredentialsFile
	}
	// We need to explicitly check booleans
	if override.GCP.EnableTracing {
		result.GCP.EnableTracing = true
	}
	if override.GCP.TraceSamplingRatio != 0 {
		result.GCP.TraceSamplingRatio = override.GCP.TraceSamplingRatio
	}
	if override.GCP.PubSubBatchSize != 0 {
		result.GCP.PubSubBatchSize = override.GCP.PubSubBatchSize
	}
	if override.GCP.PubSubRetryMaxAttempts != 0 {
		result.GCP.PubSubRetryMaxAttempts = override.GCP.PubSubRetryMaxAttempts
	}

	// Webhook config
	if override.Webhook.Token != "" {
		result.Webhook.Token = override.Webhook.Token
	}
	if override.Webhook.HMACSecret != "" {
		result.Webhook.HMACSecret = override.Webhook.HMACSecret
	}
	if override.Webhook.Path != "" {
		result.Webhook.Path = override.Webhook.Path
	}

	// Server config
	if override.Server.Port != 0 {
		result.Server.Port = override.Server.Port
	}
	if override.Server.LogLevel != "" {
		result.Server.LogLevel = override.Server.LogLevel
	}
	if override.Server.MaxRequestSize != 0 {
		result.Server.MaxRequestSize = override.Server.MaxRequestSize
	}
	if override.Server.RequestTimeout != 0 {
		result.Server.RequestTimeout = override.Server.RequestTimeout
	}
	if override.Server.ReadTimeout != 0 {
		result.Server.ReadTimeout = override.Server.ReadTimeout
	}
	if override.Server.WriteTimeout != 0 {
		result.Server.WriteTimeout = override.Server.WriteTimeout
	}
	if override.Server.IdleTimeout != 0 {
		result.Server.IdleTimeout = override.Server.IdleTimeout
	}

	// Security config
	if override.Security.RateLimit != 0 {
		result.Security.RateLimit = override.Security.RateLimit
	}
	if override.Security.IPRateLimit != 0 {
		result.Security.IPRateLimit = override.Security.IPRateLimit
	}
	if len(override.Security.AllowedOrigins) > 0 {
		result.Security.AllowedOrigins = override.Security.AllowedOrigins
	}
	if len(override.Security.AllowedMethods) > 0 {
		result.Security.AllowedMethods = override.Security.AllowedMethods
	}
	if len(override.Security.AllowedHeaders) > 0 {
		result.Security.AllowedHeaders = override.Security.AllowedHeaders
	}
	if override.Security.EnableCSRFProtection {
		result.Security.EnableCSRFProtection = true
	}
	if override.Security.CSRFCookieName != "" {
		result.Security.CSRFCookieName = override.Security.CSRFCookieName
	}
	if override.Security.CSRFHeaderName != "" {
		result.Security.CSRFHeaderName = override.Security.CSRFHeaderName
	}

	return &result
}

// Load loads the configuration from multiple sources with the following precedence:
// 1. Override (highest precedence)
// 2. Environment variables
// 3. Config file
// 4. Default values (lowest precedence)
func Load(configFile string, override *Config) (*Config, error) {
	// Start with default configuration
	cfg := DefaultConfig()

	// Load from file if provided
	if configFile != "" {
		fileCfg, err := LoadFromFile(configFile)
		if err != nil {
			return nil, err
		}
		cfg = MergeConfigs(cfg, fileCfg)
	}

	// Load from environment variables
	envCfg, err := LoadFromEnv()
	if err != nil {
		return nil, err
	}
	cfg = MergeConfigs(cfg, envCfg)

	// Apply explicit overrides
	if override != nil {
		cfg = MergeConfigs(cfg, override)
	}

	// Validate the final configuration
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// String returns a string representation of the configuration
// with sensitive fields masked
func (c *Config) String() string {
	// Create a copy to avoid modifying the original
	copy := *c

	// Mask sensitive fields
	if copy.Webhook.Token != "" {
		copy.Webhook.Token = "********"
	}
	if copy.Webhook.HMACSecret != "" {
		copy.Webhook.HMACSecret = "********"
	}

	// Convert to JSON
	bytes, err := json.MarshalIndent(copy, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error marshaling config: %v", err)
	}

	return string(bytes)
}
