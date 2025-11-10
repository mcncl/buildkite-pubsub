package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadFromEnv(t *testing.T) {
	// Save original environment and restore after test
	envBackup := make(map[string]string)
	for _, key := range []string{
		"PROJECT_ID",
		"TOPIC_ID",
		"BUILDKITE_WEBHOOK_TOKEN",
		"PORT",
		"LOG_LEVEL",
		"MAX_REQUEST_SIZE",
		"REQUEST_TIMEOUT",
		"RATE_LIMIT",
		"IP_RATE_LIMIT",
	} {
		if val, exists := os.LookupEnv(key); exists {
			envBackup[key] = val
		}
	}
	defer func() {
		// Restore environment
		for key := range envBackup {
			_ = os.Unsetenv(key)
		}
		for key, val := range envBackup {
			_ = os.Setenv(key, val)
		}
	}()

	// Clear and set test environment variables
	for key := range envBackup {
		_ = os.Unsetenv(key)
	}

	// Set test environment variables
	_ = os.Setenv("PROJECT_ID", "test-project")
	_ = os.Setenv("TOPIC_ID", "test-topic")
	_ = os.Setenv("BUILDKITE_WEBHOOK_TOKEN", "test-token")
	_ = os.Setenv("PORT", "9090")
	_ = os.Setenv("LOG_LEVEL", "debug")
	_ = os.Setenv("MAX_REQUEST_SIZE", "5242880") // 5 MB
	_ = os.Setenv("REQUEST_TIMEOUT", "45")       // 45 seconds
	_ = os.Setenv("RATE_LIMIT", "120")           // 120 requests per minute
	_ = os.Setenv("IP_RATE_LIMIT", "60")         // 60 requests per minute per IP

	// Load configuration from environment
	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("Failed to load config from environment: %v", err)
	}

	// Verify values
	if cfg.GCP.ProjectID != "test-project" {
		t.Errorf("ProjectID = %q, want %q", cfg.GCP.ProjectID, "test-project")
	}
	if cfg.GCP.TopicID != "test-topic" {
		t.Errorf("TopicID = %q, want %q", cfg.GCP.TopicID, "test-topic")
	}
	if cfg.Webhook.Token != "test-token" {
		t.Errorf("Token = %q, want %q", cfg.Webhook.Token, "test-token")
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("Port = %d, want %d", cfg.Server.Port, 9090)
	}
	if cfg.Server.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want %q", cfg.Server.LogLevel, "debug")
	}
	if cfg.Server.MaxRequestSize != 5*1024*1024 {
		t.Errorf("MaxRequestSize = %d, want %d", cfg.Server.MaxRequestSize, 5*1024*1024)
	}
	if cfg.Server.RequestTimeout != 45*time.Second {
		t.Errorf("RequestTimeout = %v, want %v", cfg.Server.RequestTimeout, 45*time.Second)
	}
	if cfg.Security.RateLimit != 120 {
		t.Errorf("RateLimit = %d, want %d", cfg.Security.RateLimit, 120)
	}
	if cfg.Security.IPRateLimit != 60 {
		t.Errorf("IPRateLimit = %d, want %d", cfg.Security.IPRateLimit, 60)
	}
}

func TestLoadFromFile(t *testing.T) {
	// Create temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "config-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create JSON configuration file
	jsonConfig := `{
		"gcp": {
			"project_id": "file-project",
			"topic_id": "file-topic"
		},
		"webhook": {
			"token": "file-token"
		},
		"server": {
			"port": 8888,
			"log_level": "info",
			"max_request_size": 10485760,
			"request_timeout": "60s"
		},
		"security": {
			"rate_limit": 100,
			"ip_rate_limit": 50
		}
	}`
	jsonPath := filepath.Join(tmpDir, "config.json")
	if err := os.WriteFile(jsonPath, []byte(jsonConfig), 0o644); err != nil {
		t.Fatalf("Failed to write test JSON file: %v", err)
	}

	// Create YAML configuration file
	yamlConfig := `gcp:
  project_id: yaml-project
  topic_id: yaml-topic
webhook:
  token: yaml-token
server:
  port: 7777
  log_level: warn
  max_request_size: 2097152
  request_timeout: "20s"
security:
  rate_limit: 80
  ip_rate_limit: 40
`
	yamlPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlConfig), 0o644); err != nil {
		t.Fatalf("Failed to write test YAML file: %v", err)
	}

	// Test loading from JSON
	jsonCfg, err := LoadFromFile(jsonPath)
	if err != nil {
		t.Fatalf("Failed to load config from JSON: %v", err)
	}

	// Verify JSON values
	if jsonCfg.GCP.ProjectID != "file-project" {
		t.Errorf("JSON ProjectID = %q, want %q", jsonCfg.GCP.ProjectID, "file-project")
	}
	if jsonCfg.Server.Port != 8888 {
		t.Errorf("JSON Port = %d, want %d", jsonCfg.Server.Port, 8888)
	}

	// Test loading from YAML
	yamlCfg, err := LoadFromFile(yamlPath)
	if err != nil {
		t.Fatalf("Failed to load config from YAML: %v", err)
	}

	// Verify YAML values
	if yamlCfg.GCP.ProjectID != "yaml-project" {
		t.Errorf("YAML ProjectID = %q, want %q", yamlCfg.GCP.ProjectID, "yaml-project")
	}
	if yamlCfg.Server.Port != 7777 {
		t.Errorf("YAML Port = %d, want %d", yamlCfg.Server.Port, 7777)
	}

	// Test loading from nonexistent file
	_, err = LoadFromFile("nonexistent.json")
	if err == nil {
		t.Error("Expected error when loading from nonexistent file, got nil")
	}

	// Test loading from invalid file
	invalidPath := filepath.Join(tmpDir, "invalid.json")
	if err := os.WriteFile(invalidPath, []byte("not valid json"), 0o644); err != nil {
		t.Fatalf("Failed to write invalid file: %v", err)
	}
	_, err = LoadFromFile(invalidPath)
	if err == nil {
		t.Error("Expected error when loading from invalid file, got nil")
	}
}

func TestMergeConfigs(t *testing.T) {
	// Create base config with defaults
	base := DefaultConfig()

	// Create override config
	override := &Config{
		GCP: GCPConfig{
			ProjectID: "override-project",
		},
		Server: ServerConfig{
			Port:     9999,
			LogLevel: "trace",
		},
	}

	// Merge configs
	merged := MergeConfigs(base, override)

	// Check that override values are used
	if merged.GCP.ProjectID != "override-project" {
		t.Errorf("Merged ProjectID = %q, want %q", merged.GCP.ProjectID, "override-project")
	}
	if merged.Server.Port != 9999 {
		t.Errorf("Merged Port = %d, want %d", merged.Server.Port, 9999)
	}
	if merged.Server.LogLevel != "trace" {
		t.Errorf("Merged LogLevel = %q, want %q", merged.Server.LogLevel, "trace")
	}

	// Check that non-overridden values remain from base
	if merged.Security.RateLimit != base.Security.RateLimit {
		t.Errorf("Merged RateLimit = %d, want %d", merged.Security.RateLimit, base.Security.RateLimit)
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name      string
		config    Config
		wantError bool
	}{
		{
			name: "valid config",
			config: Config{
				GCP: GCPConfig{
					ProjectID: "valid-project",
					TopicID:   "valid-topic",
				},
				Webhook: WebhookConfig{
					Token: "valid-token",
				},
				Server: ServerConfig{
					Port:           8080,
					LogLevel:       "info",
					MaxRequestSize: 1024 * 1024,
					RequestTimeout: 30 * time.Second,
				},
				Security: SecurityConfig{
					RateLimit:   60,
					IPRateLimit: 30,
				},
			},
			wantError: false,
		},
		{
			name: "missing project ID",
			config: Config{
				GCP: GCPConfig{
					TopicID: "valid-topic",
				},
				Webhook: WebhookConfig{
					Token: "valid-token",
				},
			},
			wantError: true,
		},
		{
			name: "missing topic ID",
			config: Config{
				GCP: GCPConfig{
					ProjectID: "valid-project",
				},
				Webhook: WebhookConfig{
					Token: "valid-token",
				},
			},
			wantError: true,
		},
		{
			name: "missing webhook token and HMAC secret",
			config: Config{
				GCP: GCPConfig{
					ProjectID: "valid-project",
					TopicID:   "valid-topic",
				},
				Webhook: WebhookConfig{},
			},
			wantError: true,
		},
		{
			name: "valid config with HMAC secret only",
			config: Config{
				GCP: GCPConfig{
					ProjectID: "valid-project",
					TopicID:   "valid-topic",
				},
				Webhook: WebhookConfig{
					HMACSecret: "valid-hmac-secret",
				},
				Server: ServerConfig{
					Port:           8080,
					LogLevel:       "info",
					MaxRequestSize: 1024 * 1024,
					RequestTimeout: 30 * time.Second,
				},
				Security: SecurityConfig{
					RateLimit:   60,
					IPRateLimit: 30,
				},
			},
			wantError: false,
		},
		{
			name: "valid config with both token and HMAC secret",
			config: Config{
				GCP: GCPConfig{
					ProjectID: "valid-project",
					TopicID:   "valid-topic",
				},
				Webhook: WebhookConfig{
					Token:      "valid-token",
					HMACSecret: "valid-hmac-secret",
				},
				Server: ServerConfig{
					Port:           8080,
					LogLevel:       "info",
					MaxRequestSize: 1024 * 1024,
					RequestTimeout: 30 * time.Second,
				},
				Security: SecurityConfig{
					RateLimit:   60,
					IPRateLimit: 30,
				},
			},
			wantError: false,
		},
		{
			name: "invalid port (too low)",
			config: Config{
				GCP: GCPConfig{
					ProjectID: "valid-project",
					TopicID:   "valid-topic",
				},
				Webhook: WebhookConfig{
					Token: "valid-token",
				},
				Server: ServerConfig{
					Port: 80, // Ports below 1024 require root privileges
				},
			},
			wantError: true,
		},
		{
			name: "invalid port (too high)",
			config: Config{
				GCP: GCPConfig{
					ProjectID: "valid-project",
					TopicID:   "valid-topic",
				},
				Webhook: WebhookConfig{
					Token: "valid-token",
				},
				Server: ServerConfig{
					Port: 70000, // Port too high
				},
			},
			wantError: true,
		},
		{
			name: "invalid log level",
			config: Config{
				GCP: GCPConfig{
					ProjectID: "valid-project",
					TopicID:   "valid-topic",
				},
				Webhook: WebhookConfig{
					Token: "valid-token",
				},
				Server: ServerConfig{
					LogLevel: "invalid",
				},
			},
			wantError: true,
		},
		{
			name: "negative rate limit",
			config: Config{
				GCP: GCPConfig{
					ProjectID: "valid-project",
					TopicID:   "valid-topic",
				},
				Webhook: WebhookConfig{
					Token: "valid-token",
				},
				Security: SecurityConfig{
					RateLimit: -10,
				},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestLoadWithPrecedence(t *testing.T) {
	// Save original environment and restore after test
	envBackup := make(map[string]string)
	for _, key := range []string{
		"PROJECT_ID",
		"TOPIC_ID",
		"BUILDKITE_WEBHOOK_TOKEN",
		"PORT",
		"LOG_LEVEL",
	} {
		if val, exists := os.LookupEnv(key); exists {
			envBackup[key] = val
		}
	}
	defer func() {
		// Restore environment
		for key := range envBackup {
			_ = os.Unsetenv(key)
		}
		for key, val := range envBackup {
			_ = os.Setenv(key, val)
		}
	}()

	// Clear environment variables
	for key := range envBackup {
		_ = os.Unsetenv(key)
	}

	// Create temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "config-precedence-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create config file
	configContent := `{
		"gcp": {
			"project_id": "file-project",
			"topic_id": "file-topic"
		},
		"webhook": {
			"token": "file-token"
		},
		"server": {
			"port": 8888,
			"log_level": "info",
			"request_timeout": "30s"
		}
	}`
	configPath := filepath.Join(tmpDir, "config.json")
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	// Test 1: Load with defaults only
	defaults := DefaultConfig()
	_, err = Load("", defaults)
	if err == nil {
		t.Error("Expected error when loading with missing required fields")
	}

	// Test 2: Load with file only
	cfg2, err := Load(configPath, nil)
	if err != nil {
		t.Fatalf("Failed to load config from file: %v", err)
	}
	if cfg2.GCP.ProjectID != "file-project" {
		t.Errorf("ProjectID = %q, want %q", cfg2.GCP.ProjectID, "file-project")
	}
	if cfg2.Server.Port != 8888 {
		t.Errorf("Port = %d, want %d", cfg2.Server.Port, 8888)
	}
	if cfg2.Server.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want %q", cfg2.Server.LogLevel, "info")
	}

	// Test 3: Set environment variables to override file values
	_ = os.Setenv("PROJECT_ID", "env-project")
	_ = os.Setenv("PORT", "9999")
	_ = os.Setenv("LOG_LEVEL", "debug")

	cfg3, err := Load(configPath, nil)
	if err != nil {
		t.Fatalf("Failed to load config with file and env: %v", err)
	}
	if cfg3.GCP.ProjectID != "env-project" {
		t.Errorf("ProjectID = %q, want %q", cfg3.GCP.ProjectID, "env-project")
	}
	if cfg3.GCP.TopicID != "file-topic" {
		t.Errorf("TopicID = %q, want %q", cfg3.GCP.TopicID, "file-topic")
	}
	if cfg3.Server.Port != 9999 {
		t.Errorf("Port = %d, want %d", cfg3.Server.Port, 9999)
	}
	if cfg3.Server.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want %q", cfg3.Server.LogLevel, "debug")
	}

	// Test 4: Override with explicit values (highest precedence)
	override := &Config{
		Server: ServerConfig{
			Port: 7777,
		},
	}

	cfg4, err := Load(configPath, override)
	if err != nil {
		t.Fatalf("Failed to load config with overrides: %v", err)
	}
	if cfg4.GCP.ProjectID != "env-project" {
		t.Errorf("ProjectID = %q, want %q", cfg4.GCP.ProjectID, "env-project")
	}
	if cfg4.Server.Port != 7777 {
		t.Errorf("Port = %d, want %d", cfg4.Server.Port, 7777)
	}

	// Test 5: Ensure default values are used for unspecified fields
	if cfg4.Server.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want %q", cfg4.Server.LogLevel, "debug")
	}
}
